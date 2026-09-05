//go:build server

package conversation

import (
	"time"

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
	ValidationGroupSuccessorIDInvalid  ValidationCode = "group_successor_id_invalid"
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
	// ConflictReasonGroupSuccessorRequired 表示群主退出前必须指定继任者。
	ConflictReasonGroupSuccessorRequired = "group_successor_required"
	// ConflictReasonReplyTargetInvalid 表示引用目标不是当前会话中的有效文本消息。
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
	SourceOrder  int64
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
	GroupMessageSequence *int64
	OriginatedAt         time.Time
	SourceOrder          int64
	ID                   string
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
}

// ConversationMessageReference 定义引用消息的一层摘要。
type ConversationMessageReference struct {
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
	AgentProcess         *ConversationAgentProcess
	GroupMessageSequence *int64
	ID                   string
	Type                 domain.MessageType
	Body                 string
	OriginatedAt         time.Time
	SourceOrder          int64
	CreatedAt            time.Time
	Sender               *ConversationMessageSender
	SessionStart         *ConversationMessageSessionStart
	SystemEvent          *ConversationSystemEvent
	ReplyTo              *ConversationMessageReference
	Mentions             []ConversationMessageMention
	MentionAll           bool
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
	AgentName string
	ID        string
	Status    domain.AgentRunStatus
	ErrorCode *string
	LastError *string
}

// CustomerTextMessageInput 定义成员发送的客户会话文本消息。
type CustomerTextMessageInput struct {
	ConversationID  string
	ClientMessageID string
	Body            string
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
	ID             string
	PeerIdentityID string
	PeerType       domain.OrganizationIdentityType
	PeerName       string
	Preview        *string
	LastMessageAt  *time.Time
}

// DirectTextMessageInput 定义成员发送的内部单聊文本消息。
type DirectTextMessageInput struct {
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

// GroupConversationLeaveInput 定义当前成员退出群聊参数。
type GroupConversationLeaveInput struct {
	ConversationID      string
	SuccessorIdentityID string
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
