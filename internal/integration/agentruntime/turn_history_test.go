//go:build server

package agentruntime

import (
	"reflect"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestTurnHistoryRetainsOutputsAcrossSlidingInput 验证滑动历史和重复正文不会丢失或重复执行上下文。
func TestTurnHistoryRetainsOutputsAcrossSlidingInput(t *testing.T) {
	history := &turnHistory{}
	first := Message{ID: "1", Role: MessageRoleUser, Content: "继续"}
	second := Message{ID: "2", Role: MessageRoleUser, Content: "继续"}
	history.appendInput([]Message{first})
	call := schema.AssistantMessage("先查资料", []schema.ToolCall{{ID: "lookup", Function: schema.FunctionCall{Name: "search", Arguments: "{}"}}})
	call.ReasoningContent = "需要查证"
	result := schema.ToolMessage("找到规定", "lookup")
	history.appendOutput([]*schema.Message{call, result})
	history.appendInput([]Message{first, second})
	history.appendOutput([]*schema.Message{schema.AssistantMessage("已查到差旅规定", nil)})
	got := history.appendInput([]Message{second, {ID: "3", Role: MessageRoleUser, Content: "用中文"}})
	want := []*schema.Message{schema.UserMessage("继续"), call, result, schema.UserMessage("继续"), schema.AssistantMessage("已查到差旅规定", nil), schema.UserMessage("用中文")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history = %#v, want %#v", got, want)
	}
}

// TestTurnHistoryDropsUnansweredCalls 验证抢占后只保留有结果的调用，保留说明且不修改原事件。
func TestTurnHistoryDropsUnansweredCalls(t *testing.T) {
	history := &turnHistory{}
	call := schema.AssistantMessage("准备查询", []schema.ToolCall{{ID: "done"}, {ID: "skipped"}})
	call.ReasoningContent = "分析过程"
	failed := schema.ToolMessage(`{"error":"查询失败"}`, "done")
	history.appendOutput([]*schema.Message{call, failed})
	skipped := schema.AssistantMessage("稍后计算", []schema.ToolCall{{ID: "not-started"}})
	history.appendOutput([]*schema.Message{skipped, schema.AssistantMessage("", []schema.ToolCall{{ID: "empty"}})})
	got := history.messages
	if len(got) != 3 || len(got[0].ToolCalls) != 1 || got[0].ToolCalls[0].ID != "done" ||
		got[0].ReasoningContent != "分析过程" || got[1] != failed || got[2].Content != "稍后计算" || len(got[2].ToolCalls) != 0 {
		t.Fatalf("retained output = %#v", got)
	}
	if len(call.ToolCalls) != 2 || len(skipped.ToolCalls) != 1 {
		t.Fatal("history modified original events")
	}
}
