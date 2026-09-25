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
	"github.com/runforyou-ai/cervi/internal/actions/customernotify"
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
	db           *bun.DB
	enqueuer     servertask.TxEnqueuer
	runtime      agentruntime.Runtime
	attachments  *AttachmentReader
	knowledge    KnowledgeRetrieval
	emailSender  customernotify.Sender
	runningMu    sync.Mutex
	runningRuns  map[string]*runningAgentRun
	typingMu     sync.Mutex
	deviceTyping map[string]*runTyping
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
	HandlesCustomers bool                          `bun:"handles_customers"`
	OrganizationName string                        `bun:"organization_name"`
}

// NewExecuteAction 创建 Agent Worker Action。
func NewExecuteAction(db *bun.DB, enqueuer servertask.TxEnqueuer, runtime agentruntime.Runtime, attachments *AttachmentReader, knowledge KnowledgeRetrieval, emailSender customernotify.Sender) *ExecuteAction {
	return &ExecuteAction{db: db, enqueuer: enqueuer, runtime: runtime, attachments: attachments, knowledge: knowledge, emailSender: emailSender, runningRuns: make(map[string]*runningAgentRun), deviceTyping: make(map[string]*runTyping)}
}

// runAssignment 表示一次已认领运行的执行指派：有效配置、运行期依赖与本次执行的取消与流式句柄。
type runAssignment struct {
	Execution      executionContext
	Policy         agentRunPolicy
	Assignment     agentruntime.Assignment
	MCPConnections []agentruntime.MCPServer
	Knowledge      agentruntime.KnowledgeSearch
	History        agentruntime.CustomerHistorySearch
	Running        *runningAgentRun
	RunCtx         context.Context
	Release        func() // 结束本次执行的输入状态、取消注册与运行 context。
}

// Execute 取得执行指派、运行 TurnLoop 并收尾，只保存吸收完当前输入后的稳定回复。
func (a *ExecuteAction) Execute(ctx context.Context, input RunInput) error {
	if !common.ValidUUID(input.RunID) {
		return task.Permanent(errors.New("agent run id is invalid"))
	}
	assigned, assignErr := a.assign(ctx, input.RunID)
	if assigned.Release != nil {
		defer assigned.Release()
	}
	if assignErr != nil || assigned.Running == nil {
		return assignErr
	}
	result, runErr := a.runAssigned(assigned)
	return a.settle(ctx, assigned, result, runErr)
}

