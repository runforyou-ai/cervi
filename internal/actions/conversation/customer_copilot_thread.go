//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CustomerCopilotThread 定义客户会话中 Copilot 线程的摘要。
type CustomerCopilotThread struct {
	ID                  string            `bun:"id"`
	Title               string            `bun:"title"`
	AgentIdentityID     string            `bun:"agent_identity_id"`
	AgentName           string            `bun:"agent_name"`
	AgentAvatarFileID   *string           `bun:"agent_avatar_file_id"`
	AgentStatus         domain.UserStatus `bun:"agent_status"`
	CreatedByIdentityID string            `bun:"created_by_identity_id"`
	CreatedByName       string            `bun:"created_by_name"`
	CreatedAt           time.Time         `bun:"created_at"`
	LastActivityAt      time.Time         `bun:"last_activity_at"`
}

// FirstCustomerCopilotMessageInput 定义新线程的稳定编号、所属客户会话、回答的 AI 员工和首条提问。
type FirstCustomerCopilotMessageInput struct {
	ThreadID               string
	CustomerConversationID string
	AgentIdentityID        string
	ClientMessageID        string
	Body                   string
}

// FirstCustomerCopilotMessageResult 定义首条提问确认的线程和消息。
type FirstCustomerCopilotMessageResult struct {
	Thread  CustomerCopilotThread
	Message ConversationMessage
}

// SendFirstCustomerCopilotMessageAction 以首条提问原子创建 Copilot 线程。
type SendFirstCustomerCopilotMessageAction struct {
	db        *bun.DB
	scheduler AgentChatMessageScheduler
}

// NewSendFirstCustomerCopilotMessageAction 创建 Copilot 线程首条提问操作。
func NewSendFirstCustomerCopilotMessageAction(db *bun.DB, scheduler AgentChatMessageScheduler) *SendFirstCustomerCopilotMessageAction {
	return &SendFirstCustomerCopilotMessageAction{db: db, scheduler: scheduler}
}

// Execute 按线程稳定编号创建线程并幂等保存首条提问。
func (a *SendFirstCustomerCopilotMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input FirstCustomerCopilotMessageInput) (FirstCustomerCopilotMessageResult, error) {
	messageInput, fields := normalizeInternalMessageInput(InternalTextMessageInput{ConversationID: input.ThreadID, ClientMessageID: input.ClientMessageID, Body: input.Body})
	customerConversationID, valid := common.NormalizeUUID(input.CustomerConversationID)
	if !valid {
		fields["customerConversationId"] = ValidationConversationIDInvalid
	}
	agentID, valid := common.NormalizeUUID(input.AgentIdentityID)
	if !valid {
		fields["agentIdentityId"] = ValidationTargetIdentityIDInvalid
	}
	if len(fields) > 0 {
		return FirstCustomerCopilotMessageResult{}, &ValidationError{Fields: fields}
	}
	var result FirstCustomerCopilotMessageResult
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := ensureCustomerCopilotThread(ctx, tx, identity, messageInput.ConversationID, customerConversationID, agentID, messageInput.Body); err != nil {
			return err
		}
		sendContext, err := lockCustomerCopilotSendContext(ctx, tx, identity, messageInput.ConversationID)
		if err != nil {
			return err
		}
		result.Message, err = saveInternalTextMessage(ctx, tx, identity, messageInput, sendContext, a.scheduler)
		if err != nil {
			return err
		}
		threads, err := loadCustomerCopilotThreads(ctx, tx, identity.Organization.ID, customerConversationID, messageInput.ConversationID)
		if err != nil {
			return err
		}
		if len(threads) != 1 {
			return ErrDataInvariant
		}
		result.Thread = threads[0]
		return nil
	})
	if err != nil {
		return FirstCustomerCopilotMessageResult{}, err
	}
	return result, nil
}

// SendCustomerCopilotTextMessageAction 向已有 Copilot 线程发送成员提问。
type SendCustomerCopilotTextMessageAction struct {
	db        *bun.DB
	scheduler AgentChatMessageScheduler
}

