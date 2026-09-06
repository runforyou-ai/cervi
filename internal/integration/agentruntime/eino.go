//go:build server

package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const defaultMaxIterations = 8

type runIDContextKey struct{}

// EinoRuntime 使用标准 Message TurnLoop 执行 Agent。
type EinoRuntime struct {
	newModel modelFactory
	tools    []tool.BaseTool
}

// New 创建带计算器 Tool 的 Eino Runtime。
func New() (*EinoRuntime, error) {
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
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: request.Name, Instruction: request.Instruction, Model: chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: tools, ToolCallMiddlewares: []compose.ToolMiddleware{toolExecutionMiddleware(recorder)},
		}},
		Handlers:      []adk.ChatModelAgentMiddleware{recorder},
		MaxIterations: maxIterations,
	})
	if err != nil {
		return RunResult{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	execution := &einoExecution{
		inputs: &turnInputs{feed: feed}, recorder: recorder, maxTurns: request.MaxTurns,
	}
	execution.inputs.loop = adk.NewTurnLoop(adk.TurnLoopConfig[Trigger, *schema.Message]{
		GenInput: execution.genInput,
		PrepareAgent: func(context.Context, *adk.TurnLoop[Trigger, *schema.Message], []Trigger) (adk.Agent, error) {
			return agent, nil
		},
		OnAgentEvents: execution.onAgentEvents,
	})
	if err := execution.inputs.run(ctx); err != nil {
		return RunResult{}, err
	}
	if execution.result.Content == "" || execution.inputs.claimedSeq <= 0 {
		return RunResult{}, errors.New("agent run stopped without a stable response")
	}
	execution.result.EndSeq = execution.inputs.claimedSeq
	execution.result.Blocks = recorder.blocks()
	return execution.result, nil
}

// einoExecution 保存单次运行的上下文、轮次和结果，回调按轮次顺序访问。
type einoExecution struct {
	inputs   *turnInputs
	history  turnHistory
	recorder *processRecorder
	maxTurns int
	turns    int
	result   RunResult
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
		Input: &adk.AgentInput{Messages: e.history.appendInput(claimed.Messages)},
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
