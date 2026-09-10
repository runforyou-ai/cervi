//go:build server

package agentruntime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type emptyThenAnswerChatModel struct {
	mu           sync.Mutex
	calls        int
	emptyReplies int
}

func (m *emptyThenAnswerChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls <= m.emptyReplies {
		return schema.AssistantMessage("", nil), nil
	}
	return schema.AssistantMessage("重试后的回答", nil), nil
}

func (m *emptyThenAnswerChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *emptyThenAnswerChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestEmptyFinalResponseRetryIsBounded 验证空正文按有界次数重新执行本次输入。
func TestEmptyFinalResponseRetryIsBounded(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		emptyReplies int
		wantCalls    int
		wantErr      bool
	}{
		{"空正文后重试成功", emptyResponseRetryLimit, emptyResponseRetryLimit + 1, false},
		{"连续空正文达到上限", emptyResponseRetryLimit + 1, emptyResponseRetryLimit + 1, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			chatModel := &emptyThenAnswerChatModel{emptyReplies: scenario.emptyReplies}
			runtime := &EinoRuntime{
				newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil },
			}
			feed := &testInputFeed{}
			feed.appendUser("群里的问题")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := runtime.Run(ctx, RunRequest{RunID: "test-run-id", Name: "群协作助手"}, feed)
			if scenario.wantErr && err == nil {
				t.Fatalf("期望达到上限后失败，实际结果 = %#v", result)
			}
			if !scenario.wantErr && (err != nil || result.Content != "重试后的回答") {
				t.Fatalf("结果 = %#v，error = %v", result, err)
			}
			if chatModel.calls != scenario.wantCalls {
				t.Fatalf("模型调用次数 = %d，期望 %d", chatModel.calls, scenario.wantCalls)
			}
		})
	}
}
