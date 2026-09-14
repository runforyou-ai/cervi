//go:build server

package agentruntime

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// turnHistory 按消费顺序保留当前运行的会话消息和完整工具交互。
type turnHistory struct {
	messages   []*schema.Message
	seen       map[string]struct{}
	mediaCount int
	mediaBytes int64
}

// appendInput 按消息编号去重，把新消息追加在上一轮中间结果之后；模型支持的附件由新到旧在预算内随消息直传。
func (h *turnHistory) appendInput(ctx context.Context, messages []Message, media mediaInput) []*schema.Message {
	if h.seen == nil {
		h.seen = make(map[string]struct{})
	}
	fresh := make([]Message, 0, len(messages))
	for _, message := range messages {
		if _, exists := h.seen[message.ID]; exists {
			continue
		}
		h.seen[message.ID] = struct{}{}
		fresh = append(fresh, message)
	}
	// 由新到旧读取模型支持格式的用户附件，读取成功才计入数量和字节预算，其余附件只保留正文中的链接。
	inline := make(map[string]*schema.Message)
	for i := len(fresh) - 1; i >= 0; i-- {
		attachment := fresh[i].Media
		if attachment == nil || fresh[i].Role != MessageRoleUser || h.mediaCount >= media.maxCount ||
			attachment.ByteSize > maxMediaBytes || h.mediaBytes+attachment.ByteSize > maxRunMediaBytes {
			continue
		}
		modality, supported := inlineMediaTypes[attachment.MIMEType]
		if !supported || !media.modalities[modality] {
			continue
		}
		direct, read := mediaUserMessage(ctx, fresh[i], modality, media.read)
		if !read {
			continue
		}
		inline[fresh[i].ID] = direct
		h.mediaCount++
		h.mediaBytes += attachment.ByteSize
	}
	for _, message := range fresh {
		direct, inlined := inline[message.ID]
		switch {
		case message.Role == MessageRoleAssistant:
			h.messages = append(h.messages, schema.AssistantMessage(message.Content, nil))
		case inlined:
			h.messages = append(h.messages, direct)
		default:
			h.messages = append(h.messages, schema.UserMessage(message.Content))
		}
	}
	return append([]*schema.Message(nil), h.messages...)
}

// appendOutput 保留有效回复和完整工具交互，移除抢占跳过的调用及其空消息。
func (h *turnHistory) appendOutput(messages []*schema.Message) {
	completed := make(map[string]struct{})
	for _, message := range messages {
		if message.Role == schema.Tool {
			completed[message.ToolCallID] = struct{}{}
		}
	}
	for _, message := range messages {
		if message.Role != schema.Assistant {
			h.messages = append(h.messages, message)
			continue
		}
		// 复制 SDK 事件中的工具调用列表。
		retained := *message
		retained.ToolCalls = nil
		for _, call := range message.ToolCalls {
			if _, ok := completed[call.ID]; ok {
				retained.ToolCalls = append(retained.ToolCalls, call)
			}
		}
		// 过程记录器保存思考内容，assistant 历史消息由正文和工具调用构成。
		if len(retained.ToolCalls) > 0 || retained.Content != "" {
			h.messages = append(h.messages, &retained)
		}
	}
}
