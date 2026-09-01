//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/task"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const (
	agentRunTimeout       = 5 * time.Minute
	agentHistoryLimit     = 100
	agentMaxOutputTokens  = 4096
	agentRunErrorMaxRunes = 4000
)

// ExecuteAction 执行并收尾一次 Agent 业务运行。
type ExecuteAction struct {
	db          *bun.DB
	enqueuer    servertask.TxEnqueuer
	runtime     agentruntime.Runtime
	runningMu   sync.Mutex
	runningRuns map[string]*runningAgentRun
}

type executionContext struct {
	Run             servermodels.AgentRun `bun:",embed"`
	AgentName       string                `bun:"agent_name"`
	Brand           string                `bun:"brand"`
	APIKey          string                `bun:"api_key"`
	APIURL          string                `bun:"api_url"`
	ModelIdentifier string                `bun:"model_identifier"`
	MaxOutputTokens int64                 `bun:"max_output_tokens"`
	Instruction     string                `bun:"instruction"`
}

// NewExecuteAction 创建 Agent Worker Action。
func NewExecuteAction(db *bun.DB, enqueuer servertask.TxEnqueuer, runtime agentruntime.Runtime) *ExecuteAction {
	return &ExecuteAction{db: db, enqueuer: enqueuer, runtime: runtime, runningRuns: make(map[string]*runningAgentRun)}
}

// Execute 运行 TurnLoop，并只保存吸收完当前输入后的稳定回复。
func (a *ExecuteAction) Execute(ctx context.Context, input RunInput) error {
	if !common.ValidUUID(input.RunID) {
		return task.Permanent(errors.New("agent run id is invalid"))
	}
	execution, terminal, err := a.begin(ctx, input.RunID)
	if err != nil {
		return err
	}
	if terminal {
		return nil
	}
	runCtx, cancel := context.WithTimeout(ctx, agentRunTimeout)
	defer cancel()
	unregister := a.registerRunContext(execution.Run.ID, cancel)
	defer unregister()
	maxOutputTokens := agentMaxOutputTokens
	if execution.MaxOutputTokens > 0 && execution.MaxOutputTokens < int64(maxOutputTokens) {
		maxOutputTokens = int(execution.MaxOutputTokens)
	}
	var feed agentruntime.InputFeed
	switch domain.AgentTriggerType(execution.Run.TriggerType) {
	case domain.AgentTriggerTypeDirect:
		feed = &databaseInputFeed{db: a.db, execution: execution}
	case domain.AgentTriggerTypeCustomerAuto:
		feed = &customerInputFeed{db: a.db, execution: execution}
	default:
		return task.Permanent(fmt.Errorf("unsupported agent trigger type %q", execution.Run.TriggerType))
	}
	result, err := a.runtime.Run(runCtx, agentruntime.RunRequest{
		RunID: execution.Run.ID, Name: execution.AgentName, Instruction: execution.Instruction,
		Model: agentruntime.ModelConfig{
			Brand: execution.Brand, APIKey: execution.APIKey, BaseURL: execution.APIURL,
			Identifier: execution.ModelIdentifier, MaxOutputTokens: maxOutputTokens,
		},
	}, feed)
	if errors.Is(err, errCustomerRunSuppressed) {
		return nil
	}
	if err == nil {
		if completeErr := a.complete(ctx, execution, result); completeErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("persist completed agent run: %w", completeErr)
		}
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	terminal, failErr := a.fail(ctx, execution.Run.ID, err)
	if failErr != nil {
		return fmt.Errorf("agent run failed: %v; persist failure: %w", err, failErr)
	}
	if terminal {
		return nil
	}
	return task.Permanent(fmt.Errorf("execute agent run: %w", err))
}

