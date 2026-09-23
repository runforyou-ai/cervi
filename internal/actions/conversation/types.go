//go:build server

package conversation

import (
	"time"

	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// ValidationCode 标识会话业务输入的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationChannelIDInvalid          ValidationCode = "channel_id_invalid"
	ValidationExternalIDInvalid         ValidationCode = "external_id_invalid"
	ValidationConversationIDInvalid     ValidationCode = "conversation_id_invalid"
	ValidationTargetIdentityIDInvalid   ValidationCode = "target_identity_id_invalid"
	ValidationTargetTeamIDInvalid       ValidationCode = "target_team_id_invalid"
	ValidationTransferTargetKindInvalid ValidationCode = "transfer_target_kind_invalid"
	ValidationGroupTitleRequired        ValidationCode = "group_title_required"
	ValidationGroupTitleTooLong         ValidationCode = "group_title_too_long"
	ValidationGroupDescriptionTooLong   ValidationCode = "group_description_too_long"
	ValidationGroupImageFileIDInvalid   ValidationCode = "group_image_file_id_invalid"
	ValidationGroupMembersRequired      ValidationCode = "group_members_required"
	ValidationGroupMembersTooMany       ValidationCode = "group_members_too_many"
	ValidationGroupMemberIDsInvalid     ValidationCode = "group_member_ids_invalid"
	ValidationGroupMemberIDInvalid      ValidationCode = "group_member_id_invalid"
	ValidationGroupOwnerIDInvalid       ValidationCode = "group_owner_id_invalid"
	ValidationClientMessageIDInvalid    ValidationCode = "client_message_id_invalid"
	ValidationLastReadMessageIDInvalid  ValidationCode = "last_read_message_id_invalid"
	ValidationReplyToMessageIDInvalid   ValidationCode = "reply_to_message_id_invalid"
	ValidationMentionSubjectIDsInvalid  ValidationCode = "mention_subject_ids_invalid"
	ValidationMentionIdentityIDsInvalid ValidationCode = "mention_identity_ids_invalid"
	ValidationMessageVisibilityInvalid  ValidationCode = "message_visibility_invalid"
	ValidationBodyRequired              ValidationCode = "body_required"
	ValidationBodyTooLong               ValidationCode = "body_too_long"
	ValidationCursorInvalid             ValidationCode = "cursor_invalid"
	ValidationFileIDInvalid             ValidationCode = "file_id_invalid"
	ValidationNeighborIDInvalid         ValidationCode = "neighbor_id_invalid"
	ValidationPinPositionInvalid        ValidationCode = "pin_position_invalid"
	ValidationPinOrderVersionInvalid    ValidationCode = "pin_order_version_invalid"
)

