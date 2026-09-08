//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type customerRunPolicy struct {
	enqueuer servertask.TxEnqueuer
}

// lockContext 锁定客户 Agent 所属会话的当前客服周期。
func (p customerRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	conversation, err := chatstate.LockCustomerConversation(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	session, err := chatstate.LockCurrentServiceSession(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	return agentRunPolicyContext{Conversation: conversation, ServiceSession: session}, nil
}

// prepareLocked 校验客户运行仍属于当前负责人，并收敛已经失效的运行。
func (p customerRunPolicy) prepareLocked(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun) (bool, error) {
	// 校验运行仍属于当前开放周期和有效 AI 客服。
	session := policyContext.ServiceSession
	eligible := false
	if run.ServiceSessionID != nil && *run.ServiceSessionID == session.ID &&
		domain.ServiceSessionStatus(session.Status) == domain.ServiceSessionStatusOpen &&
		session.AssigneeIdentityID != nil && *session.AssigneeIdentityID == run.AgentIdentityID {
		_, current, err := loadCustomerAgentEligibility(ctx, db, session, run.AgentRevisionID)
		if err != nil {
			return false, err
		}
		eligible = current
	}
	if eligible {
		return true, nil
	}
	if err := suppressCustomerRun(ctx, db, run, policyContext.ServiceSession); err != nil {
		return false, err
	}
	return false, nil
}

// loadMessages 读取本轮客服周期内的模型上下文。
func (p customerRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	return loadClaimedCustomerMessages(ctx, db, run, endSeq)
}

// persistMessage 追加客服 Agent 结果并记录有效首响。
func (p customerRunPolicy) persistMessage(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID string, messageType domain.MessageType, content string) error {
	participantID, err := ensureCustomerAgentParticipant(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
	if err != nil {
		return err
	}
	message, inserted, err := appendAgentMessage(
		ctx, db, policyContext.Conversation, run, messageID, participantID, messageType, content, &policyContext.ServiceSession.ID,
	)
	if err != nil || !inserted {
		return err
	}
	// 只有推进周期摘要的正常回复记录首响，失败消息不计入首响。
	if message.Type == string(domain.MessageTypeText) {
		if _, err := db.NewUpdate().Model(policyContext.ServiceSession).
			Set("first_response_at = COALESCE(first_response_at, ?)", message.OriginatedAt).
			WherePK().Where("organization_id = ? AND status = ? AND last_message_id = ?", message.OrganizationID, domain.ServiceSessionStatusOpen, message.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("record customer agent first response: %w", err)
		}
	}
	return nil
}

// enqueueNext 在当前负责人仍合格时投递客户 Agent 的剩余输入。
func (p customerRunPolicy) enqueueNext(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, startSeq int64) error {
	eligibility, eligible, err := loadCustomerAgentEligibility(ctx, db, policyContext.ServiceSession, "")
	if err != nil {
		return err
	}
	if !eligible {
		return nil
	}
	_, err = insertAndEnqueueRun(ctx, db, p.enqueuer, agentRunSpec{
		OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
		AgentIdentityID: run.AgentIdentityID, RevisionID: eligibility.RevisionID,
		TriggerType:      domain.AgentTriggerTypeCustomerAuto,
		ServiceSessionID: &policyContext.ServiceSession.ID,
	}, startSeq)
	return err
}

type customerMessageRow struct {
	ID               string  `bun:"id"`
	ReplyToMessageID *string `bun:"reply_to_message_id"`
	ReplyDeleted     bool    `bun:"reply_deleted"`
	ReplyBody        string  `bun:"reply_body"`
	ReplySenderKind  string  `bun:"reply_sender_kind"`
	ReplySenderID    string  `bun:"reply_sender_id"`
	ReplySenderName  string  `bun:"reply_sender_name"`
	Body             string  `bun:"body"`
	Kind             string  `bun:"kind"`
}

type customerMessageReference struct {
	MessageID      string `json:"messageId"`
	Deleted        bool   `json:"deleted,omitempty"`
	SenderKind     string `json:"senderKind,omitempty"`
	SenderSourceID string `json:"senderSourceId,omitempty"`
	SenderName     string `json:"senderName,omitempty"`
	Body           string `json:"body,omitempty"`
}

// loadClaimedCustomerMessages 读取本轮客服周期内不越过已认领 Trigger 的消息。
func loadClaimedCustomerMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	boundary, err := loadClaimedMessageBoundary(ctx, db, run, endSeq)
	if err != nil {
		return nil, err
	}
	rows := make([]customerMessageRow, 0, agentHistoryLimit)
	// 仅筛选主消息的客服周期；当前消息主动引用的旧周期原文仍作为一层引用传入。
	if err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id, msg.body, cs.kind").
		ColumnExpr("msg.reply_to_message_id").
		ColumnExpr("reply.deleted_at IS NOT NULL AS reply_deleted").
		ColumnExpr("? AS reply_body", messagequery.Summary("reply")).
		ColumnExpr("reply_cs.kind AS reply_sender_kind, reply_cs.source_id AS reply_sender_id").
		ColumnExpr("CASE WHEN reply_cs.kind = ? THEN COALESCE(reply_cci.display_name, reply_c.display_name) ELSE reply_oi.display_name END AS reply_sender_name", domain.ChatSubjectKindContact).
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN messages AS reply ON reply.id = msg.reply_to_message_id AND reply.organization_id = msg.organization_id AND reply.conversation_id = msg.conversation_id AND reply.type IN (?, ?)", domain.MessageTypeText, domain.MessageTypeAttachment).
		Join("LEFT JOIN conversation_participants AS reply_cp ON reply_cp.id = reply.sender_participant_id AND reply_cp.organization_id = reply.organization_id AND reply_cp.conversation_id = reply.conversation_id").
		Join("LEFT JOIN chat_subjects AS reply_cs ON reply_cs.id = reply_cp.subject_id AND reply_cs.organization_id = reply_cp.organization_id").
		Join("LEFT JOIN organization_identities AS reply_oi ON reply_oi.id = reply_cs.source_id AND reply_oi.organization_id = reply_cs.organization_id AND reply_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN customer_conversations AS cc ON cc.conversation_id = msg.conversation_id AND cc.organization_id = msg.organization_id").
		Join("LEFT JOIN contact_channel_identities AS reply_cci ON reply_cci.id = cc.contact_channel_identity_id AND reply_cci.organization_id = cc.organization_id AND reply_cci.contact_id = reply_cs.source_id AND reply_cs.kind = ?", domain.ChatSubjectKindContact).
		Join("LEFT JOIN contacts AS reply_c ON reply_c.id = reply_cs.source_id AND reply_c.organization_id = reply_cs.organization_id AND reply_cs.kind = ?", domain.ChatSubjectKindContact).
		Where("msg.organization_id = ?", run.OrganizationID).
		Where("msg.conversation_id = ?", run.ConversationID).
		Where("msg.service_session_id = ?", run.ServiceSessionID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("cs.kind IN (?, ?)", domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.message_seq <= ?", boundary.MessageSeq).
		OrderExpr("msg.message_seq DESC").
		Limit(agentHistoryLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load claimed customer conversation context: %w", err)
	}
	slices.Reverse(rows)
	messages := make([]agentruntime.Message, 0, len(rows))
	for _, row := range rows {
		role := agentruntime.MessageRoleAssistant
		if domain.ChatSubjectKind(row.Kind) == domain.ChatSubjectKindContact {
			role = agentruntime.MessageRoleUser
		}
		content := row.Body
		// 引用保留一层原文和真实主体类型，不增加模型对话角色。
		if row.ReplyToMessageID != nil {
			reference := customerMessageReference{MessageID: *row.ReplyToMessageID, Deleted: row.ReplyDeleted}
			if !row.ReplyDeleted {
				reference.Body, reference.SenderKind = row.ReplyBody, row.ReplySenderKind
				reference.SenderSourceID, reference.SenderName = row.ReplySenderID, row.ReplySenderName
			}
			encoded, _ := json.Marshal(struct {
				Body    string                   `json:"body"`
				ReplyTo customerMessageReference `json:"replyTo"`
			}{Body: row.Body, ReplyTo: reference})
			content = string(encoded)
		}
		messages = append(messages, agentruntime.Message{ID: row.ID, Role: role, Content: content})
	}
	return messages, nil
}

// suppressCustomerRun 把资格变化后的客户运行收敛为取消终态。
func suppressCustomerRun(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, session *servermodels.ServiceSession) error {
	var errorCode any
	lastError := "customer agent eligibility changed"
	switch {
	case domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen:
		errorCode = domain.AgentRunErrorCodeSessionClosed
		lastError = "customer service session closed"
	case run.ServiceSessionID == nil || *run.ServiceSessionID != session.ID ||
		session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != run.AgentIdentityID:
		errorCode = domain.AgentRunErrorCodeAssigneeChanged
		lastError = "customer service assignee changed"
	default:
		errorCode = nil
	}
	_, err := db.NewUpdate().Model(run).
		Set("status = ?", domain.AgentRunStatusCancelled).
		Set("error_code = ?", errorCode).
		Set("last_error = ?", lastError).
		Set("completed_at = now()").
		Set("updated_at = now()").
		WherePK().
		Where("status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("suppress customer agent run: %w", err)
	}
	return nil
}

// ensureCustomerAgentParticipant 取得或创建客户会话中的 Agent 参与者。
func ensureCustomerAgentParticipant(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (string, error) {
	subject, err := chatstate.EnsureOrganizationIdentityChatSubject(ctx, db, organizationID, agentIdentityID, uuid.NewV7().String())
	if err != nil {
		return "", err
	}
	participant := &servermodels.ConversationParticipant{}
	err = db.NewSelect().Model(participant).
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.subject_id = ?", subject.ID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		participant = &servermodels.ConversationParticipant{
			ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversationID,
			SubjectID: subject.ID, Role: string(domain.ConversationParticipantRoleMember),
		}
		if _, err := db.NewInsert().Model(participant).
			Column("id", "organization_id", "conversation_id", "subject_id", "role").Exec(ctx); err != nil {
			return "", fmt.Errorf("create customer agent conversation participant: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("load customer agent conversation participant: %w", err)
	} else if participant.LeftAt != nil {
		if _, err := db.NewUpdate().Model(participant).
			Set("left_at = NULL").
			Set("role = ?", domain.ConversationParticipantRoleMember).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return "", fmt.Errorf("restore customer agent conversation participant: %w", err)
		}
	}
	return participant.ID, nil
}
