//go:build server

package agentruntime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// TestCountContextTokens 验证中文按字计、其余字符按四分之一计。
func TestCountContextTokens(t *testing.T) {
	messages := []*schema.Message{
		schema.UserMessage(strings.Repeat("字", 100)),
		schema.AssistantMessage(strings.Repeat("a", 40), nil),
	}
	tokens, err := countContextTokens(context.Background(), messages, nil)
	if err != nil || tokens != 110 {
		t.Fatalf("tokens = %d, err = %v", tokens, err)
	}
}

type repeatedToolChatModel struct {
	mu           sync.Mutex
	calls        int
	earliestSeen string
	latestSeen   string
}

// Generate 反复调用计算器，并记录最后一次规划时最早与最近的工具结果内容。
func (m *repeatedToolChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls > 4 {
		for _, message := range input {
			if message.Role != schema.Tool {
				continue
			}
			if m.earliestSeen == "" {
				m.earliestSeen = message.Content
			}
			m.latestSeen = message.Content
		}
		return schema.AssistantMessage("算完了", nil), nil
	}
	return schema.AssistantMessage("继续算", []schema.ToolCall{{
		ID: "calculator-call-" + string(rune('a'+m.calls)), Type: "function",
		Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":1}`},
	}}), nil
}

func (m *repeatedToolChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *repeatedToolChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestContextClearsOldToolResults 验证上下文超过模型窗口预算后清理较早的工具结果，并完整保留最近一轮。
func TestContextClearsOldToolResults(t *testing.T) {
	calculator, err := newCalculatorTool()
	if err != nil {
		t.Fatal(err)
	}
	chatModel := &repeatedToolChatModel{}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil },
		tools:    []tool.BaseTool{calculator},
	}
	feed := &testInputFeed{}
	feed.appendUser("一直算")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID: "clear-run", Name: "test-agent", MaxIterations: 6, MaxTurns: 2,
		Model: ModelConfig{ContextWindow: 40},
	}, feed)
	if err != nil || result.Content != "算完了" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if chatModel.earliestSeen == `{"result":2}` {
		t.Fatalf("earliest tool result was not cleared: %q", chatModel.earliestSeen)
	}
	if chatModel.latestSeen != `{"result":2}` {
		t.Fatalf("latest tool result was cleared: %q", chatModel.latestSeen)
	}
}