// begin 将待执行或崩溃恢复中的业务运行标记为运行中并读取配置。
func (a *ExecuteAction) begin(ctx context.Context, runID string) (executionContext, bool, error) {
	var status string
	if err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Column("status").Where("agr.id = ?", runID).Scan(ctx, &status); errors.Is(err, sql.ErrNoRows) {
		return executionContext{}, false, task.Permanent(errors.New("agent run not found"))
	} else if err != nil {
		return executionContext{}, false, fmt.Errorf("load agent run status: %w", err)
	}
	if agentRunStatusTerminal(status) {
		return executionContext{}, true, nil
	}
	if status != string(domain.AgentRunStatusQueued) && status != string(domain.AgentRunStatusRunning) {
		return executionContext{}, false, task.Permanent(fmt.Errorf("unsupported agent run status %q", status))
	}
	result, err := a.db.NewUpdate().Model((*servermodels.AgentRun)(nil)).
		Set("status = ?", domain.AgentRunStatusRunning).
		Set("started_at = COALESCE(started_at, now())").
		Set("updated_at = now()").
		Where("id = ?", runID).
		Where("status IN (?, ?)", domain.AgentRunStatusQueued, domain.AgentRunStatusRunning).
		Exec(ctx)
	if err != nil {
		return executionContext{}, false, fmt.Errorf("begin agent run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return executionContext{}, false, fmt.Errorf("read begun agent run rows: %w", err)
	}
	if rows == 0 {
		if err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
			Column("status").Where("agr.id = ?", runID).Scan(ctx, &status); err != nil {
			return executionContext{}, false, fmt.Errorf("reload agent run status: %w", err)
		}
		if agentRunStatusTerminal(status) {
			return executionContext{}, true, nil
		}
		return executionContext{}, false, fmt.Errorf("agent run status changed to %q before begin", status)
	}
	execution := executionContext{}
	err = a.db.NewSelect().
		TableExpr("agent_runs AS agr").
		ColumnExpr("agr.*").
		ColumnExpr("oi.display_name AS agent_name").
		ColumnExpr("aip.brand AS brand, aip.api_key AS api_key, aip.api_url AS api_url").
		ColumnExpr("ar.configuration->'model'->>'identifier' AS model_identifier").
		ColumnExpr("aipm.max_output_tokens AS max_output_tokens").
		ColumnExpr("ar.configuration->>'systemInstruction' AS instruction").
		Join("JOIN agents AS a ON a.identity_id = agr.agent_identity_id AND a.organization_id = agr.organization_id").
		Join("JOIN organization_identities AS oi ON oi.id = a.identity_id AND oi.organization_id = a.organization_id").
		Join("JOIN agent_revisions AS ar ON ar.id = agr.agent_revision_id AND ar.agent_id = a.id AND ar.organization_id = agr.organization_id").
		Join("JOIN ai_providers AS aip ON aip.id = (ar.configuration->'model'->>'providerId')::uuid AND aip.organization_id = agr.organization_id").
		Join("JOIN ai_provider_models AS aipm ON aipm.provider_id = aip.id AND aipm.organization_id = aip.organization_id AND aipm.identifier = ar.configuration->'model'->>'identifier'").
		Where("agr.id = ?", runID).
		Where("agr.status = ?", domain.AgentRunStatusRunning).
		Where("ar.execution_mode = ?", domain.AgentExecutionModeManaged).
		Where("ar.schema_version = 1").
		Where("aipm.model_type = ?", domain.AIModelTypeChat).
		Scan(ctx, &execution)
	if errors.Is(err, sql.ErrNoRows) {
		if reloadErr := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
			Column("status").Where("agr.id = ?", runID).Scan(ctx, &status); reloadErr != nil {
			return executionContext{}, false, fmt.Errorf("reload unavailable agent run status: %w", reloadErr)
		}
		if agentRunStatusTerminal(status) {
			return executionContext{}, true, nil
		}
	}
	if err != nil {
		return executionContext{}, false, fmt.Errorf("load agent run execution: %w", err)
	}
	return execution, false, nil
}

// agentRunStatusTerminal 判断 Agent Run 是否已经进入不可覆盖的终态。
func agentRunStatusTerminal(status string) bool {
	return status == string(domain.AgentRunStatusSucceeded) ||
		status == string(domain.AgentRunStatusFailed) ||
		status == string(domain.AgentRunStatusCancelled)
}

// complete 原子写入最终回复、推进消费序号并补投递竞态中的后续输入。
func (a *ExecuteAction) complete(ctx context.Context, execution executionContext, result agentruntime.RunResult) error {
	if domain.AgentTriggerType(execution.Run.TriggerType) == domain.AgentTriggerTypeCustomerAuto {
		return a.completeCustomer(ctx, execution, result)
	}
	if domain.AgentTriggerType(execution.Run.TriggerType) != domain.AgentTriggerTypeDirect {
		return fmt.Errorf("unsupported agent trigger type %q", execution.Run.TriggerType)
	}
	var status string
	if err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Column("status").Where("agr.id = ?", execution.Run.ID).Scan(ctx, &status); err != nil {
		return err
	}
	if agentRunStatusTerminal(status) {
		return nil
	}
	content := strings.TrimSpace(result.Content)
	if content == "" || result.EndSeq <= 0 {
		return errors.New("agent runtime returned an invalid result")
	}
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return fmt.Errorf("encode agent run usage: %w", err)
	}
	messageID := uuid.NewV7()
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.conversation_id = ?", execution.Run.ConversationID).
			Where("cas.organization_id = ?", execution.Run.OrganizationID).
			Where("cas.agent_identity_id = ?", execution.Run.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock agent conversation state for completion: %w", err)
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", execution.Run.ID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock agent run for completion: %w", err)
		}
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		if run.Status != string(domain.AgentRunStatusRunning) || run.TriggerEndSeq == nil || *run.TriggerEndSeq != result.EndSeq {
			return errors.New("agent run completion boundary is inconsistent")
		}
		var participantID string
		if err := tx.NewSelect().TableExpr("conversation_participants AS cp").
			ColumnExpr("cp.id").
			Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
			Where("cp.organization_id = ?", run.OrganizationID).
			Where("cp.conversation_id = ?", run.ConversationID).
			Where("cp.left_at IS NULL").
			Where("cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
			Where("cs.source_id = ?", execution.Run.AgentIdentityID).
			Scan(ctx, &participantID); err != nil {
			return fmt.Errorf("load agent conversation participant: %w", err)
		}
		idempotencyKey := "agent:" + run.ID
		originatedAt := time.Now().UTC()
		message := &servermodels.Message{
			ID: messageID.String(), OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
			SenderParticipantID: &participantID, Type: string(domain.MessageTypeText), Body: content,
			IdempotencyKey: &idempotencyKey, OriginatedAt: originatedAt,
		}
		if _, err := tx.NewInsert().Model(message).
			Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
			Returning("created_at").Exec(ctx); err != nil {
			return fmt.Errorf("create agent response message: %w", err)
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Conversation)(nil)).
			Set("last_message_id = ?", message.ID).
			Set("last_message_at = ?", message.OriginatedAt).
			Set("updated_at = now()").
			Where("id = ?", run.ConversationID).
			Where("organization_id = ?", run.OrganizationID).
			Where("last_message_at IS NULL OR (last_message_at, last_message_id) < (?, ?)", message.OriginatedAt, message.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("update conversation after agent response: %w", err)
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusSucceeded).
			Set("response_message_id = ?", message.ID).
			Set("usage = ?::jsonb", string(usage)).
			Set("last_error = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("complete agent run: %w", err)
		}
		if _, err := tx.NewUpdate().Model(state).
			Set("processed_seq = ?", result.EndSeq).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("advance processed agent input sequence: %w", err)
		}
		if state.DesiredSeq <= result.EndSeq {
			return nil
		}
		var revisionID string
		if err := tx.NewSelect().Model((*servermodels.Agent)(nil)).
			Column("active_revision_id").
			Where("a.identity_id = ?", run.AgentIdentityID).
			Where("a.organization_id = ?", run.OrganizationID).
			Scan(ctx, &revisionID); err != nil {
			return fmt.Errorf("load next agent run revision: %w", err)
		}
		_, err := insertAndEnqueueRun(ctx, tx, a.enqueuer, run.OrganizationID, run.ConversationID, run.AgentIdentityID, revisionID, result.EndSeq+1)
		return err
	})
}