// NewSendCustomerCopilotTextMessageAction 创建 Copilot 线程提问操作。
func NewSendCustomerCopilotTextMessageAction(db *bun.DB, scheduler AgentChatMessageScheduler) *SendCustomerCopilotTextMessageAction {
	return &SendCustomerCopilotTextMessageAction{db: db, scheduler: scheduler}
}

// Execute 校验线程归属与 AI 员工可用后，在事务中保存提问及执行输入。
func (a *SendCustomerCopilotTextMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input InternalTextMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeInternalMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	var result ConversationMessage
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		sendContext, err := lockCustomerCopilotSendContext(ctx, tx, identity, normalized.ConversationID)
		if err != nil {
			return err
		}
		result, err = saveInternalTextMessage(ctx, tx, identity, normalized, sendContext, a.scheduler)
		return err
	})
	return result, err
}

// ListCustomerCopilotThreadsQuery 读取客户会话中的 Copilot 线程。
type ListCustomerCopilotThreadsQuery struct {
	db *bun.DB
}

// NewListCustomerCopilotThreadsQuery 创建 Copilot 线程列表查询。
func NewListCustomerCopilotThreadsQuery(db *bun.DB) *ListCustomerCopilotThreadsQuery {
	return &ListCustomerCopilotThreadsQuery{db: db}
}

// Execute 校验客户会话属于当前企业后，按最近活动倒序返回全部线程。
func (q *ListCustomerCopilotThreadsQuery) Execute(ctx context.Context, identity *servermodels.Identity, customerConversationID string) ([]CustomerCopilotThread, error) {
	if !common.ValidUUID(customerConversationID) {
		return nil, ErrConversationNotFound
	}
	var threads []CustomerCopilotThread
	err := q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := authorizeConversationHistory(ctx, tx, identity, customerConversationID); err != nil {
			return err
		}
		var err error
		threads, err = loadCustomerCopilotThreads(ctx, tx, identity.Organization.ID, customerConversationID, "")
		return err
	})
	if err != nil {
		return nil, err
	}
	return threads, nil
}

// loadCustomerCopilotThreads 读取客户会话的线程摘要，threadID 非空时只读取该线程。
func loadCustomerCopilotThreads(ctx context.Context, db bun.IDB, organizationID, customerConversationID, threadID string) ([]CustomerCopilotThread, error) {
	threads := make([]CustomerCopilotThread, 0)
	query := db.NewSelect().
		TableExpr("customer_copilot_threads AS cct").
		ColumnExpr("cv.id::text AS id, COALESCE(cv.title, '') AS title").
		ColumnExpr("cct.agent_identity_id::text AS agent_identity_id, agent_oi.display_name AS agent_name, agent_oi.avatar_file_id::text AS agent_avatar_file_id, agent.status AS agent_status").
		ColumnExpr("cct.created_by_identity_id::text AS created_by_identity_id, creator.display_name AS created_by_name").
		ColumnExpr("cct.created_at, COALESCE(cv.last_activity_at, cct.created_at) AS last_activity_at").
		Join("JOIN conversations AS cv ON cv.organization_id = cct.organization_id AND cv.id = cct.conversation_id AND cv.type = ?", domain.ConversationTypeCopilot).
		Join("JOIN organization_identities AS agent_oi ON agent_oi.organization_id = cct.organization_id AND agent_oi.id = cct.agent_identity_id").
		Join("JOIN agents AS agent ON agent.organization_id = cct.organization_id AND agent.identity_id = cct.agent_identity_id").
		Join("JOIN organization_identities AS creator ON creator.organization_id = cct.organization_id AND creator.id = cct.created_by_identity_id").
		Where("cct.organization_id = ? AND cct.customer_conversation_id = ?", organizationID, customerConversationID).
		OrderExpr("COALESCE(cv.last_activity_at, cct.created_at) DESC, cv.id DESC")
	if threadID != "" {
		query = query.Where("cct.conversation_id = ?", threadID)
	}
	if err := query.Scan(ctx, &threads); err != nil {
		return nil, fmt.Errorf("load customer copilot threads: %w", err)
	}
	return threads, nil
}

