//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CancelServiceSessionRuns 取消服务周期内负责人的在途运行并结算其输入队列。
func CancelServiceSessionRuns(ctx context.Context, db bun.IDB, organizationID, serviceSessionID, agentIdentityID string, reason domain.AgentRunErrorCode) ([]string, error) {
	lane := &servermodels.AgentLane{}
	err := db.NewSelect().Model(lane).
		Where("al.organization_id = ?", organizationID).
		Where("al.scope_kind = ? AND al.scope_id = ?", domain.AgentExecutionScopeServiceSession, serviceSessionID).
		Where("al.agent_identity_id = ?", agentIdentityID).
		For("UPDATE").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock assigned agent lane: %w", err)
	}
	runIDs := make([]string, 0)
	if err := db.NewRaw(`
		UPDATE agent_runs
		SET status = ?, error_code = ?, completed_at = now(), updated_at = now()
		WHERE lane_id = ?
			AND status IN (?, ?)
		RETURNING id
	`, domain.AgentRunStatusCancelled, reason, lane.ID, domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Scan(ctx, &runIDs); err != nil {
		return nil, fmt.Errorf("cancel assigned agent runs: %w", err)
	}
	if _, err := db.NewUpdate().Model(lane).
		Set("processed_seq = desired_seq").
		Set("updated_at = now()").
		WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("advance cancelled agent lane: %w", err)
	}
	return runIDs, nil
}

// CancelChannelRuns 推进渠道内当前负责人有在途运行的客户会话版本，并批量取消这些运行、结算其输入队列，调用方已锁定渠道。
func CancelChannelRuns(ctx context.Context, db bun.IDB, organizationID, channelID string, reason domain.AgentRunErrorCode) (int, error) {
	// 渠道客户会话当前服务周期中负责人尚未结算的执行通道。
	activeLanes := func() *bun.SelectQuery {
		return db.NewSelect().TableExpr("agent_lanes AS al").
			Join("JOIN service_sessions AS ss ON ss.organization_id = al.organization_id AND ss.id = al.scope_id AND ss.assignee_identity_id = al.agent_identity_id").
			Join("JOIN service_conversations AS svc ON svc.organization_id = ss.organization_id AND svc.current_service_session_id = ss.id").
			Join("JOIN channel_conversations AS cc ON cc.organization_id = svc.organization_id AND cc.conversation_id = svc.conversation_id").
			Join("JOIN contact_channel_identities AS cci ON cci.organization_id = cc.organization_id AND cci.id = cc.contact_channel_identity_id").
			Where("al.organization_id = ? AND al.scope_kind = ? AND cci.channel_id = ?", organizationID, domain.AgentExecutionScopeServiceSession, channelID).
			Where("al.processed_seq < al.desired_seq OR EXISTS (SELECT 1 FROM agent_runs AS ar WHERE ar.lane_id = al.id AND ar.status IN (?, ?))", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning)
	}
	if err := TouchConversations(ctx, db, organizationID, activeLanes().Column("al.conversation_id"), domain.ConversationChangeTimeline|domain.ConversationChangeService); err != nil {
		return 0, err
	}
	laneIDs := make([]string, 0)
	if err := activeLanes().Column("al.id").OrderExpr("al.id").For("UPDATE OF al").Scan(ctx, &laneIDs); err != nil {
		return 0, fmt.Errorf("lock channel agent lanes: %w", err)
	}
	if len(laneIDs) == 0 {
		return 0, nil
	}
	runIDs := make([]string, 0)
	if err := db.NewRaw(`
		UPDATE agent_runs
		SET status = ?, error_code = ?, completed_at = now(), updated_at = now()
		WHERE lane_id IN (?)
			AND status IN (?, ?)
		RETURNING id
	`, domain.AgentRunStatusCancelled, reason, bun.In(laneIDs), domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Scan(ctx, &runIDs); err != nil {
		return 0, fmt.Errorf("cancel channel agent runs: %w", err)
	}
	if _, err := db.NewUpdate().Model((*servermodels.AgentLane)(nil)).
		Set("processed_seq = desired_seq").
		Set("updated_at = now()").
		Where("organization_id = ? AND id IN (?)", organizationID, bun.In(laneIDs)).
		Exec(ctx); err != nil {
		return 0, fmt.Errorf("advance cancelled channel agent lanes: %w", err)
	}
	return len(runIDs), nil
}