// fail 标记最终失败，并为执行期间刚到达的新输入补建下一次运行。
func (a *ExecuteAction) fail(ctx context.Context, runID string, runErr error) (bool, error) {
	message := truncateRunError(runErr)
	initial := &servermodels.AgentRun{}
	if err := a.db.NewSelect().Model(initial).Where("agr.id = ?", runID).Scan(ctx); err != nil {
		return false, err
	}
	if agentRunStatusTerminal(initial.Status) {
		return true, nil
	}
	if domain.AgentTriggerType(initial.TriggerType) == domain.AgentTriggerTypeCustomerAuto {
		return a.failCustomer(ctx, initial, message)
	}
	if domain.AgentTriggerType(initial.TriggerType) != domain.AgentTriggerTypeDirect {
		return false, fmt.Errorf("unsupported agent trigger type %q", initial.TriggerType)
	}
	terminal := false
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		state := &servermodels.ConversationAgentState{}
		if err := tx.NewSelect().Model(state).
			Where("cas.conversation_id = ?", initial.ConversationID).
			Where("cas.organization_id = ?", initial.OrganizationID).
			Where("cas.agent_identity_id = ?", initial.AgentIdentityID).
			For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", runID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if agentRunStatusTerminal(run.Status) {
			terminal = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusQueued) && run.Status != string(domain.AgentRunStatusRunning) {
			return fmt.Errorf("cannot fail agent run in status %q", run.Status)
		}
		failureEnd := run.TriggerStartSeq
		if run.TriggerEndSeq != nil {
			failureEnd = *run.TriggerEndSeq
		}
		if run.TriggerStartSeq != state.ProcessedSeq+1 || failureEnd < run.TriggerStartSeq || failureEnd > state.DesiredSeq {
			return errors.New("agent run failure boundary is inconsistent")
		}
		var failedSeqs []int64
		if err := tx.NewRaw(`
			UPDATE conversation_agent_triggers
			SET agent_run_id = ?
			WHERE organization_id = ?
				AND conversation_id = ?
				AND agent_identity_id = ?
				AND trigger_seq > ?
				AND trigger_seq <= ?
			RETURNING trigger_seq
		`, run.ID, run.OrganizationID, run.ConversationID, run.AgentIdentityID, state.ProcessedSeq, failureEnd).Scan(ctx, &failedSeqs); err != nil {
			return err
		}
		if int64(len(failedSeqs)) != failureEnd-state.ProcessedSeq {
			return errors.New("failed agent trigger sequence is not contiguous")
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusFailed).
			Set("trigger_end_seq = ?", failureEnd).
			Set("last_error = ?", message).
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(state).
			Set("processed_seq = ?", failureEnd).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return err
		}
		if state.DesiredSeq <= failureEnd {
			return nil
		}
		var revisionID string
		if err := tx.NewSelect().Model((*servermodels.Agent)(nil)).
			Column("active_revision_id").Where("a.identity_id = ?", run.AgentIdentityID).Where("a.organization_id = ?", run.OrganizationID).
			Scan(ctx, &revisionID); err != nil {
			return err
		}
		_, err := insertAndEnqueueRun(ctx, tx, a.enqueuer, run.OrganizationID, run.ConversationID, run.AgentIdentityID, revisionID, failureEnd+1)
		return err
	})
	return terminal, err
}

// FinalizeFailure 在任务达到最终失败时收敛 Agent 业务运行。
func (a *ExecuteAction) FinalizeFailure(ctx context.Context, input RunInput, runErr error) error {
	if !common.ValidUUID(input.RunID) {
		return errors.New("agent run id is invalid")
	}
	_, err := a.fail(ctx, input.RunID, runErr)
	return err
}

// truncateRunError 限制持久化错误详情长度。
func truncateRunError(err error) string {
	if err == nil {
		return "agent run failed"
	}
	runes := []rune(err.Error())
	if len(runes) > agentRunErrorMaxRunes {
		runes = runes[:agentRunErrorMaxRunes]
	}
	return string(runes)
}
