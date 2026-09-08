//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// FirstAgentTextMessageInput 定义 AI 草稿及首条消息。
type FirstAgentTextMessageInput struct {
	ConversationID  string
	AgentIdentityID string
	ClientMessageID string
	Body            string
}

// FirstAgentTextMessageResult 定义首次发送确认的会话和消息。
type FirstAgentTextMessageResult struct {
	Conversation inboxaction.ConversationSummary
	Message      ConversationMessage
}

// SendFirstAgentTextMessageAction 在首次发送时原子创建 AI 聊天。
type SendFirstAgentTextMessageAction struct {
	db        *bun.DB
	scheduler AgentChatMessageScheduler
}

// NewSendFirstAgentTextMessageAction 创建 AI 聊天首次发送操作。
func NewSendFirstAgentTextMessageAction(db *bun.DB, scheduler AgentChatMessageScheduler) *SendFirstAgentTextMessageAction {
	return &SendFirstAgentTextMessageAction{db: db, scheduler: scheduler}
}

// Execute 按草稿稳定编号创建会话并幂等保存消息。
func (a *SendFirstAgentTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input FirstAgentTextMessageInput) (FirstAgentTextMessageResult, error) {
	messageInput, fields := normalizeInternalMessageInput(InternalTextMessageInput{ConversationID: input.ConversationID, ClientMessageID: input.ClientMessageID, Body: input.Body})
	agentID, valid := common.NormalizeUUID(input.AgentIdentityID)
	if !valid {
		fields["agentIdentityId"] = ValidationTargetIdentityIDInvalid
	}
	if len(fields) > 0 {
		return FirstAgentTextMessageResult{}, &ValidationError{Fields: fields}
	}
	var result FirstAgentTextMessageResult
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := ensureAgentConversation(ctx, tx, identity, messageInput.ConversationID, agentID, messageInput.Body); err != nil {
			return err
		}
		sendContext, err := lockAgentSendContext(ctx, tx, identity, messageInput.ConversationID)
		if err != nil {
			return err
		}
		result.Message, err = saveInternalTextMessage(ctx, tx, identity, messageInput, sendContext, a.scheduler)
		if err != nil {
			return err
		}
		result.Conversation, err = inboxaction.NewLoadInboxQuery(tx).LoadAgentConversation(ctx, identity, messageInput.ConversationID)
		return err
	})
	if err != nil {
		return FirstAgentTextMessageResult{}, err
	}
	return result, nil
}

// ensureAgentConversation 创建 AI 聊天，重试时核对固定的业务归属。
func ensureAgentConversation(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID, agentID, body string) error {
	// 已有草稿先锁定并核对归属，不重新创建共享主体。
	if found, err := lockAgentConversationDraft(ctx, tx, identity, conversationID, agentID); err != nil || found {
		return err
	}
	var target servermodels.OrganizationIdentity
	err := tx.NewSelect().Model(&target).
		Join("JOIN agents AS agent ON agent.organization_id = oi.organization_id AND agent.identity_id = oi.id").
		Where("oi.organization_id = ? AND oi.id = ? AND oi.type = ?", identity.Organization.ID, agentID, domain.OrganizationIdentityTypeAgent).
		Where("agent.status = ?", domain.UserStatusActive).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAgentTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("load AI chat target: %w", err)
	}
	subjects, err := ensureOrganizationIdentityChatSubjects(ctx, tx, identity.Organization.ID, []string{identity.OrganizationIdentity.ID, agentID})
	if err != nil {
		return err
	}
	userSubject, agentSubject := subjects[identity.OrganizationIdentity.ID], subjects[agentID]
	title := []rune(strings.Join(strings.Fields(body), " "))
	if len(title) > 40 {
		title = title[:40]
	}
	titleText := string(title)
	cv := &servermodels.Conversation{ID: conversationID, OrganizationID: identity.Organization.ID, Type: string(domain.ConversationTypeAgent), Status: string(domain.ConversationStatusActive), Title: &titleText, CreatedBySubjectID: &userSubject.ID}
	inserted, err := tx.NewInsert().Model(cv).Column("id", "organization_id", "type", "status", "title", "created_by_subject_id").On("CONFLICT (id) DO NOTHING").Exec(ctx)
	if err != nil {
		return fmt.Errorf("create AI conversation: %w", err)
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		found, err := lockAgentConversationDraft(ctx, tx, identity, conversationID, agentID)
		if err != nil {
			return err
		}
		if !found {
			return &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		return nil
	}
	relation := &servermodels.AgentConversation{ConversationID: conversationID, OrganizationID: identity.Organization.ID, UserIdentityID: identity.OrganizationIdentity.ID, AgentIdentityID: agentID}
	if _, err := tx.NewInsert().Model(relation).Column("conversation_id", "organization_id", "user_identity_id", "agent_identity_id").Exec(ctx); err != nil {
		return fmt.Errorf("create AI conversation relation: %w", err)
	}
	participants := []*servermodels.ConversationParticipant{
		{ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: conversationID, SubjectID: userSubject.ID, Role: string(domain.ConversationParticipantRoleMember)},
		{ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: conversationID, SubjectID: agentSubject.ID, Role: string(domain.ConversationParticipantRoleMember)},
	}
	if _, err := tx.NewInsert().Model(&participants).Column("id", "organization_id", "conversation_id", "subject_id", "role").Exec(ctx); err != nil {
		return fmt.Errorf("create AI conversation participants: %w", err)
	}
	return nil
}

