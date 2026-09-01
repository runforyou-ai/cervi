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

// GroupParticipantRole 表示群聊成员角色。
type GroupParticipantRole string

const (
	GroupParticipantRoleOwner  GroupParticipantRole = GroupParticipantRole(domain.ConversationParticipantRoleOwner)
	GroupParticipantRoleMember GroupParticipantRole = GroupParticipantRole(domain.ConversationParticipantRoleMember)
)

// ConversationMessageListInput 定义成员消息查询方向。
type ConversationMessageListInput struct {
	Before string `json:"before" query:"before"`
	After  string `json:"after" query:"after"`
}

// CustomerTextMessageInput 定义成员发送的客户会话文本消息。
type CustomerTextMessageInput struct {
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

// TransferServiceSessionInput 定义客服处理周期转交目标。
type TransferServiceSessionInput struct {
	AssigneeIdentityID string `json:"assigneeIdentityId"`
}

// CustomerServiceSession 定义客户会话最新客服处理周期。
type CustomerServiceSession struct {
	ID       string               `json:"id"`
	Status   ServiceSessionStatus `json:"status"`
	Assignee *InboxAssignee       `json:"assignee"`
	ClosedAt *time.Time           `json:"closedAt"`
}

// ConversationMessageSender 定义消息发送主体。
type ConversationMessageSender struct {
	ChatSubjectID string          `json:"chatSubjectId"`
	Kind          ChatSubjectKind `json:"kind"`
	SourceID      string          `json:"sourceId"`
	DisplayName   *string         `json:"displayName"`
}

// ConversationMessageSessionStart 定义客服处理周期开始标记。
type ConversationMessageSessionStart struct {
	Sequence  int64                `json:"sequence"`
	StartedAt time.Time            `json:"startedAt"`
	Status    ServiceSessionStatus `json:"status"`
}

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ID           string                           `json:"id"`
	Type         MessageType                      `json:"type"`
	Body         string                           `json:"body"`
	OriginatedAt time.Time                        `json:"originatedAt"`
	CreatedAt    time.Time                        `json:"createdAt"`
	Sender       *ConversationMessageSender       `json:"sender"`
	SessionStart *ConversationMessageSessionStart `json:"sessionStart"`
}

// ConversationMessageList 定义成员消息页。
type ConversationMessageList struct {
	Messages []ConversationMessage `json:"messages"`
	Before   *string               `json:"before"`
	After    *string               `json:"after"`
}

// DirectConversationInput 定义成员发起内部单聊的目标。
type DirectConversationInput struct {
	TargetIdentityID string `json:"targetIdentityId"`
}

// DirectTextMessageInput 定义成员发送的内部单聊文本消息。
type DirectTextMessageInput struct {
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

// GroupConversationInput 定义群聊标题和创建时加入的成员。
type GroupConversationInput struct {
	Title             string   `json:"title"`
	MemberIdentityIDs []string `json:"memberIdentityIds"`
}

// GroupParticipant 定义群聊当前有效成员。
type GroupParticipant struct {
	IdentityID  string               `json:"identityId"`
	DisplayName string               `json:"displayName"`
	AvatarURL   string               `json:"avatarUrl"`
	Role        GroupParticipantRole `json:"role"`
}

// GroupConversation 定义群聊资料和当前有效成员。
type GroupConversation struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Participants []GroupParticipant `json:"participants"`
}

// GroupTextMessageInput 定义成员发送的群聊文本消息。
type GroupTextMessageInput struct {
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}
