//go:build server

package conversation

import (
	"time"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// ValidationCode 标识会话业务输入的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationChannelIDInvalid         ValidationCode = "channel_id_invalid"
	ValidationExternalIDInvalid        ValidationCode = "external_id_invalid"
	ValidationConversationIDInvalid    ValidationCode = "conversation_id_invalid"
	ValidationTargetIdentityIDInvalid  ValidationCode = "target_identity_id_invalid"
	ValidationGroupTitleRequired       ValidationCode = "group_title_required"
	ValidationGroupTitleTooLong        ValidationCode = "group_title_too_long"
	ValidationGroupDescriptionTooLong  ValidationCode = "group_description_too_long"
	ValidationGroupImageFileIDInvalid  ValidationCode = "group_image_file_id_invalid"
	ValidationGroupMembersRequired     ValidationCode = "group_members_required"
	ValidationGroupMembersTooMany      ValidationCode = "group_members_too_many"
	ValidationGroupMemberIDsInvalid    ValidationCode = "group_member_ids_invalid"
	ValidationGroupMemberIDInvalid     ValidationCode = "group_member_id_invalid"
	ValidationGroupOwnerIDInvalid      ValidationCode = "group_owner_id_invalid"
	ValidationClientMessageIDInvalid   ValidationCode = "client_message_id_invalid"
	ValidationLastReadMessageIDInvalid ValidationCode = "last_read_message_id_invalid"
	ValidationReplyToMessageIDInvalid  ValidationCode = "reply_to_message_id_invalid"
	ValidationMentionSubjectIDsInvalid ValidationCode = "mention_subject_ids_invalid"
	ValidationBodyRequired             ValidationCode = "body_required"
	ValidationBodyTooLong              ValidationCode = "body_too_long"
	ValidationCursorInvalid            ValidationCode = "cursor_invalid"
)

const (
	// ConflictReasonIdempotencyMismatch 表示同一消息编号对应了不同写入意图。
	ConflictReasonIdempotencyMismatch = "idempotency_mismatch"
	// ConflictReasonServiceSessionOwned 表示客服处理周期已由其他主体负责。
	ConflictReasonServiceSessionOwned = "service_session_owned"
	// ConflictReasonServiceSessionNotReplyable 表示客服处理周期当前不可回复。
	ConflictReasonServiceSessionNotReplyable = "service_session_not_replyable"
	// ConflictReasonChannelOutboundUnavailable 表示来源渠道已停用或尚未配置。
	ConflictReasonChannelOutboundUnavailable = "channel_outbound_unavailable"
	// ConflictReasonChannelOutboundUnsupported 表示来源渠道尚不支持外发。
	ConflictReasonChannelOutboundUnsupported = "channel_outbound_unsupported"
	// ConflictReasonServiceSessionAlreadyOpen 表示客服处理周期已经打开。
	ConflictReasonServiceSessionAlreadyOpen = "service_session_already_open"
	// ConflictReasonGroupMemberAlreadyActive 表示成员已经在群聊中。
	ConflictReasonGroupMemberAlreadyActive = "group_member_already_active"
	// ConflictReasonGroupMemberNotActive 表示目标不是当前有效群成员。
	ConflictReasonGroupMemberNotActive = "group_member_not_active"
	// ConflictReasonGroupOwnerCannotBeRemoved 表示群主不能通过移除成员操作退出。
	ConflictReasonGroupOwnerCannotBeRemoved = "group_owner_cannot_be_removed"
	// ConflictReasonGroupOwnerCannotLeave 表示群主必须先转让后退出。
	ConflictReasonGroupOwnerCannotLeave = "group_owner_cannot_leave"
	// ConflictReasonReplyTargetInvalid 表示引用目标不是当前会话中的有效文本或附件消息。
	ConflictReasonReplyTargetInvalid = "reply_target_invalid"
	// ConflictReasonGroupMentionTargetInvalid 表示提醒目标不是当前群聊中的有效参与者。
	ConflictReasonGroupMentionTargetInvalid = "group_mention_target_invalid"
)

// ServiceSessionAssignee 定义客服处理周期负责人。
type ServiceSessionAssignee struct {
	IdentityID   string
	Type         domain.OrganizationIdentityType
	DisplayName  string
	AvatarFileID *string
}

// ServiceSessionResult 定义客服处理周期命令结果。
type ServiceSessionResult struct {
	ID       string
	Status   domain.ServiceSessionStatus
	Assignee *ServiceSessionAssignee
	ClosedAt *time.Time
}

// TransferServiceSessionInput 定义客服处理周期转交目标。
type TransferServiceSessionInput struct {
	ConversationID     string
	AssigneeIdentityID string
}