// lockAgentConversationDraft 锁定已有草稿编号并核对首次发送的业务归属。
func lockAgentConversationDraft(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID, agentID string) (bool, error) {
	cv, err := chatstate.LockConversation(ctx, tx, identity.Organization.ID, conversationID)
	if errors.Is(err, chatstate.ErrConversationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	matches, err := tx.NewSelect().Model((*servermodels.AgentConversation)(nil)).
		Where("ac.conversation_id = ? AND ac.organization_id = ? AND ac.user_identity_id = ? AND ac.agent_identity_id = ?", conversationID, identity.Organization.ID, identity.OrganizationIdentity.ID, agentID).Exists(ctx)
	if err != nil {
		return false, err
	}
	if !matches || cv.Type != string(domain.ConversationTypeAgent) || cv.Status != string(domain.ConversationStatusActive) {
		return false, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	return true, nil
}

// SendAgentTextMessageAction 向已有 AI 聊天发送成员消息。
type SendAgentTextMessageAction struct {
	db        *bun.DB
	scheduler AgentChatMessageScheduler
}

// NewSendAgentTextMessageAction 创建已有 AI 聊天发送操作。
func NewSendAgentTextMessageAction(db *bun.DB, scheduler AgentChatMessageScheduler) *SendAgentTextMessageAction {
	return &SendAgentTextMessageAction{db: db, scheduler: scheduler}
}

// Execute 校验 AI 会话归属并在事务中保存成员消息及执行输入。
func (a *SendAgentTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input InternalTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeInternalMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	var result ConversationMessage
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		sendContext, err := lockAgentSendContext(ctx, tx, identity, normalized.ConversationID)
		if err != nil {
			return err
		}
		result, err = saveInternalTextMessage(ctx, tx, identity, normalized, sendContext, a.scheduler)
		return err
	})
	return result, err
}

// lockAgentSendContext 锁定会话与成员后复核 AI 会话归属和发送资格。
func lockAgentSendContext(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, conversationID string) (internalMessageContext, error) {
	row := internalMessageContext{}
	member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
	if err != nil {
		return row, err
	}
	err = tx.NewSelect().TableExpr("agent_conversations AS ac").
		ColumnExpr("ac.conversation_id, mine.id AS participant_id, mine.subject_id, ac.agent_identity_id, agent.active_revision_id AS agent_revision_id").
		Join("JOIN conversations AS cv ON cv.id = ac.conversation_id AND cv.organization_id = ac.organization_id").
		Join("JOIN chat_subjects AS user_cs ON user_cs.organization_id = ac.organization_id AND user_cs.kind = ? AND user_cs.source_id = ac.user_identity_id", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN conversation_participants AS mine ON mine.organization_id = ac.organization_id AND mine.conversation_id = ac.conversation_id AND mine.subject_id = user_cs.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS agent_cs ON agent_cs.organization_id = ac.organization_id AND agent_cs.kind = ? AND agent_cs.source_id = ac.agent_identity_id", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN conversation_participants AS peer ON peer.organization_id = ac.organization_id AND peer.conversation_id = ac.conversation_id AND peer.subject_id = agent_cs.id AND peer.left_at IS NULL").
		Join("JOIN agents AS agent ON agent.organization_id = ac.organization_id AND agent.identity_id = ac.agent_identity_id").
		Where("ac.organization_id = ? AND ac.conversation_id = ? AND ac.user_identity_id = ?", identity.Organization.ID, conversationID, identity.OrganizationIdentity.ID).
		Where("cv.type = ? AND cv.status = ? AND agent.status = ?", domain.ConversationTypeAgent, domain.ConversationStatusActive, domain.UserStatusActive).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrConversationNotFound
	}
	if err != nil {
		return row, fmt.Errorf("load AI conversation send context: %w", err)
	}
	row.Conversation = member.Conversation
	return row, nil
}
