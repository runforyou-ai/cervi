//go:build server

package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultMaxTurns     = 8
	triggerPollInterval = 200 * time.Millisecond
)

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

// Run 执行有界 TurnLoop，并在安全点吸收持久化后续输入。
func (r *EinoRuntime) Run(ctx context.Context, request RunRequest, feed InputFeed) (RunResult, error) {
	if feed == nil {
		return RunResult{}, errors.New("agent input feed is required")
	}
	ctx = context.WithValue(ctx, runIDContextKey{}, request.RunID)
	maxTurns := request.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	chatModel, err := r.newModel(ctx, request.Model)
	if err != nil {
		return RunResult{}, err
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: request.Name, Instruction: request.Instruction, Model: chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{
			Tools: r.tools, ToolCallMiddlewares: []compose.ToolMiddleware{toolLoggingMiddleware()},
		}},
		MaxIterations: maxTurns,
	})
	if err != nil {
		return RunResult{}, fmt.Errorf("create Eino chat model agent: %w", err)
	}

	var stateMu sync.Mutex
	var maxPushedSeq int64
	var claimedSeq int64
	var turnCount int
	var finalContent string
	var usage Usage
	var watcherErr error

	loop := adk.NewTurnLoop(adk.TurnLoopConfig[Trigger, *schema.Message]{
		GenInput: func(ctx context.Context, loop *adk.TurnLoop[Trigger, *schema.Message], items []Trigger) (*adk.GenInputResult[Trigger, *schema.Message], error) {
			turnCount++
			if turnCount > maxTurns {
				return nil, fmt.Errorf("agent turn limit %d exceeded", maxTurns)
			}
			throughSeq := maxTriggerSeq(items)
			claimed, err := feed.Claim(ctx, throughSeq)
			if err != nil {
				return nil, err
			}
			if claimed.EndSeq <= 0 || len(claimed.Messages) == 0 {
				return nil, errors.New("agent input feed returned no claimed messages")
			}
			stateMu.Lock()
			claimedSeq = claimed.EndSeq
			if maxPushedSeq < claimed.EndSeq {
				maxPushedSeq = claimed.EndSeq
			}
			stateMu.Unlock()
			return &adk.GenInputResult[Trigger, *schema.Message]{
				Input: &adk.AgentInput{Messages: schemaMessages(claimed.Messages)},
				RunOpts: []adk.AgentRunOption{
					adk.WithAfterToolCallsHook(func(hookCtx context.Context) error {
						return r.pushPending(hookCtx, loop, feed, &stateMu, &maxPushedSeq, true)
					}),
				},
				Consumed: items,
			}, nil
		},
		PrepareAgent: func(context.Context, *adk.TurnLoop[Trigger, *schema.Message], []Trigger) (adk.Agent, error) {
			return agent, nil
		},
		OnAgentEvents: func(eventCtx context.Context, turn *adk.TurnContext[Trigger, *schema.Message], events *adk.AsyncIterator[*adk.AgentEvent]) error {
			candidate := ""
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
				if event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.Role != schema.Assistant {
					continue
				}
				message, err := event.Output.MessageOutput.GetMessage()
				if err != nil {
					return err
				}
				if message == nil {
					continue
				}
				if message.ResponseMeta != nil && message.ResponseMeta.Usage != nil {
					usage.PromptTokens += message.ResponseMeta.Usage.PromptTokens
					usage.CompletionTokens += message.ResponseMeta.Usage.CompletionTokens
					usage.TotalTokens += message.ResponseMeta.Usage.TotalTokens
				}
				if len(message.ToolCalls) == 0 && strings.TrimSpace(message.Content) != "" {
					candidate = strings.TrimSpace(message.Content)
				}
			}
			if turnPreempted(turn) {
				return nil
			}
			if err := r.pushPending(eventCtx, turn.Loop, feed, &stateMu, &maxPushedSeq, false); err != nil {
				return err
			}
			stateMu.Lock()
			hasBufferedInput := maxPushedSeq > claimedSeq
			stateMu.Unlock()
			if hasBufferedInput {
				return nil
			}
			if candidate == "" {
				return errors.New("agent returned an empty final response")
			}
			finalContent = candidate
			turn.Loop.Stop()
			return nil
		},
	})

	if err := r.pushPending(ctx, loop, feed, &stateMu, &maxPushedSeq, false); err != nil {
		return RunResult{}, err
	}
	stateMu.Lock()
	hasInitialInput := maxPushedSeq > 0
	stateMu.Unlock()
	if !hasInitialInput {
		return RunResult{}, errors.New("agent run has no pending trigger")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	loop.Run(runCtx)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		ticker := time.NewTicker(triggerPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := r.pushPending(runCtx, loop, feed, &stateMu, &maxPushedSeq, true); err != nil {
					if runCtx.Err() != nil {
						return
					}
					stateMu.Lock()
					watcherErr = err
					stateMu.Unlock()
					loop.Stop(adk.WithImmediate())
					return
				}
			}
		}
	}()
	exit := loop.Wait()
	cancel()
	<-watcherDone
	stateMu.Lock()
	deferredWatcherErr := watcherErr
	endSeq := claimedSeq
	stateMu.Unlock()
	if deferredWatcherErr != nil {
		return RunResult{}, deferredWatcherErr
	}
	if exit.ExitReason != nil {
		return RunResult{}, exit.ExitReason
	}
	if finalContent == "" || endSeq <= 0 {
		return RunResult{}, errors.New("agent run stopped without a stable response")
	}
	return RunResult{Content: finalContent, EndSeq: endSeq, Usage: usage}, nil
}

