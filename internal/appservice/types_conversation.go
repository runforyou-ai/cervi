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
	MessageTypeText           MessageType = MessageType(domain.MessageTypeText)
	MessageTypeSystem         MessageType = MessageType(domain.MessageTypeSystem)
	MessageTypeAgentError     MessageType = MessageType(domain.MessageTypeAgentError)
	MessageTypeAgentCancelled MessageType = MessageType(domain.MessageTypeAgentCancelled)
	MessageTypeAttachment     MessageType = MessageType(domain.MessageTypeAttachment)
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
	ReplyToMessageID string `json:"replyToMessageId"`
	ClientMessageID  string `json:"clientMessageId"`
	Body             string `json:"body"`
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
	ChatSubjectID string                    `json:"chatSubjectId"`
	Kind          ChatSubjectKind           `json:"kind"`
	SourceID      string                    `json:"sourceId"`
	DisplayName   *string                   `json:"displayName"`
	AvatarURL     string                    `json:"avatarUrl"`
	IdentityType  *OrganizationIdentityType `json:"identityType"`
}

// ConversationMessageReference 定义引用消息的一层摘要。
type ConversationMessageReference struct {
	// ExternalSenderName 仅未关联本地原消息的引用返回平台名称，此时 ID 为空。
	ExternalSenderName string                     `json:"externalSenderName,omitempty"`
	Type               MessageType                `json:"type"`
	Deleted            bool                       `json:"deleted"`
	ID                 string                     `json:"id"`
	Body               string                     `json:"body"`
	Sender             *ConversationMessageSender `json:"sender"`
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
	CanReply bool `json:"canReply"`
	// ClientMessageID 仅向原发送身份返回。
	ClientMessageID *string                          `json:"clientMessageId"`
	Attachment      *MessageAttachment               `json:"attachment"`
	AgentProcess    *ConversationAgentProcess        `json:"agentProcess"`
	MessageSeq      string                           `json:"messageSeq"`
	ID              string                           `json:"id"`
	Type            MessageType                      `json:"type"`
	Body            string                           `json:"body"`
	OriginatedAt    time.Time                        `json:"originatedAt"`
	SourceOrder     int64                            `json:"sourceOrder"`
	CreatedAt       time.Time                        `json:"createdAt"`
	Sender          *ConversationMessageSender       `json:"sender"`
	SessionStart    *ConversationMessageSessionStart `json:"sessionStart"`
	SystemEvent     *ConversationSystemEvent         `json:"systemEvent"`
	ReplyTo         *ConversationMessageReference    `json:"replyTo"`
	Mentions        []ConversationMessageMention     `json:"mentions"`
	MentionAll      bool                             `json:"mentionAll"`
}

// ConversationMessageList 定义成员消息页。
type ConversationMessageList struct {
	LatestAgentRun *ConversationAgentRun `json:"latestAgentRun"`
	HasEarlier     bool                  `json:"hasEarlier"`
	HasLater       bool                  `json:"hasLater"`
	Messages       []ConversationMessage `json:"messages"`
	Before         *string               `json:"before"`
	After          *string               `json:"after"`
}

// MarkConversationReadInput 定义用户确认已读的消息水位。
type MarkConversationReadInput struct {
	LastReadMessageID string `json:"lastReadMessageId"`
	ClearUnreadMark   bool   `json:"clearUnreadMark"`
}

