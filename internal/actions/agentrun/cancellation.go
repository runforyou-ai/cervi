//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

type runningAgentRun struct {
	cancel   context.CancelFunc
	attempt  int
	progress agentruntime.Progress
}

// CancelForServiceSession 在客服事务内取消原负责人尚未结束的运行。
func (a *ExecuteAction) CancelForServiceSession(ctx context.Context, db bun.IDB, organizationID, conversationID, agentIdentityID string, reason domain.AgentRunErrorCode) ([]string, error) {
	agent, err := db.NewSelect().Model((*servermodels.Agent)(nil)).
		Where("a.organization_id = ?", organizationID).
		Where("a.identity_id = ?", agentIdentityID).
		Exists(ctx)
	if err != nil || !agent {
		return nil, err
	}
	state := &servermodels.ConversationAgentState{}
	stateExists := true
	if err := db.NewSelect().Model(state).
		Where("cas.organization_id = ?", organizationID).
		Where("cas.conversation_id = ?", conversationID).
		Where("cas.agent_identity_id = ?", agentIdentityID).
		For("UPDATE").
		Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		stateExists = false
	} else if err != nil {
		return nil, fmt.Errorf("lock assigned agent state: %w", err)
	}
	runIDs := make([]string, 0)
	if err := db.NewRaw(`
		UPDATE agent_runs
		SET status = ?, error_code = ?, completed_at = now(), updated_at = now()
		WHERE organization_id = ?
			AND conversation_id = ?
			AND agent_identity_id = ?
			AND status IN (?, ?)
		RETURNING id
	`, domain.AgentRunStatusCancelled, reason, organizationID, conversationID, agentIdentityID, domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Scan(ctx, &runIDs); err != nil {
		return nil, fmt.Errorf("cancel assigned agent runs: %w", err)
	}
	if stateExists {
		if _, err := db.NewUpdate().Model(state).
			Set("processed_seq = ?", state.DesiredSeq).
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("advance cancelled agent state: %w", err)
		}
	}
	return runIDs, nil
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
	running := &runningAgentRun{cancel: cancel, attempt: execution.Attempt, progress: agentruntime.Progress{RunID: runID, StreamID: uuid.NewV7().String(), Attempt: execution.Attempt}}
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
	}, nil
}

// Progress 返回本进程中当前执行尝试的快照，调用方负责会话访问校验。
func (a *ExecuteAction) Progress(runID string) (agentruntime.Progress, bool) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	running := a.runningRuns[runID]
	if running == nil {
		return agentruntime.Progress{}, false
	}
	return running.progress.Clone(), true
}
