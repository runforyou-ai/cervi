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

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/task"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const (
	agentRunTimeout       = 5 * time.Minute
	agentHistoryLimit     = 100
	agentRunErrorMaxRunes = 4000
)

// ExecuteAction 执行并收尾一次 Agent 业务运行。
type ExecuteAction struct {
	db          *bun.DB
	enqueuer    servertask.TxEnqueuer
	runtime     agentruntime.Runtime
	attachments *AttachmentReader
	knowledge   KnowledgeRetrieval
	runningMu   sync.Mutex
	runningRuns map[string]*runningAgentRun
}

type executionContext struct {
	Run              servermodels.AgentRun         `bun:",embed"`
	AgentName        string                        `bun:"agent_name"`
	Brand            string                        `bun:"brand"`
	APIKey           string                        `bun:"api_key"`
	APIURL           string                        `bun:"api_url"`
	ModelIdentifier  string                        `bun:"model_identifier"`
	MaxOutputTokens  int64                         `bun:"max_output_tokens"`
	ContextWindow    int64                         `bun:"context_window"`
	InputModalities  []domain.AIModelInputModality `bun:"input_modalities,type:jsonb"`
	Instruction      string                        `bun:"instruction"`
	KnowledgeBaseIDs []string                      `bun:"knowledge_base_ids,type:jsonb"`
	ProviderID       string                        `bun:"provider_id"`
	RoleKind         string                        `bun:"role_kind"`
	OrganizationName string                        `bun:"organization_name"`
}

