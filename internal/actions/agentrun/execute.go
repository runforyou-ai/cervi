//go:build server

package agentrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
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

type knowledgeSearchFactory interface {
	ForKnowledgeBases(context.Context, string, []string) (func(context.Context, knowledgeretrieval.Request) (knowledgeretrieval.Result, error), error)
}

// ExecuteAction 执行并收尾一次 Agent 业务运行。
type ExecuteAction struct {
	db                     *bun.DB
	enqueuer               servertask.TxEnqueuer
	runtime                agentruntime.Runtime
	knowledgeSearchFactory knowledgeSearchFactory
	runningMu              sync.Mutex
	runningRuns            map[string]*runningAgentRun
}

type executionContext struct {
	Run              servermodels.AgentRun `bun:",embed"`
	AgentName        string                `bun:"agent_name"`
	Brand            string                `bun:"brand"`
	APIKey           string                `bun:"api_key"`
	APIURL           string                `bun:"api_url"`
	ModelIdentifier  string                `bun:"model_identifier"`
	MaxOutputTokens  int64                 `bun:"max_output_tokens"`
	Instruction      string                `bun:"instruction"`
	KnowledgeBaseIDs []string              `bun:"knowledge_base_ids,type:jsonb"`
}

// NewExecuteAction 创建 Agent Worker Action。
func NewExecuteAction(db *bun.DB, enqueuer servertask.TxEnqueuer, runtime agentruntime.Runtime, knowledgeSearchFactory knowledgeSearchFactory) *ExecuteAction {
	return &ExecuteAction{db: db, enqueuer: enqueuer, runtime: runtime, knowledgeSearchFactory: knowledgeSearchFactory, runningRuns: make(map[string]*runningAgentRun)}
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
	running, unregister, err := a.registerRunContext(ctx, execution.Run.ID, cancel)
	if err != nil {
		return err
	}
	defer unregister()
	if running.attempt > 1 {
		taskExecution, _ := servertask.CurrentExecution(ctx)
		slog.Warn("Agent 任务重新计算", "agent_run_id", execution.Run.ID, "task_run_id", taskExecution.TaskRunID,
			"attempt", running.attempt, "stream_id", running.progress.StreamID)
	}
	maxOutputTokens := agentMaxOutputTokens
	if execution.MaxOutputTokens > 0 && execution.MaxOutputTokens < int64(maxOutputTokens) {
		maxOutputTokens = int(execution.MaxOutputTokens)
	}
	policy, _, err := a.policyForRun(&execution.Run)
	if err != nil {
		return task.Permanent(err)
	}
	feed := &databaseInputFeed{db: a.db, execution: execution, policy: policy}
	var customerHistorySearch agentruntime.CustomerHistorySearch
	if domain.AgentTriggerType(execution.Run.TriggerType) == domain.AgentTriggerTypeCustomerAuto {
		// TODO：全文检索方案确定后接入历史查询，范围限定为本企业、本 Conversation 内已关闭的 ServiceSession。
		// 占位结果明确表示功能不可用，不能让模型据此断言没有历史记录。
		customerHistorySearch = func(context.Context, string) (agentruntime.CustomerHistoryResult, error) {
			return agentruntime.CustomerHistoryResult{
				Available: false,
				Message:   "历史消息查询暂未开放，无法确认以往的沟通内容。请根据本轮消息回答，必要时请客户补充信息；不要重复调用此工具。",
			}, nil
		}
	}
	// 空绑定不注册知识检索；非空绑定只从本次 Run 的版本加载。
	var knowledgeSearch agentruntime.KnowledgeSearch
	if len(execution.KnowledgeBaseIDs) > 0 {
		knowledgeSearch, err = a.knowledgeSearchFactory.ForKnowledgeBases(runCtx, execution.Run.OrganizationID, execution.KnowledgeBaseIDs)
		if err != nil {
			slog.Warn("Agent 知识库范围加载失败", "agent_run_id", execution.Run.ID,
				"organization_id", execution.Run.OrganizationID, "knowledge_base_count", len(execution.KnowledgeBaseIDs), "error", err)
		}
	}
	// 范围加载失败也走运行失败收尾。
	var result agentruntime.RunResult
	if err == nil {
		result, err = a.runtime.Run(runCtx, agentruntime.RunRequest{
			RunID: execution.Run.ID, Name: execution.AgentName, Instruction: execution.Instruction,
			Model: agentruntime.ModelConfig{
				Brand: execution.Brand, APIKey: execution.APIKey, BaseURL: execution.APIURL,
				Identifier: execution.ModelIdentifier, MaxOutputTokens: maxOutputTokens,
			},
			KnowledgeSearch:       knowledgeSearch,
			CustomerHistorySearch: customerHistorySearch,
			StreamID:              running.progress.StreamID,
			Attempt:               running.attempt,
			OnProgress: func(progress agentruntime.Progress) {
				a.runningMu.Lock()
				defer a.runningMu.Unlock()
				if a.runningRuns[execution.Run.ID] == running && runCtx.Err() == nil {
					running.progress = progress.Clone()
				}
			},
		}, feed)
	}
	if errors.Is(err, errAgentRunSuppressed) {
		return nil
	}
	if err == nil {
		if completeErr := a.complete(ctx, execution, policy, result); completeErr != nil {
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
	initial := &servermodels.AgentRun{}
	if err := a.db.NewSelect().Model(initial).Where("agr.id = ?", runID).Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return executionContext{}, false, task.Permanent(errors.New("agent run not found"))
	} else if err != nil {
		return executionContext{}, false, fmt.Errorf("load agent run: %w", err)
	}
	if agentRunStatusTerminal(initial.Status) {
		return executionContext{}, true, nil
	}
	policy, _, err := a.policyForRun(initial)
	if err != nil {
		return executionContext{}, false, task.Permanent(err)
	}
	terminal := false
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return err
		}
		run := locked.Run
		if agentRunStatusTerminal(run.Status) {
			terminal = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusQueued) && run.Status != string(domain.AgentRunStatusRunning) {
			return task.Permanent(fmt.Errorf("unsupported agent run status %q", run.Status))
		}
		_, err = tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusRunning).
			Set("started_at = COALESCE(started_at, now())").
			Set("updated_at = now()").WherePK().Exec(ctx)
		return err
	})
	if err != nil {
		return executionContext{}, false, fmt.Errorf("begin agent run: %w", err)
	}
	if terminal {
		return executionContext{}, true, nil
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
		ColumnExpr("ar.configuration->'knowledgeBaseIds' AS knowledge_base_ids").
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
		var status string
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

// policyForRun 根据持久化触发类型选择运行策略和输入范围。
func (a *ExecuteAction) policyForRun(run *servermodels.AgentRun) (agentRunPolicy, agentRunScope, error) {
	scope, err := agentRunScopeFor(run)
	if err != nil {
		return nil, agentRunScope{}, err
	}
	switch domain.AgentTriggerType(run.TriggerType) {
	case domain.AgentTriggerTypeDirect:
		return agentChatRunPolicy{enqueuer: a.enqueuer}, scope, nil
	case domain.AgentTriggerTypeCustomerAuto:
		return customerRunPolicy{enqueuer: a.enqueuer}, scope, nil
	default:
		return nil, agentRunScope{}, fmt.Errorf("unsupported agent trigger type %q", run.TriggerType)
	}
}

// insertAgentMessage 写入一条带运行幂等键的 Agent 结果消息。
func insertAgentMessage(ctx context.Context, db bun.IDB, run *servermodels.AgentRun, messageID, participantID string, messageType domain.MessageType, content string, serviceSessionID *string) (*servermodels.Message, error) {
	idempotencyKey := "agent:" + run.ID
	message := &servermodels.Message{
		ID: messageID, OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
		ServiceSessionID: serviceSessionID, SenderParticipantID: &participantID,
		Type: string(messageType), Body: content, IdempotencyKey: &idempotencyKey,
		OriginatedAt: time.Now().UTC(),
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at").
		Returning("*").Exec(ctx); err != nil {
		return nil, fmt.Errorf("create agent result message: %w", err)
	}
	return message, nil
}

// updateConversationAfterAgentResponse 按消息稳定顺序更新会话摘要。
func updateConversationAfterAgentResponse(ctx context.Context, db bun.IDB, message *servermodels.Message) error {
	if _, err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		Where("id = ?", message.ConversationID).
		Where("organization_id = ?", message.OrganizationID).
		Where("last_message_at IS NULL OR (last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("update conversation after agent response: %w", err)
	}
	return nil
}

// complete 按运行策略抑制失效结果或原子写入回复并推进消费序号。
func (a *ExecuteAction) complete(ctx context.Context, execution executionContext, policy agentRunPolicy, result agentruntime.RunResult) error {
	content := strings.TrimSpace(result.Content)
	if content == "" || result.EndSeq <= 0 {
		return errors.New("agent runtime returned an invalid result")
	}
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return fmt.Errorf("encode agent run usage: %w", err)
	}
	messageID := uuid.NewV7().String()
	// 成功内容与最终消息共享事务，失败或取消不写入内容块。
	blocks := make([]servermodels.AgentRunBlock, 0, len(result.Blocks))
	for _, block := range result.Blocks {
		payload, err := json.Marshal(block.Payload)
		if err != nil {
			return fmt.Errorf("encode agent run block: %w", err)
		}
		blocks = append(blocks, servermodels.AgentRunBlock{
			ID: block.ID, OrganizationID: execution.Run.OrganizationID, AgentRunID: execution.Run.ID,
			Position: block.Position, ModelCallID: block.ModelCallID, Kind: string(block.Kind), Payload: payload,
		})
	}
	suppressed := false
	completed := false
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, &execution.Run)
		if err != nil {
			return fmt.Errorf("lock agent run for completion: %w", err)
		}
		policyContext, state, run := locked.PolicyContext, locked.State, locked.Run
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		allowed, err := policy.prepareLocked(ctx, tx, policyContext, run)
		if err != nil {
			return err
		}
		if !allowed {
			suppressed = true
			return nil
		}
		if run.Status != string(domain.AgentRunStatusRunning) || run.TriggerEndSeq == nil ||
			*run.TriggerEndSeq != result.EndSeq || run.TriggerStartSeq != state.ProcessedSeq+1 {
			return errors.New("agent run completion boundary is inconsistent")
		}
		if err := policy.persistMessage(ctx, tx, policyContext, run, messageID, domain.MessageTypeText, content); err != nil {
			return err
		}
		if len(blocks) > 0 {
			if _, err := tx.NewInsert().Model(&blocks).Exec(ctx); err != nil {
				return fmt.Errorf("persist agent run blocks: %w", err)
			}
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusSucceeded).
			Set("response_message_id = ?", messageID).
			Set("usage = ?::jsonb", string(usage)).
			Set("last_error = NULL").
			Set("error_code = NULL").
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
		if state.DesiredSeq > result.EndSeq {
			if err := policy.enqueueNext(ctx, tx, policyContext, run, result.EndSeq+1); err != nil {
				return err
			}
		}
		completed = true
		return nil
	})
	if err != nil {
		return err
	}
	if suppressed && execution.Run.ServiceSessionID != nil {
		// 记录客服门禁抑制的迟到结果。
		slog.Warn("网站客户 Agent 迟到结果已抑制",
			"agent_run_id", execution.Run.ID,
			"conversation_id", execution.Run.ConversationID,
		)
	}
	if completed {
		logCompletedRun(execution, result.EndSeq, messageID)
	}
	return nil
}