// WebsiteCustomerTextMessageInput 定义网站客户文本消息。
type WebsiteCustomerTextMessageInput struct {
	ReplyToMessageID string
	ChannelID        string
	ExternalID       string
	ConversationID   *string
	ClientMessageID  string
	Body             string
}

// ConversationSummary 定义访客可见会话摘要。
type ConversationSummary struct {
	LastMessageSeq            int64
	ID                        string
	Title                     string
	Preview                   string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             time.Time
	ServiceSessionID          string
	ServiceSessionStatus      domain.ServiceSessionStatus
}

// MessageReference 定义访客可见的一层引用摘要。
type MessageReference struct {
	SenderIdentityType *domain.OrganizationIdentityType
	ID                 string
	Deleted            bool
	Author             domain.MessageAuthor
	Body               string
}

// Message 定义访客可见消息。
type Message struct {
	ClientMessageID    *string
	MessageSeq         int64
	ReplyTo            *MessageReference
	ID                 string
	Author             domain.MessageAuthor
	SenderIdentityType *domain.OrganizationIdentityType
	Body               string
	OriginatedAt       time.Time
	SourceOrder        int64
	CreatedAt          time.Time
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
	MessageSeq int64
	ID         string
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

// ConversationMessageSender 定义成员可见的消息发送主体。
type ConversationMessageSender struct {
	ChatSubjectID string
	Kind          domain.ChatSubjectKind
	SourceID      string
	DisplayName   *string
	AvatarFileID  *string
	IdentityType  *domain.OrganizationIdentityType
}

// ConversationMessageReference 定义引用消息的一层摘要。
type ConversationMessageReference struct {
	Type    domain.MessageType
	Deleted bool
	ID      string
	Body    string
	Sender  *ConversationMessageSender
}

// ConversationMessageMention 定义消息提醒的聊天主体。
type ConversationMessageMention struct {
	ChatSubjectID string
	Kind          domain.ChatSubjectKind
	SourceID      string
	DisplayName   *string
}

// ConversationMessageSessionStart 定义客服处理周期开始标记。
type ConversationMessageSessionStart struct {
	Sequence  int64
	StartedAt time.Time
	Status    domain.ServiceSessionStatus
}

// ConversationSystemEventParticipant 定义系统事件中的成员快照。
type ConversationSystemEventParticipant struct {
	IdentityID  string `json:"identityId"`
	DisplayName string `json:"displayName"`
}

// ConversationSystemEvent 定义会话系统事件及其审计载荷。
type ConversationSystemEvent struct {
	Type          domain.ConversationSystemEventType   `json:"-"`
	Actor         ConversationSystemEventParticipant   `json:"actor"`
	Targets       []ConversationSystemEventParticipant `json:"targets"`
	PreviousTitle *string                              `json:"previousTitle,omitempty"`
	Title         *string                              `json:"title,omitempty"`
}

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ClientMessageID *string
	Attachment      *MessageAttachment
	AgentProcess    *ConversationAgentProcess
	MessageSeq      int64
	ID              string
	Type            domain.MessageType
	Body            string
	OriginatedAt    time.Time
	SourceOrder     int64
	CreatedAt       time.Time
	Sender          *ConversationMessageSender
	SessionStart    *ConversationMessageSessionStart
	SystemEvent     *ConversationSystemEvent
	ReplyTo         *ConversationMessageReference
	Mentions        []ConversationMessageMention
	MentionAll      bool
}

// ConversationMessageHistoryInput 定义成员消息历史查询方向。
type ConversationMessageHistoryInput struct {
	ConversationID  string
	Before          *MessageCursorPoint
	After           *MessageCursorPoint
	AroundMessageID string
}

// ConversationMessageHistory 定义成员消息历史和下一页边界。
type ConversationMessageHistory struct {
	LatestAgentRun *ConversationAgentRun
	HasEarlier     bool
	HasLater       bool
	Messages       []ConversationMessage
	Before         *MessageCursorPoint
	After          *MessageCursorPoint
}

// ConversationAgentProcess 定义成功回复的完整过程和模型用量。
type ConversationAgentProcess struct {
	ID                   string
	DurationMilliseconds int64
	Usage                agentruntime.Usage
	Blocks               []agentruntime.Block
}

// ConversationAgentRun 定义会话最近一次运行的状态。
type ConversationAgentRun struct {
	AgentAvatarFileID *string
	AgentName         string
	ID                string
	Status            domain.AgentRunStatus
	ErrorCode         *string
	LastError         *string
}

// CustomerTextMessageInput 定义成员发送的客户会话文本消息。
type CustomerTextMessageInput struct {
	ReplyToMessageID string
	ConversationID   string
	ClientMessageID  string
	Body             string
}

// FirstDirectTextMessageInput 定义成员向目标身份发送的首条单聊消息。
type FirstDirectTextMessageInput struct {
	TargetIdentityID string
	ClientMessageID  string
	Body             string
}