// NewExecuteAction 创建 Agent Worker Action。
func NewExecuteAction(db *bun.DB, enqueuer servertask.TxEnqueuer, runtime agentruntime.Runtime, attachments *AttachmentReader, knowledge KnowledgeRetrieval) *ExecuteAction {
	return &ExecuteAction{db: db, enqueuer: enqueuer, runtime: runtime, attachments: attachments, knowledge: knowledge, runningRuns: make(map[string]*runningAgentRun)}
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
			"attempt", running.attempt, "stream_id", running.streamID)
	}
	policy, err := a.policyForRun(ctx, &execution.Run)
	if err != nil {
		return task.Permanent(err)
	}
	feed := &databaseInputFeed{db: a.db, enqueuer: a.enqueuer, execution: execution, policy: policy, attachments: a.attachments}
	var customerHistorySearch agentruntime.CustomerHistorySearch
	if domain.AgentExecutionScopeKind(execution.Run.ScopeKind) == domain.AgentExecutionScopeServiceSession {
		// TODO：接入本企业、本 Conversation 内已关闭 ServiceSession 的全文历史查询。
		// 向模型返回历史查询功能不可用的占位结果。
		customerHistorySearch = func(context.Context, string) (agentruntime.CustomerHistoryResult, error) {
			return agentruntime.CustomerHistoryResult{
				Available: false,
				Message:   "历史消息查询暂未开放，无法确认以往的沟通内容。请根据本轮消息回答，必要时请客户补充信息；不要重复调用此工具。",
			}, nil
		}
	}
	mcpServers, err := loadRunMCPServers(ctx, a.db, &execution.Run)
	if err != nil {
		return fmt.Errorf("load agent run mcp servers: %w", err)
	}
	knowledgeSearch, err := loadRunKnowledgeSearch(ctx, a.db, a.knowledge, execution)
	if err != nil {
		return fmt.Errorf("load agent run knowledge bases: %w", err)
	}
	snapshot, err := a.resolveBehaviorSnapshot(ctx, execution, policy, behaviorTools{Knowledge: knowledgeSearch != nil}, mcpServers)
	if err != nil {
		return err
	}
	// 场景、依据策略、指令与模型参数以快照为准，凭据与输入模态取当前供应商配置。
	result, err := a.runtime.Run(runCtx, agentruntime.RunRequest{
		RunID: execution.Run.ID, Name: execution.AgentName, Scene: snapshot.Scene, Grounding: snapshot.Grounding, Instruction: snapshot.Instruction,
		Model: agentruntime.ModelConfig{
			Brand: execution.Brand, APIKey: execution.APIKey, BaseURL: execution.APIURL,
			Identifier: snapshot.Model.Identifier, MaxOutputTokens: int(snapshot.Model.MaxOutputTokens), ContextWindow: int(snapshot.Model.ContextWindow),
			InputModalities: execution.InputModalities,
		},
		KnowledgeSearch:       knowledgeSearch,
		CustomerHistorySearch: customerHistorySearch,
		ReadAttachment: func(ctx context.Context, messageID string) ([]byte, error) {
			return a.attachments.Content(ctx, &execution.Run, messageID)
		},
		MCPServers: mcpServers,
		StreamID:   running.streamID,
		Attempt:    running.attempt,
		OnStream: func(delta agentruntime.StreamDelta) {
			// 运行 context 已取消时丢弃增量。
			if runCtx.Err() == nil {
				running.stream.publish(delta)
			}
		},
	}, feed)
	if errors.Is(err, errAgentRunSuppressed) {
		// 运行吸收后续输入时已失去资格，保留此前已产生的过程内容。
		return a.persistPartialProcess(ctx, &execution.Run, result)
	}
	if err == nil {
		if completeErr := a.complete(ctx, execution, policy, result); completeErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("persist completed agent run: %w", completeErr)
		}
		// 迟到结果被门禁抑制时运行已被取消，成功收尾则已写入完整过程，此处只补前者。
		return a.persistPartialProcess(ctx, &execution.Run, result)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// 运行时限到期时统一以超时原因收尾。
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("%w: %w", err, context.DeadlineExceeded)
	}
	terminal, failErr := a.fail(ctx, execution.Run.ID, err)
	if failErr != nil {
		return fmt.Errorf("agent run failed: %v; persist failure: %w", err, failErr)
	}
	// 失败与被取消的运行同样保留已产生的过程内容。
	if processErr := a.persistPartialProcess(ctx, &execution.Run, result); processErr != nil {
		return fmt.Errorf("agent run failed: %v; persist partial process: %w", err, processErr)
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
	policy, err := a.policyForRun(ctx, initial)
	if err != nil {
		return executionContext{}, false, task.Permanent(err)
	}
	terminal := false
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
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
		queued := run.Status == string(domain.AgentRunStatusQueued)
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusRunning).
			Set("started_at = COALESCE(started_at, now())").
			Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return err
		}
		// 排队运行转为运行中时推进会话版本。
		if !queued {
			return nil
		}
		return chatstate.TouchConversation(ctx, tx, locked.PolicyContext.Conversation)
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
		ColumnExpr("aipm.input_modalities").
		ColumnExpr("ar.configuration->'knowledgeBaseIds' AS knowledge_base_ids").
		ColumnExpr("aip.id::text AS provider_id, r.kind AS role_kind, o.name AS organization_name").
		Join("JOIN agents AS a ON a.identity_id = agr.agent_identity_id AND a.organization_id = agr.organization_id").
		Join("JOIN organizations AS o ON o.id = agr.organization_id").
		Apply(func(query *bun.SelectQuery) *bun.SelectQuery {
			return withManagedAgentConfiguration(query, "agr.agent_revision_id")
		}).
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id").
		Where("agr.id = ?", runID).
		Where("agr.status = ?", domain.AgentRunStatusRunning).
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

