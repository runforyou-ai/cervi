//go:build server

package agentruntime

import "github.com/cloudwego/eino/schema"

// turnHistory 按消费顺序保留当前运行的会话消息和完整工具交互。
type turnHistory struct {
	messages []*schema.Message
	seen     map[string]struct{}
}

// appendInput 按消息编号去重，把新消息追加在上一轮中间结果之后。
func (h *turnHistory) appendInput(messages []Message) []*schema.Message {
	if h.seen == nil {
		h.seen = make(map[string]struct{})
	}
	for _, message := range messages {
		if _, exists := h.seen[message.ID]; exists {
			continue
		}
		h.seen[message.ID] = struct{}{}
		if message.Role == MessageRoleAssistant {
			h.messages = append(h.messages, schema.AssistantMessage(message.Content, nil))
		} else {
			h.messages = append(h.messages, schema.UserMessage(message.Content))
		}
	}
	return append([]*schema.Message(nil), h.messages...)
}

// appendOutput 保留已返回的标准消息，移除安全点抢占跳过的工具调用。
func (h *turnHistory) appendOutput(messages []*schema.Message) {
	completed := make(map[string]struct{})
	for _, message := range messages {
		if message.Role == schema.Tool {
			completed[message.ToolCallID] = struct{}{}
		}
	}
	for _, message := range messages {
		if len(message.ToolCalls) == 0 {
			h.messages = append(h.messages, message)
			continue
		}
		// 复制调用列表，不改变 SDK 事件持有的原始消息。
		retained := *message
		retained.ToolCalls = nil
		for _, call := range message.ToolCalls {
			if _, ok := completed[call.ID]; ok {
				retained.ToolCalls = append(retained.ToolCalls, call)
			}
		}
		if len(retained.ToolCalls) > 0 || retained.Content != "" || retained.ReasoningContent != "" {
			h.messages = append(h.messages, &retained)
		}
	}
}
