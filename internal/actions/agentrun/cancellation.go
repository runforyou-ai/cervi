//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type runningAgentRun struct {
	cancel   context.CancelFunc
	attempt  int
	streamID string
	stream   *agentruntime.StreamHub
}

// CancelForServiceSession 在客服事务内取消原负责人尚未结束的运行。
func (a *ExecuteAction) CancelForServiceSession(ctx context.Context, db bun.IDB, organizationID, serviceSessionID, agentIdentityID string, reason domain.AgentRunErrorCode) ([]string, error) {
	return cancelServiceSessionRuns(ctx, db, organizationID, serviceSessionID, agentIdentityID, reason)
}

// cancelServiceSessionRuns 取消服务周期内负责人的在途运行并结算其输入队列。
func cancelServiceSessionRuns(ctx context.Context, db bun.IDB, organizationID, serviceSessionID, agentIdentityID string, reason domain.AgentRunErrorCode) ([]string, error) {
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

// CancelTelegramChannelRuns 在机器人更换事务中取消旧渠道输入并推进该渠道全部客户会话版本，调用方已锁定渠道和连接设置。
func CancelTelegramChannelRuns(ctx context.Context, db bun.IDB, organizationID, channelID string) (int, error) {
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
		conversation, session, err := chatstate.LockServiceSession(ctx, db, organizationID, conversationID)
		if err != nil {
			return 0, err
		}
		if session.AssigneeIdentityID != nil {
			runIDs, err := cancelServiceSessionRuns(ctx, db, organizationID, session.ID, *session.AssigneeIdentityID, domain.AgentRunErrorCodeBotChanged)
			if err != nil {
				return 0, err
			}
			cancelled += len(runIDs)
		}
		if err := chatstate.TouchConversation(ctx, db, conversation); err != nil {
			return 0, err
		}
	}
	return cancelled, nil
}

// CancelRunContexts 尽力取消本进程中正在执行的模型调用。
func (a *ExecuteAction) CancelRunContexts(runIDs []string) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	for _, runID := range runIDs {
		if running := a.runningRuns[runID]; running != nil {
			running.cancel()
		}
	}
}

// registerRunContext 注册一次可被客服负责人变化中断的模型调用。
func (a *ExecuteAction) registerRunContext(ctx context.Context, runID string, cancel context.CancelFunc) (*runningAgentRun, func(), error) {
	execution, _ := servertask.CurrentExecution(ctx)
	streamID := uuid.NewV7().String()
	running := &runningAgentRun{cancel: cancel, attempt: execution.Attempt, streamID: streamID,
		stream: agentruntime.NewStreamHub(agentruntime.StreamSnapshot{RunID: runID, StreamID: streamID, Attempt: execution.Attempt})}
	a.runningMu.Lock()
	if previous := a.runningRuns[runID]; previous != nil {
		if previous.attempt >= running.attempt {
			a.runningMu.Unlock()
			return nil, nil, servertask.ErrExecutionLost
		}
		previous.cancel()
	}
	a.runningRuns[runID] = running
	a.runningMu.Unlock()
	return running, func() {
		a.runningMu.Lock()
		if a.runningRuns[runID] == running {
			delete(a.runningRuns, runID)
		}
		a.runningMu.Unlock()
		running.stream.End()
	}, nil
}
