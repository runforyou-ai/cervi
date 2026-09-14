//go:build server

package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	// emptyResponseRetryLimit 限制空正文的重新执行次数，避免对同一输入无界重算。
	emptyResponseRetryLimit = 1
)

type runIDContextKey struct{}

// EinoRuntime 使用标准 Message TurnLoop 执行 Agent。
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
	return &EinoRuntime{newModel: newOpenAICompatibleModel, tools: []tool.BaseTool{calculator}}, nil
}

// Run 执行受迭代上限和 context 控制的 TurnLoop，并在安全点吸收后续输入。
func (r *EinoRuntime) Run(ctx context.Context, request RunRequest, feed InputFeed) (RunResult, error) {
	if feed == nil {
		return RunResult{}, errors.New("agent input feed is required")
	}
	ctx = context.WithValue(ctx, runIDContextKey{}, request.RunID)
	recorder := newProcessRecorder(request)
	maxIterations := request.MaxIterations
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	chatModel, err := r.newModel(ctx, request.Model)
	if err != nil {
		return RunResult{}, err
	}
	tools := append([]tool.BaseTool(nil), r.tools...)
	if request.KnowledgeSearch != nil {
		knowledgeTool, toolErr := newKnowledgeSearchTool(request.KnowledgeSearch)
		if toolErr != nil {
			return RunResult{}, fmt.Errorf("create knowledge search tool: %w", toolErr)
		}
		tools = append(tools, knowledgeTool)
	}
	if request.CustomerHistorySearch != nil {
		historyTool, toolErr := newCustomerHistoryTool(request.CustomerHistorySearch)
		if toolErr != nil {
			return RunResult{}, fmt.Errorf("create customer history tool: %w", toolErr)
		}
		tools = append(tools, historyTool)
	}
	if len(request.MCPServers) > 0 {
		// 收齐本次运行的内置工具名称，远程工具重名时由 openMCPTools 跳过。
		registered := map[string]struct{}{offloadedResultToolName: {}}
		for _, existing := range tools {
			info, infoErr := existing.Info(ctx)
			if infoErr != nil {
				return RunResult{}, fmt.Errorf("read registered tool info: %w", infoErr)
			}
			registered[info.Name] = struct{}{}
		}
		mcpTools, releaseSessions := openMCPTools(ctx, request.RunID, request.MCPServers, registered)
		defer releaseSessions()
		tools = append(tools, mcpTools...)
	}
	window := contextWindowTokens(request.Model)
	reductionHandlers, err := newContextReductionHandlers(ctx, window)
	if err != nil {
		return RunResult{}, err
	}
	// 模型声明文本以外的输入模态时，按窗口推导随消息直传的附件数量上限，至少直传一个。
	media := mediaInput{read: request.ReadAttachment, modalities: make(map[domain.AIModelInputModality]bool)}
	for _, modality := range request.Model.InputModalities {
		if modality != domain.AIModelInputModalityText {
			media.modalities[modality] = true
		}
	}
	if len(media.modalities) > 0 {
		media.maxCount = max(1, window*mediaWindowPercent/100/mediaTokens)
	}
	trackedModel := &mediaTrackingModel{ToolCallingChatModel: chatModel, rejected: &atomic.Bool{}}
	handlers := append([]adk.ChatModelAgentMiddleware{recorder, newFinalIterationGuard(maxIterations)}, reductionHandlers...)
	handlers = append(handlers, &toolArgumentsNormalizer{})
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: request.Name, Instruction: request.Instruction, Model: trackedModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: tools, ToolCallMiddlewares: []compose.ToolMiddleware{toolExecutionMiddleware(recorder)},
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
			inputs: &turnInputs{feed: feed}, recorder: recorder, maxTurns: request.MaxTurns, contextWindow: window, media: media,
		}
		execution.inputs.loop = adk.NewTurnLoop(adk.TurnLoopConfig[Trigger, *schema.Message]{
			GenInput: execution.genInput,
			PrepareAgent: func(context.Context, *adk.TurnLoop[Trigger, *schema.Message], []Trigger) (adk.Agent, error) {
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
			return RunResult{}, err
		}
		if execution.result.Content == "" || execution.inputs.claimedSeq <= 0 {
			return RunResult{}, errors.New("agent run stopped without a stable response")
		}
		execution.result.Usage = carriedUsage
		execution.result.EndSeq = execution.inputs.claimedSeq
		execution.result.Blocks = recorder.blocks()
		return execution.result, nil
	}
}

// einoExecution 保存单次运行的上下文、轮次和结果，回调按轮次顺序访问。
type einoExecution struct {
	inputs        *turnInputs
	history       turnHistory
	recorder      *processRecorder
	maxTurns      int
	contextWindow int
	media         mediaInput
	turns         int
	result        RunResult
}

// genInput 认领新输入，并在已有执行上下文后追加尚未消费的会话消息。
func (e *einoExecution) genInput(ctx context.Context, _ *adk.TurnLoop[Trigger, *schema.Message], items []Trigger) (*adk.GenInputResult[Trigger, *schema.Message], error) {
	e.turns++
	e.recorder.resetCandidate()
	if e.maxTurns > 0 && e.turns > e.maxTurns {
		return nil, fmt.Errorf("agent turn limit %d exceeded", e.maxTurns)
	}
	var throughSeq int64
	for _, item := range items {
		throughSeq = max(throughSeq, item.Seq)
	}
	claimed, err := e.inputs.claim(ctx, throughSeq)
	if err != nil {
		return nil, err
	}
	return &adk.GenInputResult[Trigger, *schema.Message]{
		Input: &adk.AgentInput{Messages: e.history.appendInput(ctx, trimClaimedHistory(ctx, claimed.Messages, e.contextWindow), e.media)},
		RunOpts: []adk.AgentRunOption{
			adk.WithAfterToolCallsHook(func(hookCtx context.Context) error {
				return e.inputs.poll(hookCtx, true)
			}),
		},
		Consumed: items,
	}, nil
}

// onAgentEvents 保存完整中间消息，并由输入协调器决定继续下一轮或收尾。
func (e *einoExecution) onAgentEvents(ctx context.Context, turn *adk.TurnContext[Trigger, *schema.Message], events *adk.AsyncIterator[*adk.AgentEvent]) error {
	candidate := ""
	var intermediates []*schema.Message
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
		if message == nil || (message.Role != schema.Assistant && message.Role != schema.Tool) {
			continue
		}
		intermediates = append(intermediates, message)
		if message.Role != schema.Assistant {
			continue
		}
		if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
			e.result.Usage.PromptTokens += message.ResponseMeta.Usage.PromptTokens
			e.result.Usage.CompletionTokens += message.ResponseMeta.Usage.CompletionTokens
			e.result.Usage.TotalTokens += message.ResponseMeta.Usage.TotalTokens
		}
		if len(message.ToolCalls) == 0 && strings.TrimSpace(message.Content) != "" {
			candidate = strings.TrimSpace(message.Content)
		}
	}
	e.history.appendOutput(intermediates)
	finished, err := e.inputs.finish(ctx, turn, candidate)
	if err != nil {
		return err
	}
	if finished {
		e.result.Content = candidate
	}
	return nil
}

// runIDFromContext 返回当前 Runtime 传给组件的 Agent Run 编号。
func runIDFromContext(ctx context.Context) string {
	runID, _ := ctx.Value(runIDContextKey{}).(string)
	return runID
}