// resolveBehaviorSnapshot 首次执行时取场景规则、拼接运行指令并固定快照；重复执行尝试直接沿用已写入的快照，不再查询场景规则。
// 客服入口当前只注册不可用的客户历史占位工具，工具说明与快照都不把它算作可用工具。
func (a *ExecuteAction) resolveBehaviorSnapshot(ctx context.Context, execution executionContext, policy agentRunPolicy, tools behaviorTools, mcpServers []agentruntime.MCPServer) (BehaviorSnapshot, error) {
	snapshot := BehaviorSnapshot{}
	if len(execution.Run.BehaviorSnapshot) > 0 {
		if err := json.Unmarshal(execution.Run.BehaviorSnapshot, &snapshot); err != nil {
			return BehaviorSnapshot{}, fmt.Errorf("decode agent run behavior snapshot: %w", err)
		}
		return snapshot, nil
	}
	scene, sceneRules, err := policy.sceneRules(ctx, a.db, execution, tools)
	if err != nil {
		return BehaviorSnapshot{}, fmt.Errorf("build agent run scene rules: %w", err)
	}
	// 按注册顺序收集内置工具，开发期计算器只在内部场景注册，终止工具只在客服场景注册；MCP 服务只记录绑定的服务名称。
	names := make([]string, 0, 3)
	if scene != agentruntime.SceneCustomer {
		names = append(names, "calculator")
	}
	if tools.Knowledge {
		names = append(names, "search_knowledge")
	}
	if scene == agentruntime.SceneCustomer {
		names = append(names, "ask_customer", "handoff_to_human")
	}
	serverNames := make([]string, 0, len(mcpServers))
	for _, server := range mcpServers {
		serverNames = append(serverNames, server.Name)
	}
	snapshot = newBehaviorSnapshot(execution, scene, sceneRules, names, serverNames)
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return BehaviorSnapshot{}, fmt.Errorf("encode agent run behavior snapshot: %w", err)
	}
	result, err := a.db.NewUpdate().Model((*servermodels.AgentRun)(nil)).
		Set("behavior_snapshot = ?::jsonb", string(encoded)).
		Set("updated_at = now()").
		Where("agr.id = ?", execution.Run.ID).
		Where("agr.behavior_snapshot IS NULL").
		Exec(ctx)
	if err != nil {
		return BehaviorSnapshot{}, fmt.Errorf("persist agent run behavior snapshot: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		return snapshot, nil
	}
	// 并发的执行尝试已先写入快照，沿用那一份。
	var persisted json.RawMessage
	if err := a.db.NewSelect().Model((*servermodels.AgentRun)(nil)).
		Column("behavior_snapshot").Where("agr.id = ?", execution.Run.ID).Scan(ctx, &persisted); err != nil {
		return BehaviorSnapshot{}, fmt.Errorf("reload agent run behavior snapshot: %w", err)
	}
	if err := json.Unmarshal(persisted, &snapshot); err != nil {
		return BehaviorSnapshot{}, fmt.Errorf("decode persisted agent run behavior snapshot: %w", err)
	}
	return snapshot, nil
}

// withManagedAgentConfiguration 为已关联 agents AS a 的查询补充指定配置版本的模型和系统指令列，只保留有效的托管对话模型配置。
func withManagedAgentConfiguration(query *bun.SelectQuery, revisionIDColumn string) *bun.SelectQuery {
	return joinManagedAgentConfiguration(query, revisionIDColumn).
		ColumnExpr("aip.brand AS brand, aip.api_key AS api_key, aip.api_url AS api_url").
		ColumnExpr("ar.configuration->'model'->>'identifier' AS model_identifier").
		ColumnExpr("aipm.max_output_tokens AS max_output_tokens, aipm.context_window AS context_window").
		ColumnExpr("ar.configuration->>'systemInstruction' AS instruction")
}

// joinManagedAgentConfiguration 为已关联 agents AS a 的查询关联身份、指定配置版本与对话模型，只保留有效的托管对话模型配置。
func joinManagedAgentConfiguration(query *bun.SelectQuery, revisionIDColumn string) *bun.SelectQuery {
	return query.
		Join("JOIN organization_identities AS oi ON oi.id = a.identity_id AND oi.organization_id = a.organization_id").
		Join("JOIN agent_revisions AS ar ON ar.id = "+revisionIDColumn+" AND ar.agent_id = a.id AND ar.organization_id = a.organization_id").
		Join("JOIN ai_providers AS aip ON aip.id = (ar.configuration->'model'->>'providerId')::uuid AND aip.organization_id = a.organization_id").
		Join("JOIN ai_provider_models AS aipm ON aipm.provider_id = aip.id AND aipm.organization_id = aip.organization_id AND aipm.identifier = ar.configuration->'model'->>'identifier'").
		Where("ar.execution_mode = ?", domain.AgentExecutionModeManaged).
		Where("ar.schema_version = 1").
		Where("aipm.model_type = ?", domain.AIModelTypeChat)
}