// runIDFromContext 返回当前 Runtime 传给组件的 Agent Run 编号。
func runIDFromContext(ctx context.Context) string {
	runID, _ := ctx.Value(runIDContextKey{}).(string)
	return runID
}

// pushPending 把数据库中的新 Trigger 放入 TurnLoop，并按需请求安全点抢占。
func (r *EinoRuntime) pushPending(ctx context.Context, loop *adk.TurnLoop[Trigger, *schema.Message], feed InputFeed, stateMu *sync.Mutex, maxPushedSeq *int64, preempt bool) error {
	stateMu.Lock()
	afterSeq := *maxPushedSeq
	stateMu.Unlock()
	triggers, err := feed.Peek(ctx, afterSeq)
	if err != nil {
		return err
	}
	var preemptAck <-chan struct{}
	stateMu.Lock()
	for _, trigger := range triggers {
		if trigger.Seq <= *maxPushedSeq {
			continue
		}
		var accepted bool
		if preempt && preemptAck == nil {
			accepted, preemptAck = loop.Push(trigger, adk.WithPreempt[Trigger, *schema.Message](adk.AnySafePoint))
		} else {
			accepted, _ = loop.Push(trigger)
		}
		if !accepted {
			stateMu.Unlock()
			if preemptAck != nil {
				<-preemptAck
			}
			return nil
		}
		if trigger.Seq > *maxPushedSeq {
			*maxPushedSeq = trigger.Seq
		}
	}
	stateMu.Unlock()
	if preemptAck != nil {
		<-preemptAck
	}
	return nil
}

// schemaMessages 转换为 Eino 标准消息。
func schemaMessages(messages []Message) []*schema.Message {
	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case MessageRoleAssistant:
			result = append(result, schema.AssistantMessage(message.Content, nil))
		default:
			result = append(result, schema.UserMessage(message.Content))
		}
	}
	return result
}

// maxTriggerSeq 返回本轮输入中的最大 Trigger 序号。
func maxTriggerSeq(items []Trigger) int64 {
	var result int64
	for _, item := range items {
		if item.Seq > result {
			result = item.Seq
		}
	}
	return result
}

// turnPreempted 判断当前 Turn 是否已收到安全点抢占信号。
func turnPreempted(turn *adk.TurnContext[Trigger, *schema.Message]) bool {
	select {
	case <-turn.Preempted:
		return true
	default:
		return false
	}
}
