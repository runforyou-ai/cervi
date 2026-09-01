//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type customerRunPolicy struct {
	enqueuer servertask.TxEnqueuer
}

// lockContext 锁定客户 Agent 所属会话的最新客服周期。
func (p customerRunPolicy) lockContext(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) (agentRunPolicyContext, error) {
	session, err := lockLatestCustomerServiceSession(ctx, db, run.OrganizationID, run.ConversationID)
	if err != nil {
		return agentRunPolicyContext{}, err
	}
	return agentRunPolicyContext{ServiceSession: session}, nil
}

// prepareLocked 校验客户运行仍属于当前负责人，并收敛已经失效的运行。
func (p customerRunPolicy) prepareLocked(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun) (bool, error) {
	eligible, err := validateCurrentCustomerRun(ctx, db, policyContext.ServiceSession, run)
	if err != nil {
		return false, err
	}
	if eligible {
		return true, nil
	}
	if err := suppressCustomerRun(ctx, db, run, policyContext.ServiceSession); err != nil {
		return false, err
	}
	return false, nil
}

// loadMessages 按客户和企业身份映射读取客服会话上下文。
func (p customerRunPolicy) loadMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	return loadClaimedCustomerMessages(ctx, db, run, endSeq)
}

// persistResponse 写入客户 Agent 回复并更新客服周期摘要。
func (p customerRunPolicy) persistResponse(ctx context.Context, db bun.IDB, policyContext agentRunPolicyContext, run *servermodels.AgentRun, messageID, content string) error {
	participantID, err := ensureCustomerAgentParticipant(ctx, db, run.OrganizationID, run.ConversationID, run.AgentIdentityID)
	if err != nil {
		return err
	}
	message, err := insertAgentResponseMessage(
		ctx, db, run, messageID, participantID, content, &policyContext.ServiceSession.ID,
	)
	if err != nil {
		return err
	}
	return updateCustomerAgentSummaries(ctx, db, policyContext.ServiceSession, message)
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
	Body string `bun:"body"`
	Kind string `bun:"kind"`
}

// loadClaimedCustomerMessages 读取不越过已认领 Trigger 的客户会话上下文。
func loadClaimedCustomerMessages(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, endSeq int64) ([]agentruntime.Message, error) {
	boundary, err := loadClaimedMessageBoundary(ctx, db, run, endSeq)
	if err != nil {
		return nil, err
	}
	rows := make([]customerMessageRow, 0, agentHistoryLimit)
	if err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.body, cs.kind").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.organization_id = ?", run.OrganizationID).
		Where("msg.conversation_id = ?", run.ConversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("cs.kind IN (?, ?)", domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity).
		Where("(msg.originated_at, msg.source_order, msg.id) <= (?, ?, ?)", boundary.OriginatedAt, boundary.SourceOrder, boundary.ID).
		OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC").
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
		messages = append(messages, agentruntime.Message{Role: role, Content: row.Body})
	}
	return messages, nil
}

// validateCurrentCustomerRun 校验运行仍属于当前开放周期和有效 AI 客服。
func validateCurrentCustomerRun(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, run *servermodels.AgentRun) (bool, error) {
	if run.ServiceSessionID == nil || *run.ServiceSessionID != session.ID ||
		domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen ||
		session.AssigneeIdentityID == nil || *session.AssigneeIdentityID != run.AgentIdentityID {
		return false, nil
	}
	_, eligible, err := loadCustomerAgentEligibility(ctx, db, session, run.AgentRevisionID)
	return eligible, err
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
	subject := &servermodels.ChatSubject{}
	err := db.NewSelect().Model(subject).
		Where("cs.organization_id = ?", organizationID).
		Where("cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("cs.source_id = ?", agentIdentityID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		subject = &servermodels.ChatSubject{
			ID: uuid.NewV7().String(), OrganizationID: organizationID,
			Kind: string(domain.ChatSubjectKindOrganizationIdentity), SourceID: agentIdentityID,
		}
		if _, err := db.NewInsert().Model(subject).
			Column("id", "organization_id", "kind", "source_id").Exec(ctx); err != nil {
			return "", fmt.Errorf("create customer agent chat subject: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("load customer agent chat subject: %w", err)
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

// updateCustomerAgentSummaries 更新客服首响和会话消息摘要。
func updateCustomerAgentSummaries(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, message *servermodels.Message) error {
	if _, err := db.NewUpdate().Model(session).
		Set("first_response_at = COALESCE(first_response_at, ?)", message.OriginatedAt).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", message.OrganizationID).
		Where("status = ?", domain.ServiceSessionStatusOpen).
		Where("(last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("update service session after customer agent response: %w", err)
	}
	return updateConversationAfterAgentResponse(ctx, db, message)
}
