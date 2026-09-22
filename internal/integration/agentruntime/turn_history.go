package agentruntime

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// turnHistory 按消费顺序保留当前运行的会话消息和完整工具交互。
type turnHistory struct {
	messages   []*schema.AgenticMessage
	seen       map[string]struct{}
	mediaCount int
	mediaBytes int64
}

// appendInput 按消息编号与修订去重，把新消息追加在上一轮中间结果之后；模型支持的附件由新到旧在预算内随消息直传。
func (h *turnHistory) appendInput(ctx context.Context, messages []Message, media mediaInput) []*schema.AgenticMessage {
	if h.seen == nil {
		h.seen = make(map[string]struct{})
	}
	fresh := make([]Message, 0, len(messages))
	for _, message := range messages {
		key := message.ID + "@" + message.Revision
		if _, exists := h.seen[key]; exists {
			continue
		}
		h.seen[key] = struct{}{}
		fresh = append(fresh, message)
	}
	// 由新到旧读取模型支持格式的用户附件，读取成功才计入数量和字节预算，其余附件只保留正文中的链接。
	inline := make(map[string]*schema.AgenticMessage)
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
			h.messages = append(h.messages, &schema.AgenticMessage{
				Role:          schema.AgenticRoleTypeAssistant,
				ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.AssistantGenText{Text: message.Content})},
			})
		case inlined:
			h.messages = append(h.messages, direct)
		default:
			h.messages = append(h.messages, schema.UserAgenticMessage(message.Content))
		}
	}
	return append([]*schema.AgenticMessage(nil), h.messages...)
}

// appendOutput 保留有效回复和完整工具交互，移除抢占跳过的调用及其空消息。
func (h *turnHistory) appendOutput(messages []*schema.AgenticMessage) {
	completed := make(map[string]struct{})
	for _, message := range messages {
		for _, block := range message.ContentBlocks {
			if block.Type == schema.ContentBlockTypeFunctionToolResult {
				completed[block.FunctionToolResult.CallID] = struct{}{}
			}
		}
	}
	for _, message := range messages {
		if message.Role != schema.AgenticRoleTypeAssistant {
			h.messages = append(h.messages, message)
			continue
		}
		// 只保留已返回结果的工具调用块。
		retained := *message
		retained.ContentBlocks = nil
		hasText, hasCall := false, false
		for _, block := range message.ContentBlocks {
			switch block.Type {
			case schema.ContentBlockTypeFunctionToolCall:
				if _, ok := completed[block.FunctionToolCall.CallID]; !ok {
					continue
				}
				hasCall = true
			case schema.ContentBlockTypeAssistantGenText:
				hasText = hasText || block.AssistantGenText.Text != ""
			}
			retained.ContentBlocks = append(retained.ContentBlocks, block)
		}
		// 含正文或已完成工具调用的 assistant 消息才写入历史。
		if hasCall || hasText {
			h.messages = append(h.messages, &retained)
		}
	}
}
