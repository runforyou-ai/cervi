//go:build server

package agentruntime

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestTurnHistoryRetainsOutputsAcrossSlidingInput 验证滑动历史和重复正文下执行上下文的完整性与去重。
func TestTurnHistoryRetainsOutputsAcrossSlidingInput(t *testing.T) {
	history := &turnHistory{}
	first := Message{ID: "1", Role: MessageRoleUser, Content: "继续"}
	second := Message{ID: "2", Role: MessageRoleUser, Content: "继续"}
	history.appendInput(context.Background(), []Message{first}, mediaInput{})
	call := withReasoning(assistantReply("先查资料", &schema.FunctionToolCall{CallID: "lookup", Name: "search", Arguments: "{}"}), "需要查证")
	result := toolReply("lookup", "找到规定")
	history.appendOutput([]*schema.AgenticMessage{call, result})
	history.appendInput(context.Background(), []Message{first, second}, mediaInput{})
	history.appendOutput([]*schema.AgenticMessage{assistantReply("已查到差旅规定")})
	got := history.appendInput(context.Background(), []Message{second, {ID: "3", Role: MessageRoleUser, Content: "用中文"}}, mediaInput{})
	want := []*schema.AgenticMessage{schema.UserAgenticMessage("继续"), call, result, schema.UserAgenticMessage("继续"), assistantReply("已查到差旅规定"), schema.UserAgenticMessage("用中文")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history = %#v, want %#v", got, want)
	}
}

// TestTurnHistoryDropsEmptyAssistantOutputs 验证 assistant 历史保留有效正文或工具调用，工具历史保留空结果。
func TestTurnHistoryDropsEmptyAssistantOutputs(t *testing.T) {
	history := &turnHistory{}
	reasoning := withReasoning(assistantReply(""), "还在思考")
	skipped := withReasoning(assistantReply("", &schema.FunctionToolCall{CallID: "skipped"}), "准备调用工具")
	completed := withReasoning(assistantReply("", &schema.FunctionToolCall{CallID: "done"}), "工具已返回")
	result := toolReply("done", "")
	history.appendOutput([]*schema.AgenticMessage{assistantReply(""), reasoning, skipped, completed, result})
	if !reflect.DeepEqual(history.messages, []*schema.AgenticMessage{completed, result}) {
		t.Fatalf("retained output = %#v", history.messages)
	}
	if len(toolCalls(skipped)) != 1 || skipped.ContentBlocks[0].Reasoning.Text != "准备调用工具" {
		t.Fatal("history modified original events")
	}
}

// TestTurnHistoryDropsUnansweredCalls 验证抢占后保留已完成调用和说明，原事件保持原值。
func TestTurnHistoryDropsUnansweredCalls(t *testing.T) {
	history := &turnHistory{}
	call := withReasoning(assistantReply("准备查询", &schema.FunctionToolCall{CallID: "done"}, &schema.FunctionToolCall{CallID: "skipped"}), "分析过程")
	failed := toolReply("done", `{"error":"查询失败"}`)
	history.appendOutput([]*schema.AgenticMessage{call, failed})
	skipped := assistantReply("稍后计算", &schema.FunctionToolCall{CallID: "not-started"})
	history.appendOutput([]*schema.AgenticMessage{skipped, assistantReply("", &schema.FunctionToolCall{CallID: "empty"})})
	got := history.messages
	if len(got) != 3 || len(toolCalls(got[0])) != 1 || toolCalls(got[0])[0].CallID != "done" ||
		got[0].ContentBlocks[0].Reasoning.Text != "分析过程" || got[1] != failed || messageText(got[2]) != "稍后计算" || len(toolCalls(got[2])) != 0 {
		t.Fatalf("retained output = %#v", got)
	}
	if len(toolCalls(call)) != 2 || len(toolCalls(skipped)) != 1 {
		t.Fatal("history modified original events")
	}
}
