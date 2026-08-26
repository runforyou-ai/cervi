//go:build server

package conversation

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识公开访客输入的校验结果；取值为访客协议使用的小写形式。
type ValidationCode = common.FieldCode

const (
	ValidationChannelIDInvalid       ValidationCode = "channel_id_invalid"
	ValidationExternalIDInvalid      ValidationCode = "external_id_invalid"
	ValidationConversationIDInvalid  ValidationCode = "conversation_id_invalid"
	ValidationClientMessageIDInvalid ValidationCode = "client_message_id_invalid"
	ValidationBodyRequired           ValidationCode = "body_required"
	ValidationBodyTooLong            ValidationCode = "body_too_long"
	ValidationCursorInvalid          ValidationCode = "cursor_invalid"
)

const (
	// ConflictReasonIdempotencyMismatch 表示同一消息编号对应了不同写入意图。
	ConflictReasonIdempotencyMismatch = "idempotency_mismatch"
)

// WebsiteCustomerTextMessageInput 定义网站客户文本消息。
type WebsiteCustomerTextMessageInput struct {
	ChannelID       string
	ExternalID      string
	ConversationID  *string
	ClientMessageID string
	Body            string
}

// ConversationSummary 定义访客可见会话摘要。
type ConversationSummary struct {
	ID                   string
	Title                string
	Preview              string
	LastMessageAt        time.Time
	ServiceSessionID     string
	ServiceSessionStatus domain.ServiceSessionStatus
}

// Message 定义访客可见消息。
type Message struct {
	ID           string
	Author       domain.MessageAuthor
	Body         string
	OriginatedAt time.Time
	CreatedAt    time.Time
}

// ReceiveWebsiteCustomerTextMessageResult 定义网站消息写入结果。
type ReceiveWebsiteCustomerTextMessageResult struct {
	Conversation            ConversationSummary
	CreatedConversation     bool
	OpenedNewServiceSession bool
	Message                 Message
}

// MessageCursorPoint 定义消息分页稳定边界。
type MessageCursorPoint struct {
	OriginatedAt time.Time
	ID           string
}

// MessageHistoryInput 定义消息历史查询方向。
type MessageHistoryInput struct {
	ChannelID      string
	ExternalID     string
	ConversationID string
	Before         *MessageCursorPoint
	After          *MessageCursorPoint
}

// MessageHistory 定义消息历史和下一页边界。
type MessageHistory struct {
	Messages []Message
	Before   *MessageCursorPoint
	After    *MessageCursorPoint
}
