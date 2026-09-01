//go:build server

// Package agentrun 实现 Agent 会话触发、执行与持久账本。
package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type customerAgentEligibility struct {
	RevisionID string `bun:"revision_id"`
}

// ScheduleCustomerAuto 把一条网站客户消息追加到当前 AI 客服的持久输入流。
func (s *Scheduler) ScheduleCustomerAuto(ctx context.Context, db bun.IDB, organizationID, conversationID, serviceSessionID, messageID string) (bool, error) {
	if s == nil || s.enqueuer == nil {
		return false, errors.New("agent run scheduler is unavailable")
	}
	session, err := lockLatestCustomerServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return false, err
	}
	if session.ID != serviceSessionID {
		return false, errors.New("customer agent service session is no longer current")
	}
	if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen || session.AssigneeIdentityID == nil {
		return false, nil
	}
	messageMatches, err := customerTriggerMessageMatches(ctx, db, session, messageID)
	if err != nil {
		return false, err
	}
	if !messageMatches {
		return false, errors.New("customer agent trigger message is invalid")
	}
	assigneeType, err := loadCustomerAssigneeType(ctx, db, session)
	if err != nil {
		return false, err
	}
	if assigneeType != domain.OrganizationIdentityTypeAgent {
		return false, nil
	}
	eligibility, eligible, err := loadCustomerAgentEligibility(ctx, db, session, "")
	if err != nil {
		return false, err
	}
	if !eligible {
		slog.Warn("网站客户会话负责人不满足 Agent 执行资格",
			"organization_id", organizationID,
			"conversation_id", conversationID,
			"service_session_id", serviceSessionID,
			"assignee_identity_id", *session.AssigneeIdentityID,
		)
		return false, nil
	}

	sequence, err := advanceCustomerAgentSequence(ctx, db, organizationID, conversationID, *session.AssigneeIdentityID)
	if err != nil {
		return false, err
	}
	trigger := &servermodels.ConversationAgentTrigger{
		ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: *session.AssigneeIdentityID, TriggerType: string(domain.AgentTriggerTypeCustomerAuto),
		ServiceSessionID: &session.ID, TriggerSeq: sequence.DesiredSeq, TriggerMessageID: messageID,
	}
	if _, err := db.NewInsert().Model(trigger).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "trigger_type", "service_session_id", "trigger_seq", "trigger_message_id").
		Exec(ctx); err != nil {
		return false, fmt.Errorf("create customer agent trigger: %w", err)
	}

	active := &servermodels.AgentRun{}
	err = db.NewSelect().Model(active).
		Where("agr.organization_id = ?", organizationID).
		Where("agr.conversation_id = ?", conversationID).
		Where("agr.agent_identity_id = ?", *session.AssigneeIdentityID).
		Where("agr.status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Scan(ctx)
	if err == nil {
		if active.TriggerType != string(domain.AgentTriggerTypeCustomerAuto) || active.ServiceSessionID == nil || *active.ServiceSessionID != session.ID {
			return false, errors.New("active agent run does not match customer service session")
		}
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("check active customer agent run: %w", err)
	}
	if _, err := insertAndEnqueueCustomerRun(
		ctx, db, s.enqueuer, organizationID, conversationID, *session.AssigneeIdentityID,
		eligibility.RevisionID, session.ID, sequence.ProcessedSeq+1,
	); err != nil {
		return false, err
	}
	return true, nil
}

// loadCustomerAssigneeType 读取当前客服负责人的企业身份类型。
func loadCustomerAssigneeType(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession) (domain.OrganizationIdentityType, error) {
	if session.AssigneeIdentityID == nil {
		return "", nil
	}
	var identityType string
	if err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).
		Column("type").
		Where("oi.organization_id = ?", session.OrganizationID).
		Where("oi.id = ?", *session.AssigneeIdentityID).
		Scan(ctx, &identityType); err != nil {
		return "", fmt.Errorf("load customer assignee identity type: %w", err)
	}
	return domain.OrganizationIdentityType(identityType), nil
}

// advanceCustomerAgentSequence 分配客户 Agent 的下一条连续触发序号。
func advanceCustomerAgentSequence(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string) (conversationSequence, error) {
	sequence := conversationSequence{}
	err := db.NewRaw(`
		INSERT INTO conversation_agent_states (
			conversation_id, organization_id, agent_identity_id, desired_seq, processed_seq
		)
		VALUES (?, ?, ?, 1, 0)
		ON CONFLICT (conversation_id, agent_identity_id) DO UPDATE
		SET desired_seq = conversation_agent_states.desired_seq + 1,
			updated_at = now()
		WHERE conversation_agent_states.organization_id = EXCLUDED.organization_id
		RETURNING desired_seq, processed_seq
	`, conversationID, organizationID, agentIdentityID).Scan(ctx, &sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return conversationSequence{}, errors.New("customer agent state does not match target agent")
	}
	if err != nil {
		return conversationSequence{}, fmt.Errorf("advance customer agent input sequence: %w", err)
	}
	return sequence, nil
}