// agentRunStatusTerminal 判断 Agent Run 是否已经进入不可覆盖的终态。
func agentRunStatusTerminal(status string) bool {
	return status == string(domain.AgentRunStatusSucceeded) ||
		status == string(domain.AgentRunStatusFailed) ||
		status == string(domain.AgentRunStatusCancelled)
}

// policyForRun 根据执行范围类型和会话形态选择运行策略。
func (a *ExecuteAction) policyForRun(ctx context.Context, run *servermodels.AgentRun) (agentRunPolicy, error) {
	switch domain.AgentExecutionScopeKind(run.ScopeKind) {
	case domain.AgentExecutionScopeServiceSession:
		return customerRunPolicy{enqueuer: a.enqueuer}, nil
	case domain.AgentExecutionScopeConversation:
		var conversationType string
		if err := a.db.NewSelect().Model((*servermodels.Conversation)(nil)).
			Column("type").
			Where("cv.organization_id = ? AND cv.id = ?", run.OrganizationID, run.ConversationID).
			Scan(ctx, &conversationType); err != nil {
			return nil, fmt.Errorf("load agent run conversation type: %w", err)
		}
		switch domain.ConversationType(conversationType) {
		case domain.ConversationTypeGroup:
			return groupMentionRunPolicy{scheduler: NewScheduler(a.enqueuer)}, nil
		case domain.ConversationTypeCopilot:
			return copilotRunPolicy{}, nil
		default:
			return agentChatRunPolicy{}, nil
		}
	default:
		return nil, fmt.Errorf("unsupported agent execution scope %q", run.ScopeKind)
	}
}

// agentResultMessage 构造 Run 的主结果消息，幂等键为 agent:<run_id>。
func agentResultMessage(run *servermodels.AgentRun, messageID, participantID string, messageType domain.MessageType, content string, serviceSessionID *string) *servermodels.Message {
	idempotencyKey := "agent:" + run.ID
	return &servermodels.Message{
		ID: messageID, OrganizationID: run.OrganizationID, ConversationID: run.ConversationID,
		ServiceSessionID: serviceSessionID, SenderParticipantID: &participantID,
		Type: string(messageType), Body: content, IdempotencyKey: &idempotencyKey,
	}
}

// appendAgentMessage 在 Run 终态门禁通过后追加结果消息，与运行终态共用事务；幂等重放时核对已有消息的类型与正文，不把另一类消息当作本次写入。
func appendAgentMessage(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, message *servermodels.Message) (*servermodels.Message, bool, error) {
	message.OriginatedAt = time.Now().UTC()
	appended, inserted, err := chatstate.AppendMessage(ctx, db, conversation, message)
	if err != nil {
		return nil, false, err
	}
	if !inserted && (appended.Type != message.Type || appended.Body != message.Body) {
		return nil, false, fmt.Errorf("agent message idempotency key %q holds a different message", *message.IdempotencyKey)
	}
	return appended, inserted, nil
}