const (
	// ConflictReasonIdempotencyMismatch 表示同一消息编号对应了不同写入意图。
	ConflictReasonIdempotencyMismatch = "idempotency_mismatch"
	// ConflictReasonCustomerHandlingRequired 表示当前成员未开启接待客户。
	ConflictReasonCustomerHandlingRequired = "customer_handling_required"
	// ConflictReasonServiceSessionOwned 表示客服处理周期已由其他主体负责。
	ConflictReasonServiceSessionOwned = "service_session_owned"
	// ConflictReasonTransferTeamUnavailable 表示转交目标团队内没有开启接待的真人成员。
	ConflictReasonTransferTeamUnavailable = "transfer_team_unavailable"
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
	// ConflictReasonGroupMemberNotActive 表示目标群成员资格已失效。
	ConflictReasonGroupMemberNotActive = "group_member_not_active"
	// ConflictReasonGroupOwnerCannotBeRemoved 表示移除操作的目标为群主。
	ConflictReasonGroupOwnerCannotBeRemoved = "group_owner_cannot_be_removed"
	// ConflictReasonGroupOwnerCannotLeave 表示群主必须先转让后退出。
	ConflictReasonGroupOwnerCannotLeave = "group_owner_cannot_leave"
	// ConflictReasonReplyTargetInvalid 表示当前会话的引用消息校验失败。
	ConflictReasonReplyTargetInvalid = "reply_target_invalid"
	// ConflictReasonGroupMentionTargetInvalid 表示当前群聊的提醒目标校验失败。
	ConflictReasonGroupMentionTargetInvalid = "group_mention_target_invalid"
	// ConflictReasonNoteMentionTargetInvalid 表示内部备注的提醒目标不是本企业有效成员。
	ConflictReasonNoteMentionTargetInvalid = "note_mention_target_invalid"
	// ConflictReasonChannelAttachmentUnsupported 表示来源渠道尚不支持外发附件。
	ConflictReasonChannelAttachmentUnsupported = "channel_attachment_unsupported"
	// ConflictReasonAttachmentTooLarge 表示附件超过来源渠道的字节上限。
	ConflictReasonAttachmentTooLarge = "attachment_too_large"
	// ConflictReasonCaptionTooLong 表示附件说明超过来源渠道的字符上限。
	ConflictReasonCaptionTooLong = "caption_too_long"
	// ConflictReasonPinOrderVersionStale 表示提交的置顶顺序版本不是当前版本。
	ConflictReasonPinOrderVersionStale = "pin_order_version_stale"
	// ConflictReasonPinNeighborNotPinned 表示置顶顺序的邻居会话当前不在置顶区。
	ConflictReasonPinNeighborNotPinned = "pin_neighbor_not_pinned"
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

// TransferServiceSessionInput 定义客服处理周期的转交去向；成员去向用 IdentityID，团队去向用 TeamID，公共队列两者都不填。
type TransferServiceSessionInput struct {
	ConversationID string
	TargetKind     domain.ServiceSessionTargetKind
	TeamID         string
	IdentityID     string
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

// WebsiteCustomerAttachmentMessageInput 定义网站访客发送的附件消息。
type WebsiteCustomerAttachmentMessageInput struct {
	ReplyToMessageID string
	ChannelID        string
	ExternalID       string
	ConversationID   *string
	ClientMessageID  string
	FileID           string
	Body             string
	ImageWidth       int
	ImageHeight      int
}

// WebsiteVisitorUploadInput 定义网站访客附件上传的渠道归属与文件元数据。
type WebsiteVisitorUploadInput struct {
	ChannelID   string
	ExternalID  string
	FileName    string
	ContentType string
	ByteSize    int64
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
	Attachment         *VisitorAttachment
	ID                 string
	Author             domain.MessageAuthor
	SenderIdentityType *domain.OrganizationIdentityType
	// SenderIdentityID、SenderDisplayName 和 SenderAvatar 仅在组织身份发送时有值。
	SenderIdentityID  string
	SenderDisplayName string
	SenderAvatar      *FileLocation
	Body              string
	OriginatedAt      time.Time
	SourceOrder       int64
	CreatedAt         time.Time
}

// FileLocation 定义文件的存储位置。
type FileLocation struct {
	StorageBackend domain.FileStorageBackend
	StorageKey     string
}

// ReceiveWebsiteCustomerMessageResult 定义网站消息写入结果。
type ReceiveWebsiteCustomerMessageResult struct {
	OrganizationID          string
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
	OrganizationID string
	Messages       []Message
	Before         *MessageCursorPoint
	After          *MessageCursorPoint
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
	ExternalSenderName string
	Type               domain.MessageType
	Visibility         domain.MessageVisibility
	Deleted            bool
	ID                 string
	Body               string
	Sender             *ConversationMessageSender
}

// ConversationMessageMention 定义消息提醒的聊天主体。
type ConversationMessageMention struct {
	ChatSubjectID string
	Kind          domain.ChatSubjectKind
	SourceID      string
	DisplayName   *string
	IdentityType  domain.OrganizationIdentityType
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
	// 以下字段只由 service_session_* 事件携带，结构与 domain.ServiceSessionHandedOffEvent、domain.ServiceSessionOperatedEvent、domain.ServiceSessionReturnedEvent、domain.ServiceSessionAssignedEvent 一致。
	ServiceSessionID *string                            `json:"serviceSessionId,omitempty"`
	ActorIdentityID  *string                            `json:"actorIdentityId,omitempty"`
	ActorDisplayName *string                            `json:"actorDisplayName,omitempty"`
	FromIdentityID   *string                            `json:"fromIdentityId,omitempty"`
	FromDisplayName  *string                            `json:"fromDisplayName,omitempty"`
	Target           *domain.ServiceSessionTarget       `json:"target,omitempty"`
	Reason           *domain.AgentHandoffReason         `json:"reason,omitempty"`
	ReturnReason     *domain.ServiceSessionReturnReason `json:"returnReason,omitempty"`
	CloseReason      *domain.ServiceSessionCloseReason  `json:"closeReason,omitempty"`
	ReasonText       *string                            `json:"reasonText,omitempty"`
	CategoryName     *string                            `json:"categoryName,omitempty"`
	AgentRunID       *string                            `json:"agentRunId,omitempty"`
}

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ReplyUnavailable bool
	ClientMessageID  *string
	Attachment       *MessageAttachment
	AgentProcess     *ConversationAgentProcess
	MessageSeq       int64
	ID               string
	Type             domain.MessageType
	Visibility       domain.MessageVisibility
	Body             string
	OriginatedAt     time.Time
	SourceOrder      int64
	CreatedAt        time.Time
	Sender           *ConversationMessageSender
	SessionStart     *ConversationMessageSessionStart
	SystemEvent      *ConversationSystemEvent
	ReplyTo          *ConversationMessageReference
	Mentions         []ConversationMessageMention
	MentionAll       bool
}

// ConversationMessageHistoryInput 定义成员消息历史查询方向；Start 与 End 同时提供时读取包含两端的连续范围。
type ConversationMessageHistoryInput struct {
	ConversationID  string
	Before          *MessageCursorPoint
	After           *MessageCursorPoint
	AroundMessageID string
	Start           *MessageCursorPoint
	End             *MessageCursorPoint
}

// ConversationMessageHistory 定义成员消息历史和下一页边界。
type ConversationMessageHistory struct {
	AgentRuns     []ConversationAgentRun
	PendingAgents []ConversationPendingAgent
	HasEarlier    bool
	HasLater      bool
	Messages      []ConversationMessage
	Before        *MessageCursorPoint
	After         *MessageCursorPoint
}

// ConversationPendingAgent 定义已收到输入、等待轮转执行的 AI 员工。
type ConversationPendingAgent struct {
	IdentityID   string
	DisplayName  string
	AvatarFileID *string
}

// ConversationAgentProcess 定义已完成运行的过程引用和模型用量。
type ConversationAgentProcess struct {
	ID                   string
	DurationMilliseconds int64
	Usage                agentruntime.Usage
	Outcome              *domain.AgentRunOutcome
	OutcomeReason        *domain.AgentHandoffReason
}

// AgentRunProcess 定义一次已完成运行的有序过程内容和模型用量。
type AgentRunProcess struct {
	ID                   string
	DurationMilliseconds int64
	Usage                agentruntime.Usage
	Outcome              *domain.AgentRunOutcome
	OutcomeReason        *domain.AgentHandoffReason
	Blocks               []agentruntime.Block
}

// ConversationAgentRun 定义尚未由结果消息表达的运行状态，取消运行携带自身过程引用。
type ConversationAgentRun struct {
	AgentAvatarFileID *string
	AgentName         string
	ID                string
	AgentIdentityID   string
	Status            domain.AgentRunStatus
	ErrorCode         *string
	LastError         *string
	Process           *ConversationAgentProcess
	// ExecutionDeviceName 是执行该运行的设备名称，服务端执行时为空。
	ExecutionDeviceName *string
}

// CustomerTextMessageInput 定义成员发送的客户会话文本消息。
type CustomerTextMessageInput struct {
	ReplyToMessageID string
	ConversationID   string
	ClientMessageID  string
	Body             string
	Visibility       domain.MessageVisibility
	// MentionIdentityIDs 是内部备注提醒的企业成员身份，按正文出现顺序排列。
	MentionIdentityIDs []string
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
	LastActivityAt            *time.Time
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
	ImageWidth     int                                    `bun:"image_width"`
	ImageHeight    int                                    `bun:"image_height"`
	ID             string                                 `bun:"id"`
	Name           string                                 `bun:"name"`
	ContentType    string                                 `bun:"content_type"`
	ByteSize       int64                                  `bun:"byte_size"`
	TransferStatus domain.MessageAttachmentTransferStatus `bun:"transfer_status"`
}

// VisitorAttachment 定义访客可见的附件元数据与内容存储位置。
type VisitorAttachment struct {
	MessageAttachment
	StorageBackend domain.FileStorageBackend `bun:"storage_backend"`
	StorageKey     string                    `bun:"storage_key"`
}

// CustomerAttachmentMessageInput 定义成员发送的客户会话附件消息。
type CustomerAttachmentMessageInput struct {
	ConversationID   string
	ClientMessageID  string
	FileID           string
	Body             string
	ReplyToMessageID string
	ImageWidth       int
	ImageHeight      int
}

// AttachmentMessageInput 定义已上传附件的发送意图，AgentIdentityID 非空表示按 ConversationID 草稿编号首发 AI 聊天，同时指定 CustomerConversationID 表示首发该客户会话的 Copilot 线程；首发 AI 聊天时 WorkspaceID 非空则同时绑定本人设备上的该工作区。
type AttachmentMessageInput struct {
	ConversationID         string
	TargetIdentityID       string
	AgentIdentityID        string
	CustomerConversationID string
	ClientMessageID        string
	FileID                 string
	Body                   string
	ImageWidth             int
	ImageHeight            int
	WorkspaceID            string
}

// AttachmentMessageResult 返回附件消息，首发时返回新建单聊或 AI 聊天摘要。
type AttachmentMessageResult struct {
	ConversationID    string
	Conversation      *DirectConversationSummary
	AgentConversation *inboxaction.ConversationSummary
	Message           ConversationMessage
}