// insertAndEnqueueCustomerRun 创建客户 Agent 运行并投递隔离 Worker。
func insertAndEnqueueCustomerRun(ctx context.Context, db bun.IDB, enqueuer servertask.TxEnqueuer, organizationID, conversationID, agentIdentityID, revisionID, serviceSessionID string, startSeq int64) (string, error) {
	run := &servermodels.AgentRun{
		ID: uuid.NewV7().String(), OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: agentIdentityID, AgentRevisionID: revisionID,
		TriggerType: string(domain.AgentTriggerTypeCustomerAuto), ServiceSessionID: &serviceSessionID,
		Status: string(domain.AgentRunStatusQueued), TriggerStartSeq: startSeq,
	}
	if _, err := db.NewInsert().Model(run).
		Column("id", "organization_id", "conversation_id", "agent_identity_id", "agent_revision_id", "trigger_type", "service_session_id", "status", "trigger_start_seq").
		Exec(ctx); err != nil {
		return "", fmt.Errorf("create customer agent run: %w", err)
	}
	if _, err := enqueuer.EnqueueIn(ctx, db, RunActionName, RunInput{RunID: run.ID}, servertask.EnqueueOptions{
		Queue: servertask.QueueAgent, MaxAttempts: 3,
		IdempotencyKey: "agent:" + run.ID,
		TriggerType:    servertask.TriggerBusiness,
	}); err != nil {
		return "", fmt.Errorf("enqueue customer agent run: %w", err)
	}
	return run.ID, nil
}

// lockLatestCustomerServiceSession 锁定会话当前最新客服处理周期。
func lockLatestCustomerServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.ServiceSession, error) {
	session := &servermodels.ServiceSession{}
	err := db.NewSelect().Model(session).
		Where("ss.organization_id = ?", organizationID).
		Where("ss.conversation_id = ?", conversationID).
		OrderExpr("ss.sequence DESC").
		Limit(1).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock latest customer service session: %w", err)
	}
	return session, nil
}

// customerTriggerMessageMatches 校验触发消息属于当前周期且来自客户。
func customerTriggerMessageMatches(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, messageID string) (bool, error) {
	matched, err := db.NewSelect().
		TableExpr("messages AS msg").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.id = ?", messageID).
		Where("msg.organization_id = ?", session.OrganizationID).
		Where("msg.conversation_id = ?", session.ConversationID).
		Where("msg.service_session_id = ?", session.ID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("cs.kind = ?", domain.ChatSubjectKindContact).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("validate customer agent trigger message: %w", err)
	}
	return matched, nil
}

// loadCustomerAgentEligibility 校验当前负责人及指定运行 Revision 可以执行网站客服会话。
func loadCustomerAgentEligibility(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, runRevisionID string) (customerAgentEligibility, bool, error) {
	if session.AssigneeIdentityID == nil {
		return customerAgentEligibility{}, false, nil
	}
	row := customerAgentEligibility{}
	query := db.NewSelect().
		TableExpr("organization_identities AS oi").
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id AND r.kind = ?", domain.RoleKindCustomerService).
		Join("JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id AND a.status = ?", domain.UserStatusActive).
		Join("JOIN contact_channel_identities AS cci ON cci.id = ? AND cci.organization_id = oi.organization_id", session.ContactChannelIdentityID).
		Join("JOIN channels AS c ON c.id = cci.channel_id AND c.organization_id = cci.organization_id AND c.type = ?", domain.ChannelTypeWebsite).
		Where("oi.organization_id = ?", session.OrganizationID).
		Where("oi.id = ?", *session.AssigneeIdentityID).
		Where("oi.type = ?", domain.OrganizationIdentityTypeAgent)
	if runRevisionID == "" {
		query = query.
			ColumnExpr("a.active_revision_id AS revision_id").
			Join("JOIN agent_revisions AS ar ON ar.id = a.active_revision_id AND ar.agent_id = a.id AND ar.organization_id = a.organization_id AND ar.execution_mode = ? AND ar.schema_version = 1", domain.AgentExecutionModeManaged)
	} else {
		query = query.
			ColumnExpr("? AS revision_id", runRevisionID).
			Join("JOIN agent_revisions AS ar ON ar.id = ? AND ar.agent_id = a.id AND ar.organization_id = a.organization_id AND ar.execution_mode = ? AND ar.schema_version = 1", runRevisionID, domain.AgentExecutionModeManaged)
	}
	err := query.Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return customerAgentEligibility{}, false, nil
	}
	if err != nil {
		return customerAgentEligibility{}, false, fmt.Errorf("load customer agent eligibility: %w", err)
	}
	return row, true, nil
}