// ensureCustomerCopilotThread 创建线程及其 AI 员工参与者，重试时核对线程的固定归属。
func ensureCustomerCopilotThread(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, threadID, customerConversationID, agentID, body string) error {
	if found, err := lockCustomerCopilotThreadDraft(ctx, tx, identity, threadID, customerConversationID, agentID); err != nil || found {
		return err
	}
	customerExists, err := tx.NewSelect().TableExpr("customer_conversations AS cc").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id AND cv.type = ?", domain.ConversationTypeCustomer).
		Where("cc.organization_id = ? AND cc.conversation_id = ?", identity.Organization.ID, customerConversationID).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check copilot customer conversation: %w", err)
	}
	if !customerExists {
		return ErrConversationNotFound
	}
	agentAvailable, err := tx.NewSelect().TableExpr("agents AS agent").
		Join("JOIN organization_identities AS oi ON oi.organization_id = agent.organization_id AND oi.id = agent.identity_id AND oi.type = ?", domain.OrganizationIdentityTypeAgent).
		Where("agent.organization_id = ? AND agent.identity_id = ? AND agent.status = ?", identity.Organization.ID, agentID, domain.UserStatusActive).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check copilot agent: %w", err)
	}
	if !agentAvailable {
		return ErrAgentUnavailable
	}
	subjects, err := ensureOrganizationIdentityChatSubjects(ctx, tx, identity.Organization.ID, []string{identity.OrganizationIdentity.ID, agentID})
	if err != nil {
		return err
	}
	// 线程标题取首条提问正文的前 40 个字符。
	title := []rune(strings.Join(strings.Fields(body), " "))
	if len(title) > 40 {
		title = title[:40]
	}
	titleText := string(title)
	cv := &servermodels.Conversation{
		ID: threadID, OrganizationID: identity.Organization.ID, Type: string(domain.ConversationTypeCopilot), Status: string(domain.ConversationStatusActive),
		Title: &titleText, CreatedBySubjectID: &subjects[identity.OrganizationIdentity.ID].ID,
	}
	inserted, err := tx.NewInsert().Model(cv).Column("id", "organization_id", "type", "status", "title", "created_by_subject_id").On("CONFLICT (id) DO NOTHING").Exec(ctx)
	if err != nil {
		return fmt.Errorf("create copilot thread conversation: %w", err)
	}
	count, err := inserted.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		found, err := lockCustomerCopilotThreadDraft(ctx, tx, identity, threadID, customerConversationID, agentID)
		if err != nil {
			return err
		}
		if !found {
			return &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		return nil
	}
	thread := &servermodels.CustomerCopilotThread{
		ConversationID: threadID, OrganizationID: identity.Organization.ID, CustomerConversationID: customerConversationID,
		AgentIdentityID: agentID, CreatedByIdentityID: identity.OrganizationIdentity.ID,
	}
	if _, err := tx.NewInsert().Model(thread).Column("conversation_id", "organization_id", "customer_conversation_id", "agent_identity_id", "created_by_identity_id").Exec(ctx); err != nil {
		return fmt.Errorf("create customer copilot thread: %w", err)
	}
	participant := &servermodels.ConversationParticipant{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: threadID,
		SubjectID: subjects[agentID].ID, Role: string(domain.ConversationParticipantRoleMember),
	}
	if _, err := tx.NewInsert().Model(participant).Column("id", "organization_id", "conversation_id", "subject_id", "role").Exec(ctx); err != nil {
		return fmt.Errorf("create copilot thread agent participant: %w", err)
	}
	return nil
}