// assign 取得一次运行的执行指派：标记运行中、注册取消句柄、解析有效配置并装配运行期依赖；运行已进入终态时返回空指派。
func (a *ExecuteAction) assign(ctx context.Context, runID string) (runAssignment, error) {
	execution, terminal, err := a.begin(ctx, runID)
	if err != nil || terminal {
		return runAssignment{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, agentRunTimeout)
	running, unregister, err := a.registerRunContext(ctx, execution.Run.ID, cancel)
	if err != nil {
		cancel()
		return runAssignment{}, err
	}
	// 生成期间向会话受众发布 AI 员工正在输入。
	stopTyping := a.startServerRunTyping(runCtx, &execution.Run)
	assigned := runAssignment{
		Execution: execution, Running: running, RunCtx: runCtx,
		Release: func() {
			stopTyping()
			unregister()
			cancel()
		},
	}
	if running.attempt > 1 {
		taskExecution, _ := servertask.CurrentExecution(ctx)
		slog.Warn("Agent 任务重新计算", "agent_run_id", execution.Run.ID, "task_run_id", taskExecution.TaskRunID,
			"attempt", running.attempt, "stream_id", running.streamID)
	}
	assigned.Policy, err = a.policyForRun(ctx, &execution.Run)
	if err != nil {
		return assigned, task.Permanent(err)
	}
	if domain.AgentExecutionScopeKind(execution.Run.ScopeKind) == domain.AgentExecutionScopeServiceSession {
		// TODO：接入本企业、本 Conversation 内已关闭 ServiceSession 的全文历史查询。
		// 向模型返回历史查询功能不可用的占位结果。
		assigned.History = func(context.Context, string) (agentruntime.CustomerHistoryResult, error) {
			return agentruntime.CustomerHistoryResult{
				Available: false,
				Message:   "历史消息查询暂未开放，无法确认以往的沟通内容。请根据本轮消息回答，必要时请客户补充信息；不要重复调用此工具。",
			}, nil
		}
	}
	mcpServers, err := loadRunMCPServers(ctx, a.db, &execution.Run)
	if err != nil {
		return assigned, fmt.Errorf("load agent run mcp servers: %w", err)
	}
	assigned.MCPConnections = mcpServers.Servers
	assigned.Knowledge, err = loadRunKnowledgeSearch(ctx, a.db, a.knowledge, execution)
	if err != nil {
		return assigned, fmt.Errorf("load agent run knowledge bases: %w", err)
	}
	serverNames := make([]string, 0, len(assigned.MCPConnections))
	for _, server := range assigned.MCPConnections {
		serverNames = append(serverNames, server.Name)
	}
	assigned.Assignment, err = a.resolveAssignment(ctx, execution, assigned.Policy,
		agentruntime.Capabilities{Knowledge: assigned.Knowledge != nil, MCPServers: serverNames, CustomerLoginRequired: mcpServers.CustomerLoginRequired})
	if err != nil {
		return assigned, err
	}
	return assigned, nil
}

// runAssigned 按执行指派运行 TurnLoop，运行时限到期时统一以超时原因返回。
func (a *ExecuteAction) runAssigned(assigned runAssignment) (agentruntime.RunResult, error) {
	execution, running := assigned.Execution, assigned.Running
	feed := &databaseInputFeed{db: a.db, enqueuer: a.enqueuer, execution: execution, policy: assigned.Policy, attachments: a.attachments}
	// 场景、依据策略、指令、模型参数与输入模态以有效配置为准，供应商凭据取当前配置。
	result, err := a.runtime.Run(assigned.RunCtx, agentruntime.RunRequest{
		RunID:                 execution.Run.ID,
		Assignment:            assigned.Assignment,
		Credentials:           agentruntime.ModelCredentials{APIKey: execution.APIKey, BaseURL: execution.APIURL},
		KnowledgeSearch:       assigned.Knowledge,
		CustomerHistorySearch: assigned.History,
		ReadAttachment: func(ctx context.Context, messageID string) ([]byte, error) {
			return a.attachments.Content(ctx, &execution.Run, messageID)
		},
		MCPConnections: assigned.MCPConnections,
		StreamID:       running.streamID,
		Attempt:        running.attempt,
		OnStream: func(delta agentruntime.StreamDelta) {
			// 运行 context 已取消时丢弃增量。
			if assigned.RunCtx.Err() == nil {
				running.stream.Publish(delta)
			}
		},
	}, feed)
	if err != nil && errors.Is(assigned.RunCtx.Err(), context.DeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("%w: %w", err, context.DeadlineExceeded)
	}
	return result, err
}

// settle 按运行结果收尾：抑制失效结果、原子写入回复或标记失败，并保留已产生的过程内容。
func (a *ExecuteAction) settle(ctx context.Context, assigned runAssignment, result agentruntime.RunResult, runErr error) error {
	execution := assigned.Execution
	if errors.Is(runErr, errAgentRunSuppressed) {
		// 运行吸收后续输入时已失去资格，保留此前已产生的过程内容。
		return a.persistPartialProcess(ctx, &execution.Run, result)
	}
	if runErr == nil {
		if completeErr := a.complete(ctx, execution, assigned.Policy, result); completeErr != nil {
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
	terminal, failErr := a.fail(ctx, execution.Run.ID, runErr, "")
	if failErr != nil {
		return fmt.Errorf("agent run failed: %v; persist failure: %w", runErr, failErr)
	}
	// 失败与被取消的运行同样保留已产生的过程内容。
	if processErr := a.persistPartialProcess(ctx, &execution.Run, result); processErr != nil {
		return fmt.Errorf("agent run failed: %v; persist partial process: %w", runErr, processErr)
	}
	if terminal {
		return nil
	}
	return task.Permanent(fmt.Errorf("execute agent run: %w", runErr))
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
		// 运行期间以 AI 员工的聊天主体发布输入状态。
		if _, err := chatstate.EnsureOrganizationIdentityChatSubject(ctx, tx, run.OrganizationID, run.AgentIdentityID, uuid.NewV7().String()); err != nil {
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
	return a.loadExecution(ctx, runID)
}

// loadExecution 读取运行中的业务运行及其锁定配置版本的执行配置，运行已进入终态时返回 true。
func (a *ExecuteAction) loadExecution(ctx context.Context, runID string) (executionContext, bool, error) {
	execution := executionContext{}
	err := a.db.NewSelect().
		TableExpr("agent_runs AS agr").
		ColumnExpr("agr.*").
		ColumnExpr("oi.display_name AS agent_name").
		ColumnExpr("aipm.input_modalities").
		ColumnExpr("ar.configuration->'knowledgeBaseIds' AS knowledge_base_ids").
		ColumnExpr("aip.id::text AS provider_id, oi.handles_customers, o.name AS organization_name").
		Join("JOIN agents AS a ON a.identity_id = agr.agent_identity_id AND a.organization_id = agr.organization_id").
		Join("JOIN organizations AS o ON o.id = agr.organization_id").
		Apply(func(query *bun.SelectQuery) *bun.SelectQuery {
			return withManagedAgentConfiguration(query, "agr.agent_revision_id")
		}).
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
		if deciding, ok := policy.(decisionPolicy); ok {
			if err := deciding.applyDecision(ctx, tx, policyContext, run, lane, result, messageID); err != nil {
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

// fail 按运行策略取消失效运行或标记失败：客服运行转交人工，其他运行写入错误消息、记录错误码并为剩余输入补建下一次运行；code 为空表示没有稳定错误码。
func (a *ExecuteAction) fail(ctx context.Context, runID string, runErr error, code domain.AgentRunErrorCode) (bool, error) {
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
			Set("error_code = NULLIF(?, '')", code).
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
	_, err := a.fail(ctx, input.RunID, runErr, "")
	return err
}
