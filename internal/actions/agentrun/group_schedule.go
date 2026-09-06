//go:build server

package agentrun

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ScheduleGroupMentions 在已锁定群聊的消息事务内触发被提醒的活跃 Agent。
func (s *Scheduler) ScheduleGroupMentions(ctx context.Context, db bun.IDB, organizationID, conversationID, messageID string, subjectIDs []string, mentionAll bool) error {
	if !mentionAll && len(subjectIDs) == 0 {
		return nil
	}
	var agents []struct {
		IdentityID string `bun:"identity_id"`
		RevisionID string `bun:"active_revision_id"`
	}
	query := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.identity_id, a.active_revision_id").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = a.organization_id AND cs.kind = ? AND cs.source_id = a.identity_id", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN conversation_participants AS cp ON cp.organization_id = cs.organization_id AND cp.subject_id = cs.id AND cp.conversation_id = ? AND cp.left_at IS NULL", conversationID).
		Where("a.organization_id = ? AND a.status = ?", organizationID, domain.UserStatusActive).
		OrderExpr("a.identity_id").For("SHARE OF a")
	if !mentionAll {
		query = query.Where("cs.id IN (?)", bun.In(subjectIDs))
	}
	if err := query.Scan(ctx, &agents); err != nil {
		return fmt.Errorf("load mentioned group agents: %w", err)
	}
	for _, agent := range agents {
		if err := s.scheduleInput(ctx, db, agentRunSpec{
			OrganizationID: organizationID, ConversationID: conversationID,
			AgentIdentityID: agent.IdentityID, RevisionID: agent.RevisionID,
			TriggerType: domain.AgentTriggerTypeMention,
		}, messageID); err != nil {
			return err
		}
	}
	return nil
}

// CancelGroupRuns 在已锁定群聊或目标 Agent 的事务内取消未完成的群聊运行。
func CancelGroupRuns(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string, reason domain.AgentRunErrorCode) error {
	var states []servermodels.ConversationAgentState
	query := db.NewSelect().Model(&states).
		Where("cas.organization_id = ? AND cas.conversation_id = ?", organizationID, conversationID).
		OrderExpr("cas.agent_identity_id").For("UPDATE")
	if agentIdentityID != "" {
		query = query.Where("cas.agent_identity_id = ?", agentIdentityID)
	}
	if err := query.Scan(ctx); err != nil {
		return fmt.Errorf("lock cancelled group agent states: %w", err)
	}
	for _, state := range states {
		if _, err := db.NewUpdate().Model((*servermodels.AgentRun)(nil)).
			Set("status = ?, error_code = ?, completed_at = now(), updated_at = now()", domain.AgentRunStatusCancelled, reason).
			Where("organization_id = ? AND conversation_id = ? AND agent_identity_id = ?", organizationID, conversationID, state.AgentIdentityID).
			Where("trigger_type = ? AND status IN (?, ?)", domain.AgentTriggerTypeMention, domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
			Exec(ctx); err != nil {
			return fmt.Errorf("cancel group agent runs: %w", err)
		}
		if _, err := db.NewUpdate().Model(&state).
			Set("processed_seq = desired_seq, updated_at = now()").WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("advance cancelled group agent state: %w", err)
		}
	}
	if len(states) > 0 {
		slog.Info("群聊 Agent 运行已取消", "conversation_id", conversationID, "agent_identity_id", agentIdentityID, "reason", reason)
	}
	return nil
}