// complete 按运行策略抑制失效结果或原子写入回复并推进消费序号。
func (a *ExecuteAction) complete(ctx context.Context, execution executionContext, policy agentRunPolicy, result agentruntime.RunResult) error {
	content := strings.TrimSpace(result.Content)
	handoff := result.Decision.Kind == domain.AgentRunOutcomeHandoff
	if handoff && domain.AgentExecutionScopeKind(execution.Run.ScopeKind) != domain.AgentExecutionScopeServiceSession {
		return errors.New("agent runtime returned a handoff outside customer service")
	}
	if (content == "" && !handoff) || result.EndSeq <= 0 {
		return errors.New("agent runtime returned an invalid result")
	}
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return fmt.Errorf("encode agent run usage: %w", err)
	}
	messageID := uuid.NewV7().String()
	// 在最终消息事务中写入成功运行的内容块。
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
	if handoff {
		return a.completeCustomerHandoff(ctx, execution, policy, result, usage, blocks)
	}
	suppressed := false
	completed := false
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, &execution.Run)
		if err != nil {
			return fmt.Errorf("lock agent run for completion: %w", err)
		}
		policyContext, lane, run := locked.PolicyContext, locked.Lane, locked.Run
		if agentRunStatusTerminal(run.Status) {
			return nil
		}
		allowed, err := policy.prepareLocked(ctx, tx, policyContext, run)
		if err != nil {
			return err
		}
		if !allowed {
			suppressed = true
			if err := chatstate.TouchConversation(ctx, tx, policyContext.Conversation); err != nil {
				return err
			}
			return scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID)
		}
		if run.Status != string(domain.AgentRunStatusRunning) || run.InputEndSeq == nil ||
			*run.InputEndSeq != result.EndSeq || run.InputStartSeq != lane.ProcessedSeq+1 {
			return errors.New("agent run completion boundary is inconsistent")
		}
		if err := policy.persistMessage(ctx, tx, policyContext, run, messageID, domain.MessageTypeText, content); err != nil {
			return err
		}
		if mentioning, ok := policy.(mentionReplyPolicy); ok {
			if err := mentioning.applyMentions(ctx, tx, policyContext, run, messageID, content); err != nil {
				return err
			}
		}
		if len(blocks) > 0 {
			if _, err := tx.NewInsert().Model(&blocks).Exec(ctx); err != nil {
				return fmt.Errorf("persist agent run blocks: %w", err)
			}
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusSucceeded).
			Set("outcome = ?", result.Decision.Outcome()).
			Set("response_message_id = ?", messageID).
			Set("usage = ?::jsonb", string(usage)).
			Set("last_error = NULL").
			Set("error_code = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("complete agent run: %w", err)
		}
		if _, err := tx.NewUpdate().Model(lane).
			Set("processed_seq = ?", result.EndSeq).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("advance processed agent input sequence: %w", err)
		}
		if err := scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID); err != nil {
			return err
		}
		completed = true
		return nil
	})
	if err != nil {
		return err
	}
	if suppressed && domain.AgentExecutionScopeKind(execution.Run.ScopeKind) == domain.AgentExecutionScopeServiceSession {
		// 记录客服门禁抑制的迟到结果。
		slog.Warn("客户 Agent 迟到结果已抑制",
			"agent_run_id", execution.Run.ID,
			"conversation_id", execution.Run.ConversationID,
		)
	}
	if completed {
		logCompletedRun(execution, result.EndSeq, messageID)
	}
	return nil
}

// persistPartialProcess 在运行进入终态后保留已产生的过程内容与用量，并推进会话版本让成员重读。运行仍可继续时不写入，成功收尾的完整过程因此不会撞上半成品。
func (a *ExecuteAction) persistPartialProcess(ctx context.Context, initial *servermodels.AgentRun, partial agentruntime.RunResult) error {
	if len(partial.Blocks) == 0 {
		return nil
	}
	usage, err := json.Marshal(partial.Usage)
	if err != nil {
		return fmt.Errorf("encode partial agent run usage: %w", err)
	}
	blocks := make([]servermodels.AgentRunBlock, 0, len(partial.Blocks))
	for _, block := range partial.Blocks {
		payload, err := json.Marshal(block.Payload)
		if err != nil {
			return fmt.Errorf("encode partial agent run block: %w", err)
		}
		blocks = append(blocks, servermodels.AgentRunBlock{
			ID: block.ID, OrganizationID: initial.OrganizationID, AgentRunID: initial.ID,
			Position: block.Position, ModelCallID: block.ModelCallID, Kind: string(block.Kind), Payload: payload,
		})
	}
	return realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		conversation, err := chatstate.LockConversation(ctx, tx, initial.OrganizationID, initial.ConversationID)
		if err != nil {
			return err
		}
		run := &servermodels.AgentRun{}
		if err := tx.NewSelect().Model(run).Where("agr.id = ?", initial.ID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock agent run for partial process: %w", err)
		}
		if !agentRunStatusTerminal(run.Status) {
			return nil
		}
		// 成功运行在结果事务中写入完整过程；同一运行的重复执行尝试只保留最早写入的一份。
		written, err := tx.NewSelect().Model((*servermodels.AgentRunBlock)(nil)).
			Where("arb.organization_id = ? AND arb.agent_run_id = ?", initial.OrganizationID, initial.ID).Exists(ctx)
		if err != nil {
			return fmt.Errorf("check persisted agent run blocks: %w", err)
		}
		if written {
			return nil
		}
		if _, err := tx.NewInsert().Model(&blocks).Exec(ctx); err != nil {
			return fmt.Errorf("persist partial agent run blocks: %w", err)
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("usage = ?::jsonb", string(usage)).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return fmt.Errorf("persist partial agent run usage: %w", err)
		}
		return chatstate.TouchConversation(ctx, tx, conversation)
	})
}