// lockCustomerCopilotThreadDraft 锁定已有线程编号并核对首条提问的所属客户会话、AI 员工和创建人。
func lockCustomerCopilotThreadDraft(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, threadID, customerConversationID, agentID string) (bool, error) {
	cv, err := chatstate.LockConversation(ctx, tx, identity.Organization.ID, threadID)
	if errors.Is(err, chatstate.ErrConversationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	matches, err := tx.NewSelect().Model((*servermodels.CustomerCopilotThread)(nil)).
		Where("cct.organization_id = ? AND cct.conversation_id = ?", identity.Organization.ID, threadID).
		Where("cct.customer_conversation_id = ? AND cct.agent_identity_id = ? AND cct.created_by_identity_id = ?", customerConversationID, agentID, identity.OrganizationIdentity.ID).
		Exists(ctx)
	if err != nil {
		return false, err
	}
	if !matches || cv.Type != string(domain.ConversationTypeCopilot) {
		return false, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	return true, nil
}

// lockCustomerCopilotSendContext 锁定线程后校验所属客户会话与 AI 员工可用，并锁定或创建提问成员的参与关系。
func lockCustomerCopilotSendContext(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, threadID string) (internalMessageContext, error) {
	row := internalMessageContext{}
	conversation, err := chatstate.LockConversation(ctx, tx, identity.Organization.ID, threadID)
	if err != nil {
		return row, err
	}
	if conversation.Type != string(domain.ConversationTypeCopilot) || conversation.Status != string(domain.ConversationStatusActive) {
		return row, ErrConversationNotFound
	}
	var thread struct {
		AgentIdentityID string  `bun:"agent_identity_id"`
		AgentRevisionID *string `bun:"agent_revision_id"`
		AgentActive     bool    `bun:"agent_active"`
	}
	err = tx.NewSelect().TableExpr("customer_copilot_threads AS cct").
		ColumnExpr("cct.agent_identity_id::text AS agent_identity_id, agent.active_revision_id::text AS agent_revision_id").
		ColumnExpr("agent.status = ? AS agent_active", domain.UserStatusActive).
		Join("JOIN customer_conversations AS cc ON cc.organization_id = cct.organization_id AND cc.conversation_id = cct.customer_conversation_id").
		Join("JOIN agents AS agent ON agent.organization_id = cct.organization_id AND agent.identity_id = cct.agent_identity_id").
		Where("cct.organization_id = ? AND cct.conversation_id = ?", identity.Organization.ID, threadID).
		Scan(ctx, &thread)
	if errors.Is(err, sql.ErrNoRows) {
		return row, ErrConversationNotFound
	}
	if err != nil {
		return row, fmt.Errorf("load copilot thread send context: %w", err)
	}
	if !thread.AgentActive {
		return row, ErrAgentUnavailable
	}
	subjects, err := ensureOrganizationIdentityChatSubjects(ctx, tx, identity.Organization.ID, []string{identity.OrganizationIdentity.ID})
	if err != nil {
		return row, err
	}
	subject := subjects[identity.OrganizationIdentity.ID]
	participant := &servermodels.ConversationParticipant{}
	err = tx.NewSelect().Model(participant).
		Where("cp.organization_id = ? AND cp.conversation_id = ? AND cp.subject_id = ?", identity.Organization.ID, threadID, subject.ID).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		// 成员首次提问时加入线程参与者。
		participant = &servermodels.ConversationParticipant{
			ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: threadID,
			SubjectID: subject.ID, Role: string(domain.ConversationParticipantRoleMember),
		}
		if _, err := tx.NewInsert().Model(participant).Column("id", "organization_id", "conversation_id", "subject_id", "role").Exec(ctx); err != nil {
			return row, fmt.Errorf("create copilot thread member participant: %w", err)
		}
	} else if err != nil {
		return row, fmt.Errorf("lock copilot thread member participant: %w", err)
	}
	return internalMessageContext{
		Conversation: conversation, ConversationID: threadID, ParticipantID: participant.ID, SubjectID: subject.ID,
		AgentIdentityID: thread.AgentIdentityID, AgentRevisionID: thread.AgentRevisionID, AgentInputKind: domain.AgentInputKindCopilot,
	}, nil
}