// FirstDirectTextMessageResult 定义首条单聊消息及其确定的长期会话。
type FirstDirectTextMessageResult struct {
	Conversation DirectConversationSummary
	Message      ConversationMessage
}

// DirectConversationSummary 定义成员内部单聊摘要。
type DirectConversationSummary struct {
	ID                        string
	PeerIdentityID            string
	PeerType                  domain.OrganizationIdentityType
	PeerName                  string
	PeerAvatarFileID          *string
	Preview                   *string
	PreviewSenderIdentityType *domain.OrganizationIdentityType
	LastMessageAt             *time.Time
}

// InternalTextMessageInput 定义成员发送的内部单聊文本消息。
type InternalTextMessageInput struct {
	ConversationID   string
	ClientMessageID  string
	Body             string
	ReplyToMessageID string
}

// GroupConversationInput 定义企业成员创建群聊的资料和初始成员。
type GroupConversationInput struct {
	Title             string
	Description       string
	ImageFileID       string
	MemberIdentityIDs []string
}

// GroupConversationProfileInput 定义群聊资料修改参数。
type GroupConversationProfileInput struct {
	ConversationID string
	Title          string
	Description    string
	ImageFileID    *string
}

// GroupConversationMembersInput 定义群聊批量增员参数。
type GroupConversationMembersInput struct {
	ConversationID    string
	MemberIdentityIDs []string
}

// GroupConversationMemberInput 定义群聊单个成员操作参数。
type GroupConversationMemberInput struct {
	ConversationID   string
	MemberIdentityID string
}

// GroupConversationOwnerInput 定义群主转让参数。
type GroupConversationOwnerInput struct {
	ConversationID  string
	OwnerIdentityID string
}

// GroupConversationSummary 定义企业内部群聊摘要。
type GroupConversationSummary struct {
	ID          string
	Title       string
	Status      domain.ConversationStatus
	MemberCount int
	ImageFileID *string
}

// GroupParticipant 定义群聊中的当前有效成员。
type GroupParticipant struct {
	IdentityType  domain.OrganizationIdentityType
	ChatSubjectID string
	IdentityID    string
	DisplayName   string
	AvatarFileID  *string
	Role          domain.ConversationParticipantRole
}

// GroupConversation 定义群聊资料和当前有效成员。
type GroupConversation struct {
	ID           string
	Title        string
	Description  string
	ImageFileID  *string
	Status       domain.ConversationStatus
	CreatedAt    time.Time
	Participants []GroupParticipant
	Muted        bool
}

// GroupTextMessageInput 定义成员发送的群聊文本消息。
type GroupTextMessageInput struct {
	ConversationID    string
	ClientMessageID   string
	Body              string
	ReplyToMessageID  string
	MentionSubjectIDs []string
	MentionAll        bool
}

// ConversationNotificationSettings 定义当前用户的会话提醒设置。
type ConversationNotificationSettings struct {
	Muted bool
}

// MessageAttachment 定义消息文件的元数据。
type MessageAttachment struct {
	UploadStatus domain.AttachmentUploadStatus `bun:"upload_status"`
	ImageWidth   int                           `bun:"image_width"`
	ImageHeight  int                           `bun:"image_height"`
	ID           string                        `bun:"id"`
	Name         string                        `bun:"name"`
	ContentType  string                        `bun:"content_type"`
	ByteSize     int64                         `bun:"byte_size"`
}

// AttachmentMessageInput 定义已有会话或单聊目标的附件发送意图。
type AttachmentMessageInput struct {
	ConversationID   string
	TargetIdentityID string
	ClientMessageID  string
	FileID           string
}

// AttachmentMessageResult 返回附件消息和首发单聊摘要。
type AttachmentMessageResult struct {
	ConversationID string
	Conversation   *DirectConversationSummary
	Message        ConversationMessage
}

// AttachmentBatchItem 定义一条待创建的附件消息。
type AttachmentBatchItem struct {
	Body            string
	File            fileaction.UploadInput
	ClientMessageID string
	ImageWidth      int
	ImageHeight     int
}

// AttachmentBatchInput 定义按选择顺序发送的附件消息。
type AttachmentBatchInput struct {
	ConversationID   string
	TargetIdentityID string
	Attachments      []AttachmentBatchItem
}

// AttachmentBatchResult 返回一次发送的全部消息。
type AttachmentBatchResult struct {
	ConversationID string
	Conversation   *DirectConversationSummary
	Messages       []ConversationMessage
}

// AttachmentMessageState 保存附件消息的当前内容状态和撤去标记。
type AttachmentMessageState struct {
	MessageID  string
	Attachment MessageAttachment
	Deleted    bool
}
