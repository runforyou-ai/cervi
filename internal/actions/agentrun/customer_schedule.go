//go:build server

// Package agentrun 实现 Agent 会话触发、执行与持久账本。
package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type customerAgentEligibility struct {
	RevisionID string `bun:"revision_id"`
}

// ScheduleCustomerAuto 把一条客户消息追加到当前 AI 客服的持久输入流。
func (s *Scheduler) ScheduleCustomerAuto(ctx context.Context, db bun.IDB, organizationID, conversationID, serviceSessionID, messageID string) (bool, error) {
	if s == nil || s.enqueuer == nil {
		return false, errors.New("agent run scheduler is unavailable")
	}
	session, err := chatstate.LockCustomerServiceSession(ctx, db, organizationID, conversationID)
	if err != nil {
		return false, err
	}
	if session.ID != serviceSessionID {
		return false, errors.New("customer agent service session is no longer current")
	}
	if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen || session.AssigneeIdentityID == nil {
		return false, nil
	}
	senderSubjectID, err := loadCustomerInputSender(ctx, db, session, messageID)
	if err != nil {
		return false, err
	}
	if senderSubjectID == "" {
		return false, errors.New("customer agent input message is invalid")
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
		slog.Warn("客户会话负责人不满足 Agent 执行资格",
			"organization_id", organizationID,
			"conversation_id", conversationID,
			"service_session_id", serviceSessionID,
			"assignee_identity_id", *session.AssigneeIdentityID,
		)
		return false, nil
	}

	if err := s.scheduleInput(ctx, db, agentRunSpec{
		OrganizationID: organizationID, ConversationID: conversationID,
		AgentIdentityID: *session.AssigneeIdentityID, RevisionID: eligibility.RevisionID,
		ScopeKind: domain.AgentExecutionScopeServiceSession, ScopeID: session.ID,
		Kind: domain.AgentInputKindCustomerAuto, SourceSubjectID: senderSubjectID,
	}, messageID); err != nil {
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

// loadCustomerInputSender 校验来源消息属于当前周期且来自客户，并返回其聊天主体。
func loadCustomerInputSender(ctx context.Context, db bun.IDB, session *servermodels.ServiceSession, messageID string) (string, error) {
	var subjectID string
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("cs.id").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.id = ?", messageID).
		Where("msg.organization_id = ?", session.OrganizationID).
		Where("msg.conversation_id = ?", session.ConversationID).
		Where("msg.service_session_id = ?", session.ID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL").
		Where("cs.kind = ?", domain.ChatSubjectKindContact).
		Scan(ctx, &subjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load customer agent input sender: %w", err)
	}
	return subjectID, nil
}

// loadCustomerAgentEligibility 校验当前负责人及指定运行 Revision 可以执行渠道客服会话。
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
		Join("JOIN channels AS c ON c.id = cci.channel_id AND c.organization_id = cci.organization_id").
		Join("LEFT JOIN telegram_channel_settings AS tcs ON tcs.channel_id = c.id AND tcs.organization_id = c.organization_id").
		Where("c.type = ? OR (c.type = ? AND tcs.bot_id IS NOT NULL)", domain.ChannelTypeWebsite, domain.ChannelTypeTelegram).
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
