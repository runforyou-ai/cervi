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

func (m *emptyThenAnswerChatModel) Generate(_ context.Context, _ []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls <= m.emptyReplies {
		return assistantReply(""), nil
	}
	return assistantReply("重试后的回答"), nil
}

// Stream 以单个分片返回当前测试步骤的模型输出。
func (m *emptyThenAnswerChatModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return singleChunkStream(m.Generate(ctx, input, opts...))
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
				newModel: func(context.Context, ModelConfig) (model.AgenticModel, error) { return chatModel, nil },
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
