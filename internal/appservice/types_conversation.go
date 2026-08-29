package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// ChatSubjectKind 表示聊天主体类型。
type ChatSubjectKind string

const (
	ChatSubjectKindOrganizationIdentity ChatSubjectKind = ChatSubjectKind(domain.ChatSubjectKindOrganizationIdentity)
	ChatSubjectKindContact              ChatSubjectKind = ChatSubjectKind(domain.ChatSubjectKindContact)
)

// MessageType 表示会话消息类型。
type MessageType string

const (
	MessageTypeText MessageType = MessageType(domain.MessageTypeText)
)

// ConversationMessageListInput 定义成员消息查询方向。
type ConversationMessageListInput struct {
	Before string `json:"before" query:"before"`
	After  string `json:"after" query:"after"`
}

// ConversationMessageSender 定义消息发送主体。
type ConversationMessageSender struct {
	ChatSubjectID string          `json:"chatSubjectId"`
	Kind          ChatSubjectKind `json:"kind"`
	DisplayName   *string         `json:"displayName"`
}

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ID           string                     `json:"id"`
	Type         MessageType                `json:"type"`
	Body         string                     `json:"body"`
	OriginatedAt time.Time                  `json:"originatedAt"`
	CreatedAt    time.Time                  `json:"createdAt"`
	Sender       *ConversationMessageSender `json:"sender"`
}

// ConversationMessageList 定义成员消息页。
type ConversationMessageList struct {
	Messages []ConversationMessage `json:"messages"`
	Before   *string               `json:"before"`
	After    *string               `json:"after"`
}
