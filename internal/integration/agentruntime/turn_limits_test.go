//go:build server

package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestRunFollowUpsDoNotConsumeIterationBudget 验证连续补充使用独立迭代预算并保留历史中间正文。
func TestRunFollowUpsDoNotConsumeIterationBudget(t *testing.T) {
	for _, maxTurns := range []int{0, 3} {
		t.Run(fmt.Sprintf("max-turns-%d", maxTurns), func(t *testing.T) {
			feed := &testInputFeed{}
			feed.appendUser("开始")
			calls := 0
			chatModel := &processChatModel{generate: func(_ context.Context, messages []*schema.Message) (*schema.Message, error) {
				calls++
				var assistantCount int
				for _, message := range messages {
					if message.Role == schema.Assistant {
						assistantCount++
					}
				}
				if assistantCount != calls-1 {
					return nil, fmt.Errorf("previous assistant messages = %d, want %d", assistantCount, calls-1)
				}
				if calls < 12 {
					feed.appendUser("继续")
				}
				return schema.AssistantMessage(fmt.Sprintf("回答 %d", calls), nil), nil
			}}
			runtime := &EinoRuntime{newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := runtime.Run(ctx, RunRequest{Name: "test", MaxIterations: 1, MaxTurns: maxTurns}, feed)
			if maxTurns > 0 {
				if err == nil || !strings.Contains(err.Error(), "agent turn limit 3 exceeded") || calls != 3 {
					t.Fatalf("limited calls = %d, error = %v", calls, err)
				}
			} else if err != nil || calls != 12 || result.EndSeq != 12 || result.Content != "回答 12" {
				t.Fatalf("result = %#v, calls = %d, error = %v", result, calls, err)
			}
		})
	}
}

// TestRunRetainsToolsAcrossRepeatedPreemption 验证连续工具抢占的结果保留及各轮输入的独立迭代预算。
func TestRunRetainsToolsAcrossRepeatedPreemption(t *testing.T) {
	runtime, err := New()
	if err != nil {
		t.Fatal(err)
	}
	feed := &testInputFeed{}
	feed.appendUser("开始计算")
	calls := 0
	chatModel := &processChatModel{generate: func(_ context.Context, messages []*schema.Message) (*schema.Message, error) {
		calls++
		var results int
		for _, message := range messages {
			if message.Role == schema.Tool {
				results++
				if message.ToolCallID != fmt.Sprintf("call-%d", results) || message.Content != `{"result":3}` {
					return nil, fmt.Errorf("unexpected tool result: %#v", message)
				}
			}
		}
		if results != calls-1 || messages[len(messages)-1].Role != schema.User {
			return nil, fmt.Errorf("tool results = %d for model call %d, last role = %s", results, calls, messages[len(messages)-1].Role)
		}
		if calls == 4 {
			return schema.AssistantMessage("完成", nil), nil
		}
		return schema.AssistantMessage("正在计算", []schema.ToolCall{{ID: fmt.Sprintf("call-%d", calls), Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2}`}}}), nil
	}}
	runtime.newModel = func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }
	seen := make(map[string]bool)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{Name: "test", MaxIterations: 2, OnProgress: func(progress Progress) {
		for _, block := range progress.Blocks {
			if call := block.Payload.ToolCall; call != nil && call.Status == domain.AgentToolCallRunning && !seen[call.CallID] {
				seen[call.CallID] = true
				feed.appendUser("再补充一项")
			}
		}
	}}, feed)
	if err != nil || result.Content != "完成" || result.EndSeq != 4 || calls != 4 {
		t.Fatalf("result = %#v, calls = %d, error = %v", result, calls, err)
	}
}

// TestRunIterationLimitStillStopsToolLoop 验证单轮持续调用工具仍受迭代上限约束。
func TestRunIterationLimitStillStopsToolLoop(t *testing.T) {
	runtime, err := New()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	chatModel := &processChatModel{generate: func(context.Context, []*schema.Message) (*schema.Message, error) {
		calls++
		return schema.AssistantMessage("", []schema.ToolCall{{ID: fmt.Sprintf("call-%d", calls), Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2}`}}}), nil
	}}
	runtime.newModel = func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }
	feed := &testInputFeed{}
	feed.appendUser("计算")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = runtime.Run(ctx, RunRequest{Name: "test", MaxIterations: 2, MaxTurns: 20}, feed)
	if err == nil || calls > 2 || calls == 0 || ctx.Err() != nil {
		t.Fatalf("calls = %d, error = %v, context error = %v", calls, err, ctx.Err())
	}
}
