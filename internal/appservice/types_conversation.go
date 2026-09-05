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
	MessageTypeText   MessageType = MessageType(domain.MessageTypeText)
	MessageTypeSystem MessageType = MessageType(domain.MessageTypeSystem)
)

// ConversationStatus 表示会话生命周期状态。
type ConversationStatus string

const (
	ConversationStatusActive   ConversationStatus = ConversationStatus(domain.ConversationStatusActive)
	ConversationStatusArchived ConversationStatus = ConversationStatus(domain.ConversationStatusArchived)
)

// ConversationSystemEventType 表示会话系统事件类型。
type ConversationSystemEventType string

const (
	ConversationSystemEventGroupRenamed          ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupRenamed)
	ConversationSystemEventGroupMembersAdded     ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupMembersAdded)
	ConversationSystemEventGroupMemberRemoved    ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupMemberRemoved)
	ConversationSystemEventGroupMemberLeft       ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupMemberLeft)
	ConversationSystemEventGroupOwnerTransferred ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupOwnerTransferred)
	ConversationSystemEventGroupDissolved        ConversationSystemEventType = ConversationSystemEventType(domain.ConversationSystemEventGroupDissolved)
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

// ConversationMessageReference 定义引用消息的一层摘要。
type ConversationMessageReference struct {
	ID     string                     `json:"id"`
	Body   string                     `json:"body"`
	Sender *ConversationMessageSender `json:"sender"`
}

// ConversationMessageMention 定义消息提醒的聊天主体。
type ConversationMessageMention struct {
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

// ConversationSystemEventParticipant 定义系统事件中的成员快照。
type ConversationSystemEventParticipant struct {
	IdentityID  string `json:"identityId"`
	DisplayName string `json:"displayName"`
}

// ConversationSystemEvent 定义成员可见的系统事件。
type ConversationSystemEvent struct {
	Type          ConversationSystemEventType          `json:"type"`
	Actor         ConversationSystemEventParticipant   `json:"actor"`
	Targets       []ConversationSystemEventParticipant `json:"targets"`
	PreviousTitle *string                              `json:"previousTitle"`
	Title         *string                              `json:"title"`
}

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ID           string                           `json:"id"`
	Type         MessageType                      `json:"type"`
	Body         string                           `json:"body"`
	OriginatedAt time.Time                        `json:"originatedAt"`
	SourceOrder  int64                            `json:"sourceOrder"`
	CreatedAt    time.Time                        `json:"createdAt"`
	Sender       *ConversationMessageSender       `json:"sender"`
	SessionStart *ConversationMessageSessionStart `json:"sessionStart"`
	SystemEvent  *ConversationSystemEvent         `json:"systemEvent"`
	ReplyTo      *ConversationMessageReference    `json:"replyTo"`
	Mentions     []ConversationMessageMention     `json:"mentions"`
	MentionAll   bool                             `json:"mentionAll"`
}

// ConversationMessageList 定义成员消息页。
type ConversationMessageList struct {
	Messages []ConversationMessage `json:"messages"`
	Before   *string               `json:"before"`
	After    *string               `json:"after"`
}

// MarkConversationReadInput 定义用户确认已读的消息水位。
type MarkConversationReadInput struct {
	LastReadMessageID string `json:"lastReadMessageId"`
}

// ConversationReadState 定义用户会话的已读水位。
type ConversationReadState struct {
	LastReadMessageID string    `json:"lastReadMessageId"`
	LastReadAt        time.Time `json:"lastReadAt"`
}

// FirstDirectTextMessageInput 定义成员向目标身份发送的首条单聊消息。
type FirstDirectTextMessageInput struct {
	TargetIdentityID string `json:"targetIdentityId"`
	ClientMessageID  string `json:"clientMessageId"`
	Body             string `json:"body"`
}

// FirstDirectTextMessageResult 定义首条单聊消息及其确定的长期会话。
type FirstDirectTextMessageResult struct {
	Conversation InboxConversation   `json:"conversation"`
	Message      ConversationMessage `json:"message"`
}

// DirectConversationLookup 定义按目标身份查找单聊的结果。
type DirectConversationLookup struct {
	Conversation *InboxConversation `json:"conversation"`
}

// DirectTextMessageInput 定义成员发送的内部单聊文本消息。
type DirectTextMessageInput struct {
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

// GroupConversationInput 定义群聊资料和创建时加入的成员。
type GroupConversationInput struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	ImageFileID       string   `json:"imageFileId"`
	MemberIdentityIDs []string `json:"memberIdentityIds"`
}

// GroupConversationProfileInput 定义群聊资料修改参数。
type GroupConversationProfileInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// ImageFileID 为 nil 时保留当前图片，非 nil 时关联新图片。
	ImageFileID *string `json:"imageFileId"`
}

// GroupConversationMembersInput 定义群聊批量增员参数。
type GroupConversationMembersInput struct {
	MemberIdentityIDs []string `json:"memberIdentityIds"`
}

// GroupConversationMemberInput 定义群聊单个成员操作参数。
type GroupConversationMemberInput struct {
	MemberIdentityID string `json:"memberIdentityId"`
}

// GroupConversationOwnerInput 定义群主转让参数。
type GroupConversationOwnerInput struct {
	OwnerIdentityID string `json:"ownerIdentityId"`
}

// GroupConversationLeaveInput 定义当前成员退出群聊参数。
type GroupConversationLeaveInput struct {
	SuccessorIdentityID string `json:"successorIdentityId"`
}

// GroupParticipant 定义群聊当前有效成员。
type GroupParticipant struct {
	ChatSubjectID string               `json:"chatSubjectId"`
	IdentityID    string               `json:"identityId"`
	DisplayName   string               `json:"displayName"`
	AvatarURL     string               `json:"avatarUrl"`
	Role          GroupParticipantRole `json:"role"`
}

// GroupConversation 定义群聊资料和当前有效成员。
type GroupConversation struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	ImageURL     string             `json:"imageUrl"`
	Status       ConversationStatus `json:"status"`
	CreatedAt    time.Time          `json:"createdAt"`
	Participants []GroupParticipant `json:"participants"`
}

// GroupTextMessageInput 定义成员发送的群聊文本消息。
type GroupTextMessageInput struct {
	ClientMessageID   string   `json:"clientMessageId"`
	Body              string   `json:"body"`
	ReplyToMessageID  string   `json:"replyToMessageId"`
	MentionSubjectIDs []string `json:"mentionSubjectIds"`
	MentionAll        bool     `json:"mentionAll"`
}

// ConversationNotificationSettingsInput 定义当前用户的会话提醒设置。
type ConversationNotificationSettingsInput struct {
	Muted bool `json:"muted"`
}

// ConversationNotificationSettings 定义当前用户保存后的会话提醒设置。
type ConversationNotificationSettings struct {
	Muted bool `json:"muted"`
}
