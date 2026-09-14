//go:build server

package agentruntime

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// TestToolArgumentsNormalizerFillsEmptyArguments 验证空工具参数补为空 JSON 对象且不修改原消息。
func TestToolArgumentsNormalizerFillsEmptyArguments(t *testing.T) {
	original := schema.AssistantMessage("", []schema.ToolCall{
		{ID: "empty-call", Type: "function", Function: schema.FunctionCall{Name: "list_items"}},
		{ID: "filled-call", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"left":1}`}},
	})
	state := &adk.ChatModelAgentState{Messages: []*schema.Message{schema.UserMessage("问题"), original}}
	_, state, err := (&toolArgumentsNormalizer{}).BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := state.Messages[1].ToolCalls
	if calls[0].Function.Arguments != "{}" || calls[1].Function.Arguments != `{"left":1}` {
		t.Fatalf("tool calls = %#v", calls)
	}
	if original.ToolCalls[0].Function.Arguments != "" {
		t.Fatalf("原消息被修改：%#v", original.ToolCalls)
	}
}
