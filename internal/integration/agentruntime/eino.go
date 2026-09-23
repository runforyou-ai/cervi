package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	defaultMaxIterations = 20
	// emptyResponseRetryLimit 是同一次输入在模型只产出推理内容时允许的重新执行次数。
	emptyResponseRetryLimit = 1
)

type runIDContextKey struct{}

// EinoRuntime 使用 AgenticMessage TurnLoop 执行 Agent。
type EinoRuntime struct {
	newModel modelFactory
	tools    []tool.BaseTool
}

// New 创建带计算器 Tool 的 Eino Runtime。
func New() (*EinoRuntime, error) {
	// 框架内置提示与本项目面向模型的提示统一使用中文。
	if err := adk.SetLanguage(adk.LanguageChinese); err != nil {
		return nil, fmt.Errorf("set agent runtime language: %w", err)
	}
	calculator, err := newCalculatorTool()
	if err != nil {
		return nil, fmt.Errorf("create calculator tool: %w", err)
	}
	return &EinoRuntime{newModel: newAgenticModel, tools: []tool.BaseTool{calculator}}, nil
}

// Run 执行受迭代上限和 context 控制的 TurnLoop，并在安全点吸收后续输入。
func (r *EinoRuntime) Run(ctx context.Context, request RunRequest, feed InputFeed) (RunResult, error) {
	if feed == nil {
		return RunResult{}, errors.New("agent input feed is required")
	}
	ctx = context.WithValue(ctx, runIDContextKey{}, request.RunID)
	recorder := newProcessRecorder(request)
	recorder.publisher.start()
	defer recorder.publisher.close()
	maxIterations := request.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	modelConfig := request.modelConfig()
	chatModel, err := r.newModel(ctx, modelConfig)
	if err != nil {
		return RunResult{}, err
	}
	// 客服场景注册终止工具，其纠正额度在同一执行尝试内的重新执行之间共用；严格依据策略按本次注册的工具登记依据来源。
	var terminal *terminalTools
	var gate *groundingGate
	if request.Assignment.Scene == SceneCustomer {
		terminal = newTerminalTools()
		if request.Assignment.Grounding == GroundingStrict {
			judges := make(map[string]evidenceJudge)
			if request.KnowledgeSearch != nil {
				judges[knowledgeToolName] = knowledgeEvidence
			}
			gate = newGroundingGate(judges)
		}
	}
	tools, releaseSessions, err := r.assembleTools(ctx, request, terminal)
	if err != nil {
		return RunResult{}, err
	}
	defer releaseSessions()
	window := ContextWindowTokens(modelConfig)
	reductionHandlers, err := newContextReductionHandlers(ctx, window)
	if err != nil {
		return RunResult{}, err
	}
	// 模型声明文本以外的输入模态且执行侧提供附件读取时，按窗口推导随消息直传的附件数量上限，至少直传一个。
	media := mediaInput{read: request.ReadAttachment, modalities: make(map[domain.AIModelInputModality]bool)}
	for _, modality := range request.Assignment.Model.InputModalities {
		if modality != domain.AIModelInputModalityText {
			media.modalities[modality] = true
		}
	}
	if len(media.modalities) > 0 && media.read != nil {
		media.maxCount = max(1, window*mediaWindowPercent/100/mediaTokens)
	}
	trackedModel := &mediaTrackingModel{AgenticModel: chatModel, rejected: &atomic.Bool{}}
	guard := newFinalIterationGuard(maxIterations, terminal != nil)
	if terminal != nil {
		terminal.budgetSpent = guard.budgetExhausted
	}
	handlers := append([]adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]{recorder, guard}, reductionHandlers...)
	handlers = append(handlers, &toolArgumentsNormalizer{})
	toolMiddlewares := []compose.ToolMiddleware{toolExecutionMiddleware(recorder)}
	if terminal != nil {
		handlers = append(handlers, terminal)
		toolMiddlewares = append(toolMiddlewares, terminal.middleware())
	}
	if gate != nil {
		handlers = append(handlers, gate)
	}
	agent, err := adk.NewTypedChatModelAgent(ctx, &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
		Name: request.Assignment.AgentName, Instruction: request.Assignment.Instruction, Model: trackedModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: tools, ToolCallMiddlewares: toolMiddlewares,
		}},
		Handlers:      handlers,
		MaxIterations: maxIterations,
	})
	if err != nil {
		return RunResult{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	// 模型偶发只产出推理内容而没有正文时按有界次数重新执行；携带直传附件的模型调用失败时去掉多模态内容重新执行一次。
	var carriedUsage Usage
	for emptyRetries := 0; ; {
		execution := &einoExecution{
			inputs: &turnInputs{feed: feed, holdPreempt: terminal.handoffFixed}, recorder: recorder, terminal: terminal, gate: gate, guard: guard,
			maxTurns: request.MaxTurns, contextWindow: window, media: media,
		}
		execution.inputs.loop = adk.NewTurnLoop(adk.TurnLoopConfig[Trigger, *schema.AgenticMessage]{
			GenInput: execution.genInput,
			PrepareAgent: func(context.Context, *adk.TurnLoop[Trigger, *schema.AgenticMessage], []Trigger) (adk.TypedAgent[*schema.AgenticMessage], error) {
				return agent, nil
			},
			OnAgentEvents: execution.onAgentEvents,
		})
		err := execution.inputs.run(ctx)
		carriedUsage.PromptTokens += execution.result.Usage.PromptTokens
		carriedUsage.CompletionTokens += execution.result.Usage.CompletionTokens
		carriedUsage.TotalTokens += execution.result.Usage.TotalTokens
		if errors.Is(err, errEmptyFinalResponse) && emptyRetries < emptyResponseRetryLimit && ctx.Err() == nil {
			emptyRetries++
			slog.Warn("模型未产出正文，重新执行本次输入",
				"agent_run_id", request.RunID, "attempt", emptyRetries, "retry_limit", emptyResponseRetryLimit)
			recorder.reset()
			continue
		}
		if err != nil && ctx.Err() == nil && media.maxCount > 0 && trackedModel.rejected.Load() {
			slog.Warn("模型调用拒绝直传附件，改为仅在正文提供附件链接并重新执行",
				"agent_run_id", request.RunID, "error", err)
			recorder.reset()
			media = mediaInput{}
			continue
		}
		if err != nil {
			return RunResult{Usage: carriedUsage, Blocks: recorder.partialBlocks()}, err
		}
		if !execution.finished || execution.inputs.claimedSeq <= 0 {
			return RunResult{Usage: carriedUsage, Blocks: recorder.partialBlocks()},
				errors.New("agent run stopped without a stable response")
		}
		execution.result.Usage = carriedUsage
		execution.result.EndSeq = execution.inputs.claimedSeq
		execution.result.Blocks = recorder.blocks()
		return execution.result, nil
	}
}

// assembleTools 按场景与请求装配本次运行的工具：开发期计算器只在内部场景注册，终止工具只在客服场景注册，远程 MCP 工具在内置工具之后连接并跳过重名。
// 工具集合由本次运行注入的依赖决定，调用方必须让注入的依赖与有效配置中的工具清单一致。
func (r *EinoRuntime) assembleTools(ctx context.Context, request RunRequest, terminal *terminalTools) ([]tool.BaseTool, func(), error) {
	tools := make([]tool.BaseTool, 0, len(r.tools)+4)
	if request.Assignment.Scene != SceneCustomer {
		tools = append(tools, r.tools...)
	}
	if request.KnowledgeSearch != nil {
		knowledgeTool, err := newKnowledgeSearchTool(request.KnowledgeSearch)
		if err != nil {
			return nil, nil, fmt.Errorf("create knowledge search tool: %w", err)
		}
		tools = append(tools, knowledgeTool)
	}
	if request.CustomerHistorySearch != nil {
		historyTool, err := newCustomerHistoryTool(request.CustomerHistorySearch)
		if err != nil {
			return nil, nil, fmt.Errorf("create customer history tool: %w", err)
		}
		tools = append(tools, historyTool)
	}
	if terminal != nil {
		tools = append(tools, terminal.tools()...)
	}
	release := func() {}
	if len(request.MCPConnections) > 0 {
		// 收齐本次运行的内置工具名称，远程工具重名时由 openMCPTools 跳过。
		registered := map[string]struct{}{offloadedResultToolName: {}}
		for _, existing := range tools {
			info, err := existing.Info(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("read registered tool info: %w", err)
			}
			registered[info.Name] = struct{}{}
		}
		var mcpTools []tool.BaseTool
		mcpTools, release = openMCPTools(ctx, request.RunID, request.MCPConnections, registered)
		tools = append(tools, mcpTools...)
	}
	return tools, release, nil
}

// einoExecution 保存单次运行的上下文、轮次和结果，回调按轮次顺序访问。
type einoExecution struct {
	inputs        *turnInputs
	history       turnHistory
	recorder      *processRecorder
	terminal      *terminalTools
	gate          *groundingGate
	guard         *finalIterationGuard
	rejected      []*schema.AgenticMessage // 被依据门禁拦下、只进入下一次纠正重新执行的正文。
	maxTurns      int
	contextWindow int
	media         mediaInput
	turns         int
	result        RunResult
	finished      bool
}

// genInput 认领新输入，并在已有执行上下文后追加尚未消费的会话消息；只有依据纠正信号时在当前边界内追加纠正提示重新执行。
func (e *einoExecution) genInput(ctx context.Context, _ *adk.TurnLoop[Trigger, *schema.AgenticMessage], items []Trigger) (*adk.GenInputResult[Trigger, *schema.AgenticMessage], error) {
	e.recorder.resetCandidate()
	var throughSeq int64
	claiming := false
	for _, item := range items {
		if !item.Correction {
			throughSeq, claiming = max(throughSeq, item.Seq), true
		}
	}
	var messages []*schema.AgenticMessage
	switch {
	case claiming:
		if e.terminal != nil {
			e.terminal.beginTurn()
		}
		e.turns++
		if e.maxTurns > 0 && e.turns > e.maxTurns {
			return nil, fmt.Errorf("agent turn limit %d exceeded", e.maxTurns)
		}
		if e.gate != nil {
			e.gate.resetBoundary(e.history.messages)
		}
		e.rejected = nil
		claimed, err := e.inputs.claim(ctx, throughSeq)
		if err != nil {
			return nil, err
		}
		messages = e.history.appendInput(ctx, trimClaimedHistory(ctx, claimed.Messages, e.contextWindow), e.media)
	case e.gate != nil:
		// 纠正重新执行不认领输入、不计轮次，沿用当前边界的依据与剩余迭代预算。
		lookup := ""
		// 按名称顺序列出本次登记的依据来源工具。
		if tools := slices.Sorted(maps.Keys(e.gate.judges)); len(tools) > 0 {
			lookup = "涉及企业具体信息请先调用 " + strings.Join(tools, "、") + " 查证；"
		}
		correction := "【系统提示】上面的回答没有取得依据，不会发给客户。" + lookup + "需要客户补充信息或只是问候请调用 ask_customer；无法解答请调用 handoff_to_human。"
		// 被拦下的回答与纠正提示只进入本次重新执行的输入，不写入后续轮次的历史。
		if len(e.rejected) == 0 {
			return nil, errors.New("grounding correction has no rejected response")
		}
		messages = append(append(append([]*schema.AgenticMessage(nil), e.history.messages...), e.rejected...), schema.UserAgenticMessage(correction))
		e.rejected = nil
		e.guard.carryBudget()
	default:
		return nil, errors.New("agent turn loop received a correction without grounding gate")
	}
	return &adk.GenInputResult[Trigger, *schema.AgenticMessage]{
		Input: &adk.TypedAgentInput[*schema.AgenticMessage]{
			Messages:        messages,
			EnableStreaming: true,
		},
		RunOpts: []adk.AgentRunOption{
			adk.WithAfterToolCallsHook(func(hookCtx context.Context) error {
				return e.inputs.poll(hookCtx, true)
			}),
		},
		Consumed: items,
	}, nil
}

// onAgentEvents 保存完整中间消息，并由输入协调器决定继续下一轮或收尾。
func (e *einoExecution) onAgentEvents(ctx context.Context, turn *adk.TurnContext[Trigger, *schema.AgenticMessage], events *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.AgenticMessage]]) error {
	candidate := ""
	var resultCallIDs []string
	var intermediates []*schema.AgenticMessage
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			if _, ok := errors.AsType[*adk.CancelError](event.Err); ok {
				continue
			}
			return event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return err
		}
		// 模型输出以 assistant 角色返回，工具结果以携带结果块的 user 角色返回。
		if message == nil || message.Role == schema.AgenticRoleTypeSystem {
			continue
		}
		intermediates = append(intermediates, message)
		if message.Role != schema.AgenticRoleTypeAssistant {
			for _, block := range message.ContentBlocks {
				if block.Type == schema.ContentBlockTypeFunctionToolResult {
					resultCallIDs = append(resultCallIDs, block.FunctionToolResult.CallID)
				}
			}
			continue
		}
		if message.ResponseMeta != nil && message.ResponseMeta.TokenUsage != nil {
			e.result.Usage.PromptTokens += message.ResponseMeta.TokenUsage.PromptTokens
			e.result.Usage.CompletionTokens += message.ResponseMeta.TokenUsage.CompletionTokens
			e.result.Usage.TotalTokens += message.ResponseMeta.TokenUsage.TotalTokens
		}
		if text := strings.TrimSpace(assistantText(message)); !hasToolCalls(message) && text != "" {
			candidate = text
		}
	}
	e.history.appendOutput(intermediates)
	// 终止工具的意图按本轮成功返回的调用编号取得，没有终止意图时以正文作为回答。
	var decision TerminalDecision
	content := candidate
	if intent, ok := e.terminal.decision(resultCallIDs); ok {
		decision, content = intent.decision, intent.message
	}
	// 严格依据策略下，确认没有新输入后才检查正文依据：纠正额度内重新执行一次，额度或迭代预算用尽时转人工。
	if e.gate != nil && decision.Kind == "" && content != "" && !e.gate.verdict() {
		pending, err := e.inputs.pending(ctx, turn)
		if err != nil {
			return err
		}
		if pending {
			e.takeUnsent(decision, content)
			return nil
		}
		exhausted := e.guard.budgetExhausted()
		if !exhausted && e.terminal.takeCorrection() {
			slog.Warn("Agent 正文缺少依据，要求模型纠正", "agent_run_id", runIDFromContext(ctx))
			e.rejected = e.takeUnsent(decision, content)
			return e.inputs.rerun()
		}
		reason := domain.AgentHandoffReasonInsufficientEvidence
		if exhausted {
			reason = domain.AgentHandoffReasonBudgetExhausted
		}
		slog.Warn("Agent 正文缺少依据，转交人工", "agent_run_id", runIDFromContext(ctx), "reason", reason)
		decision, content = TerminalDecision{Kind: domain.AgentRunOutcomeHandoff, Reason: reason}, ""
	}
	finished, err := e.inputs.finish(ctx, turn, decision, content)
	if err != nil {
		return err
	}
	if finished {
		e.result.Content, e.result.Decision, e.finished = content, decision, true
		return nil
	}
	e.takeUnsent(decision, content)
	return nil
}