// logCompletedRun 记录关联客服周期的 Agent 完成结果。
func logCompletedRun(execution executionContext, endSeq int64, messageID string) {
	if execution.Run.ServiceSessionID == nil {
		return
	}
	slog.Info("网站客户 Agent 运行完成",
		"agent_run_id", execution.Run.ID,
		"conversation_id", execution.Run.ConversationID,
		"service_session_id", *execution.Run.ServiceSessionID,
		"trigger_start_seq", execution.Run.TriggerStartSeq,
		"trigger_end_seq", endSeq,
		"response_message_id", messageID,
	)
}

// fail 按运行策略取消失效运行或标记失败，并为剩余输入补建下一次运行。
func (a *ExecuteAction) fail(ctx context.Context, runID string, runErr error) (bool, error) {
	// 限制持久化错误详情长度。
	message := "agent run failed"
	if runErr != nil {
		runes := []rune(runErr.Error())
		if len(runes) > agentRunErrorMaxRunes {
			runes = runes[:agentRunErrorMaxRunes]
		}
		message = string(runes)
	}
	initial := &servermodels.AgentRun{}
	if err := a.db.NewSelect().Model(initial).Where("agr.id = ?", runID).Scan(ctx); err != nil {
		return false, err
	}
	if agentRunStatusTerminal(initial.Status) {
		return true, nil
	}
	policy, scope, err := a.policyForRun(initial)
	if err != nil {
		return false, err
	}
	terminal := false
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return fmt.Errorf("lock agent run for failure: %w", err)
		}
		policyContext, state, run := locked.PolicyContext, locked.State, locked.Run
		if agentRunStatusTerminal(run.Status) {
			terminal = true
			return nil
		}
		allowed, err := policy.prepareLocked(ctx, tx, policyContext, run)
		if err != nil {
			return err
		}
		if !allowed {
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
		failedSeqs, err := assignAgentTriggers(ctx, tx, run, scope, state.ProcessedSeq, failureEnd)
		if err != nil {
			return err
		}
		if int64(len(failedSeqs)) != failureEnd-state.ProcessedSeq {
			return errors.New("failed agent trigger sequence is not contiguous")
		}
		messageID := uuid.NewV7().String()
		if err := policy.persistMessage(ctx, tx, policyContext, run, messageID, domain.MessageTypeAgentError, ""); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusFailed).
			Set("response_message_id = ?", messageID).
			Set("trigger_end_seq = ?", failureEnd).
			Set("last_error = ?", message).
			Set("error_code = NULL").
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
		if state.DesiredSeq > failureEnd {
			if err := policy.enqueueNext(ctx, tx, policyContext, run, failureEnd+1); err != nil {
				return err
			}
		}
		return nil
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
