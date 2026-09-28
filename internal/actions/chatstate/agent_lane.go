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

// CancelChannelRuns 取消渠道全部客户会话中负责人的在途运行并推进会话版本，调用方已锁定渠道。
func CancelChannelRuns(ctx context.Context, db bun.IDB, organizationID, channelID string, reason domain.AgentRunErrorCode) (int, error) {
	cancelled := 0
	var conversationIDs []string
	if err := db.NewSelect().TableExpr("channel_conversations AS cc").
		ColumnExpr("cc.conversation_id").
		Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Where("cc.organization_id = ? AND cci.channel_id = ?", organizationID, channelID).
		OrderExpr("cc.conversation_id").Scan(ctx, &conversationIDs); err != nil {
		return 0, err
	}
	for _, conversationID := range conversationIDs {
		locked, err := LockServiceSession(ctx, db, organizationID, conversationID)
		if err != nil {
			return 0, err
		}
		conversation, session := locked.Conversation, locked.Session
		if session.AssigneeIdentityID != nil {
			runIDs, err := CancelServiceSessionRuns(ctx, db, organizationID, session.ID, *session.AssigneeIdentityID, reason)
			if err != nil {
				return 0, err
			}
			cancelled += len(runIDs)
		}
		if err := TouchConversation(ctx, db, conversation, domain.ConversationChangeTimeline|domain.ConversationChangeService); err != nil {
			return 0, err
		}
	}
	return cancelled, nil
}