// takeUnsent 在客服场景从共享历史末尾取出本轮未发给客户的正文、追问或结束语调用及其结果，返回取出的消息；其他场景保留历史。
func (e *einoExecution) takeUnsent(decision TerminalDecision, content string) []*schema.AgenticMessage {
	messages := e.history.messages
	if e.terminal == nil || content == "" || len(messages) == 0 {
		return nil
	}
	last := len(messages) - 1
	switch decision.Kind {
	case "":
		// 正文是末尾一条只含正文的模型输出。
		if messages[last].Role != schema.AgenticRoleTypeAssistant || hasToolCalls(messages[last]) ||
			strings.TrimSpace(assistantText(messages[last])) != content {
			return nil
		}
		e.history.messages = messages[:last]
		return messages[last:]
	case domain.AgentRunOutcomeAskCustomer, domain.AgentRunOutcomeResolve:
		// 追问或结束语是末尾一条只调用对应终止工具的模型输出及其工具结果。
		toolName := askCustomerToolName
		if decision.Kind == domain.AgentRunOutcomeResolve {
			toolName = resolveToolName
		}
		if last < 1 || messages[last-1].Role != schema.AgenticRoleTypeAssistant || messages[last].Role == schema.AgenticRoleTypeAssistant {
			return nil
		}
		for _, block := range messages[last-1].ContentBlocks {
			if block.Type == schema.ContentBlockTypeFunctionToolCall && block.FunctionToolCall.Name != toolName {
				return nil
			}
		}
		if !hasToolCalls(messages[last-1]) {
			return nil
		}
		e.history.messages = messages[:last-1]
		return messages[last-1:]
	}
	return nil
}

// assistantText 拼接模型输出中的全部正文块。
func assistantText(message *schema.AgenticMessage) string {
	var text strings.Builder
	for _, block := range message.ContentBlocks {
		if block.Type == schema.ContentBlockTypeAssistantGenText {
			text.WriteString(block.AssistantGenText.Text)
		}
	}
	return text.String()
}

// hasToolCalls 判断模型输出是否包含工具调用块。
func hasToolCalls(message *schema.AgenticMessage) bool {
	for _, block := range message.ContentBlocks {
		if block.Type == schema.ContentBlockTypeFunctionToolCall {
			return true
		}
	}
	return false
}

// runIDFromContext 返回当前 Runtime 传给组件的 Agent Run 编号。
func runIDFromContext(ctx context.Context) string {
	runID, _ := ctx.Value(runIDContextKey{}).(string)
	return runID
}
