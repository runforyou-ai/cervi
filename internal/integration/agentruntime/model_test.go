//go:build server

package agentruntime

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TestToolArgumentsNormalizerFillsEmptyArguments 验证空工具参数补为空 JSON 对象且不修改原消息。
func TestToolArgumentsNormalizerFillsEmptyArguments(t *testing.T) {
	original := assistantReply("",
		&schema.FunctionToolCall{CallID: "empty-call", Name: "list_items"},
		&schema.FunctionToolCall{CallID: "filled-call", Name: "calculator", Arguments: `{"left":1}`},
	)
	state := &adk.TypedChatModelAgentState[*schema.AgenticMessage]{Messages: []*schema.AgenticMessage{schema.UserAgenticMessage("问题"), original}}
	_, state, err := (&toolArgumentsNormalizer{}).BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := toolCalls(state.Messages[1])
	if calls[0].Arguments != "{}" || calls[1].Arguments != `{"left":1}` {
		t.Fatalf("tool calls = %#v", calls)
	}
	if toolCalls(original)[0].Arguments != "" {
		t.Fatalf("原消息被修改：%#v", toolCalls(original))
	}
}

// optionCountingModel 记录每次调用收到的选项数量。
type optionCountingModel struct {
	optionCounts []int
}

// Generate 记录选项数量并返回空回复。
func (m *optionCountingModel) Generate(_ context.Context, _ []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	m.optionCounts = append(m.optionCounts, len(opts))
	return assistantReply("ok"), nil
}

// Stream 记录选项数量并以单个分片返回空回复。
func (m *optionCountingModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return singleChunkStream(m.Generate(ctx, input, opts...))
}

// TestRequestOptionsModelAppendsFixedOptions 验证固定请求选项随每次调用附加且不累积。
func TestRequestOptionsModelAppendsFixedOptions(t *testing.T) {
	inner := &optionCountingModel{}
	wrapped := &requestOptionsModel{AgenticModel: inner, options: []model.Option{model.WithTemperature(0)}}
	for range 2 {
		if _, err := wrapped.Generate(context.Background(), nil, model.WithMaxTokens(10)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wrapped.Stream(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(inner.optionCounts) != 3 || inner.optionCounts[0] != 2 || inner.optionCounts[1] != 2 || inner.optionCounts[2] != 1 {
		t.Fatalf("option counts = %v", inner.optionCounts)
	}
}