// logCompletedRun 记录关联客服周期的 Agent 完成结果。
func logCompletedRun(execution executionContext, endSeq int64, messageID string) {
	if domain.AgentExecutionScopeKind(execution.Run.ScopeKind) != domain.AgentExecutionScopeServiceSession {
		return
	}
	slog.Info("客户 Agent 运行完成",
		"agent_run_id", execution.Run.ID,
		"conversation_id", execution.Run.ConversationID,
		"service_session_id", execution.Run.ScopeID,
		"input_start_seq", execution.Run.InputStartSeq,
		"input_end_seq", endSeq,
		"response_message_id", messageID,
	)
}

// fail 按运行策略取消失效运行或标记失败：客服运行转交人工，其他运行写入错误消息并为剩余输入补建下一次运行。
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
	policy, err := a.policyForRun(ctx, initial)
	if err != nil {
		return false, err
	}
	if domain.AgentExecutionScopeKind(initial.ScopeKind) == domain.AgentExecutionScopeServiceSession {
		reason := domain.AgentHandoffReasonRuntimeFailed
		if errors.Is(runErr, context.DeadlineExceeded) {
			reason = domain.AgentHandoffReasonTimeout
		}
		return a.failCustomerRun(ctx, initial, policy, message, reason)
	}
	terminal := false
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		locked, err := lockAgentRun(ctx, tx, policy, initial)
		if err != nil {
			return fmt.Errorf("lock agent run for failure: %w", err)
		}
		policyContext, lane, run := locked.PolicyContext, locked.Lane, locked.Run
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
			if err := chatstate.TouchConversation(ctx, tx, policyContext.Conversation); err != nil {
				return err
			}
			return scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID)
		}
		if run.Status != string(domain.AgentRunStatusQueued) && run.Status != string(domain.AgentRunStatusRunning) {
			return fmt.Errorf("cannot fail agent run in status %q", run.Status)
		}
		failureEnd := run.InputStartSeq
		if run.InputEndSeq != nil {
			failureEnd = *run.InputEndSeq
		}
		if run.InputStartSeq != lane.ProcessedSeq+1 || failureEnd < run.InputStartSeq || failureEnd > lane.DesiredSeq {
			return errors.New("agent run failure boundary is inconsistent")
		}
		failedSeqs, err := claimLaneInputs(ctx, tx, run, lane.ProcessedSeq, failureEnd)
		if err != nil {
			return err
		}
		if int64(len(failedSeqs)) != failureEnd-lane.ProcessedSeq {
			return errors.New("failed agent input sequence is not contiguous")
		}
		messageID := uuid.NewV7().String()
		if err := policy.persistMessage(ctx, tx, policyContext, run, messageID, domain.MessageTypeAgentError, ""); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(run).
			Set("status = ?", domain.AgentRunStatusFailed).
			Set("response_message_id = ?", messageID).
			Set("input_end_seq = ?", failureEnd).
			Set("last_error = ?", message).
			Set("error_code = NULL").
			Set("completed_at = now()").
			Set("updated_at = now()").
			WherePK().
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewUpdate().Model(lane).
			Set("processed_seq = ?", failureEnd).
			Set("updated_at = now()").
			WherePK().Exec(ctx); err != nil {
			return err
		}
		return scheduleNextRun(ctx, tx, a.enqueuer, policy, policyContext, run.OrganizationID, domain.AgentExecutionScopeKind(run.ScopeKind), run.ScopeID)
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