// ConversationReadState 定义用户会话的已读水位。
type ConversationReadState struct {
	ReadSeq           string    `json:"readSeq"`
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
	ClientMessageID  string `json:"clientMessageId"`
	Body             string `json:"body"`
	ReplyToMessageID string `json:"replyToMessageId"`
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

// GroupParticipant 定义群聊当前有效成员。
type GroupParticipant struct {
	IdentityType  OrganizationIdentityType `json:"identityType"`
	ChatSubjectID string                   `json:"chatSubjectId"`
	IdentityID    string                   `json:"identityId"`
	DisplayName   string                   `json:"displayName"`
	AvatarURL     string                   `json:"avatarUrl"`
	Role          GroupParticipantRole     `json:"role"`
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
	Muted        bool               `json:"muted"`
}

// GroupTextMessageInput 定义成员发送的群聊文本消息。
type GroupTextMessageInput struct {
	ClientMessageID   string   `json:"clientMessageId"`
	Body              string   `json:"body"`
	ReplyToMessageID  string   `json:"replyToMessageId"`
	MentionSubjectIDs []string `json:"mentionSubjectIds"`
	MentionAll        bool     `json:"mentionAll"`
}

// ConversationUnreadMarkInput 定义当前用户的独立未读标记。
type ConversationUnreadMarkInput struct {
	MarkedUnread bool `json:"markedUnread"`
}

// ConversationNotificationSettingsInput 定义当前用户的会话提醒设置。
type ConversationNotificationSettingsInput struct {
	Muted bool `json:"muted"`
}

// ConversationNotificationSettings 定义当前用户保存后的会话提醒设置。
type ConversationNotificationSettings struct {
	Muted bool `json:"muted"`
}

// ConversationNavigationState 定义群聊可见尾端和提及查看进度。
type ConversationNavigationState struct {
	PendingMentionCount      int     `json:"pendingMentionCount"`
	ReviewedThroughMessageID *string `json:"reviewedThroughMessageId"`
	ReviewedThroughSequence  string  `json:"reviewedThroughSequence"`
	LatestMessageID          *string `json:"latestMessageId"`
	LatestSequence           string  `json:"latestSequence"`
}

// PendingConversationMentions 定义本轮固定提及目标及序号上界。
type PendingConversationMentions struct {
	MessageIDs         []string `json:"messageIds"`
	LastTargetSequence *string  `json:"lastTargetSequence"`
}

// MarkConversationMentionReviewedInput 定义待确认的提及目标。
type MarkConversationMentionReviewedInput struct {
	MessageID string `json:"messageId"`
}

// ConversationMentionReviewOutcome 定义提及确认结果。
type ConversationMentionReviewOutcome string

const (
	ConversationMentionReviewed        ConversationMentionReviewOutcome = "reviewed"
	ConversationMentionAlreadyReviewed ConversationMentionReviewOutcome = "alreadyReviewed"
	ConversationMentionUnavailable     ConversationMentionReviewOutcome = "unavailable"
)

// ConversationMentionReview 定义连续确认后的服务端水位。
type ConversationMentionReview struct {
	ReviewedThroughMessageID *string                          `json:"reviewedThroughMessageId"`
	ReviewedThroughSequence  string                           `json:"reviewedThroughSequence"`
	Outcome                  ConversationMentionReviewOutcome `json:"outcome"`
}

// FirstAgentTextMessageInput 定义 AI 草稿的稳定编号、目标和首条消息。
type FirstAgentTextMessageInput struct {
	ConversationID  string `json:"conversationId"`
	AgentIdentityID string `json:"agentIdentityId"`
	ClientMessageID string `json:"clientMessageId"`
	Body            string `json:"body"`
}

// FirstAgentTextMessageResult 定义首次发送确认的 AI 会话和消息。
type FirstAgentTextMessageResult struct {
	Conversation InboxConversation   `json:"conversation"`
	Message      ConversationMessage `json:"message"`
}

// AgentTextMessageInput 定义发给 AI 会话的成员消息。
type AgentTextMessageInput struct {
	ClientMessageID  string `json:"clientMessageId"`
	Body             string `json:"body"`
	ReplyToMessageID string `json:"replyToMessageId"`
}

// AttachmentMessageInput 定义发往已有会话或单聊目标的附件消息。
type AttachmentMessageInput struct {
	ConversationID   string `json:"conversationId"`
	TargetIdentityID string `json:"targetIdentityId"`
	ClientMessageID  string `json:"clientMessageId"`
	FileID           string `json:"fileId"`
}

// AttachmentMessageResult 定义附件消息及首发时创建的单聊。
type AttachmentMessageResult struct {
	ConversationID string              `json:"conversationId"`
	Conversation   *InboxConversation  `json:"conversation"`
	Message        ConversationMessage `json:"message"`
}

// AttachmentUploadStatus 定义附件消息的上传状态。
type AttachmentUploadStatus string

const (
	AttachmentUploading AttachmentUploadStatus = AttachmentUploadStatus(domain.AttachmentUploading)
	AttachmentReady     AttachmentUploadStatus = AttachmentUploadStatus(domain.AttachmentReady)
	AttachmentFailed    AttachmentUploadStatus = AttachmentUploadStatus(domain.AttachmentFailed)
	AttachmentCancelled AttachmentUploadStatus = AttachmentUploadStatus(domain.AttachmentCancelled)
)

// MessageAttachment 定义消息的文件信息与上传状态。
type MessageAttachment struct {
	File
	UploadStatus AttachmentUploadStatus `json:"uploadStatus"`
	ImageWidth   int                    `json:"imageWidth"`
	ImageHeight  int                    `json:"imageHeight"`
}

// AttachmentBatchItem 定义一条附件消息的文件、说明和图片展示尺寸。
type AttachmentBatchItem struct {
	Body            string `json:"body"`
	FileName        string `json:"fileName"`
	ContentType     string `json:"contentType"`
	ByteSize        int64  `json:"byteSize"`
	ClientMessageID string `json:"clientMessageId"`
	ImageWidth      int    `json:"imageWidth"`
	ImageHeight     int    `json:"imageHeight"`
}

// AttachmentBatchInput 定义按顺序发送的附件消息。
type AttachmentBatchInput struct {
	ConversationID   string                `json:"conversationId"`
	TargetIdentityID string                `json:"targetIdentityId"`
	Attachments      []AttachmentBatchItem `json:"attachments"`
}

// AttachmentBatchResult 返回已保存的有序消息。
type AttachmentBatchResult struct {
	ConversationID string                `json:"conversationId"`
	Conversation   *InboxConversation    `json:"conversation"`
	Messages       []ConversationMessage `json:"messages"`
}

// AttachmentUploadUpdate 定义当前客户端上传的状态变更或活跃确认。
type AttachmentUploadUpdate struct {
	FileIDs []string               `json:"fileIds"`
	Status  AttachmentUploadStatus `json:"status"`
}

// AttachmentStateListInput 指定当前消息窗口需要刷新的附件。
type AttachmentStateListInput struct {
	MessageIDs string `json:"messageIds" query:"messageIds"`
}

// AttachmentMessageState 定义消息窗口内附件的可见性和上传状态。
type AttachmentMessageState struct {
	MessageID  string            `json:"messageId"`
	Attachment MessageAttachment `json:"attachment"`
	Deleted    bool              `json:"deleted"`
}

// AttachmentStateList 返回当前窗口已有附件的状态。
type AttachmentStateList struct {
	States []AttachmentMessageState `json:"states"`
}

// ConversationMessageReferenceListInput 定义窗口内需要刷新引用状态的消息编号。
type ConversationMessageReferenceListInput struct {
	MessageIDs string `json:"messageIds" query:"messageIds"`
}

// ConversationMessageReferenceState 定义一条消息的最新引用与回复可用状态。
type ConversationMessageReferenceState struct {
	MessageID string                        `json:"messageId"`
	CanReply  bool                          `json:"canReply"`
	ReplyTo   *ConversationMessageReference `json:"replyTo"`
}

// ConversationMessageReferenceList 返回窗口内的引用状态。
type ConversationMessageReferenceList struct {
	States []ConversationMessageReferenceState `json:"states"`
}
