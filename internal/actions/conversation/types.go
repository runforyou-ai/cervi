//go:build server

package conversation

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识会话业务输入的校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationChannelIDInvalid        ValidationCode = "channel_id_invalid"
	ValidationExternalIDInvalid       ValidationCode = "external_id_invalid"
	ValidationConversationIDInvalid   ValidationCode = "conversation_id_invalid"
	ValidationTargetIdentityIDInvalid ValidationCode = "target_identity_id_invalid"
	ValidationGroupTitleRequired      ValidationCode = "group_title_required"
	ValidationGroupTitleTooLong       ValidationCode = "group_title_too_long"
	ValidationGroupMembersRequired    ValidationCode = "group_members_required"
	ValidationGroupMembersTooMany     ValidationCode = "group_members_too_many"
	ValidationGroupMemberIDsInvalid   ValidationCode = "group_member_ids_invalid"
	ValidationClientMessageIDInvalid  ValidationCode = "client_message_id_invalid"
	ValidationBodyRequired            ValidationCode = "body_required"
	ValidationBodyTooLong             ValidationCode = "body_too_long"
	ValidationCursorInvalid           ValidationCode = "cursor_invalid"
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
	SourceOrder  int64
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

// ConversationMessageSender 定义成员可见的消息发送主体。
type ConversationMessageSender struct {
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

// ConversationMessage 定义成员可见的会话消息。
type ConversationMessage struct {
	ID           string
	Type         domain.MessageType
	Body         string
	OriginatedAt time.Time
	CreatedAt    time.Time
	Sender       *ConversationMessageSender
	SessionStart *ConversationMessageSessionStart
}

// ConversationMessageHistoryInput 定义成员消息历史查询方向。
type ConversationMessageHistoryInput struct {
	ConversationID string
	Before         *MessageCursorPoint
	After          *MessageCursorPoint
}

// ConversationMessageHistory 定义成员消息历史和下一页边界。
type ConversationMessageHistory struct {
	Messages []ConversationMessage
	Before   *MessageCursorPoint
	After    *MessageCursorPoint
}

// CustomerTextMessageInput 定义成员发送的客户会话文本消息。
type CustomerTextMessageInput struct {
	ConversationID  string
	ClientMessageID string
	Body            string
}

// DirectConversationInput 定义成员发起内部单聊的目标。
type DirectConversationInput struct {
	TargetIdentityID string
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
	ConversationID  string
	ClientMessageID string
	Body            string
}

// GroupConversationInput 定义企业成员创建群聊的标题和初始成员。
type GroupConversationInput struct {
	Title             string
	MemberIdentityIDs []string
}

// GroupConversationSummary 定义企业内部群聊摘要。
type GroupConversationSummary struct {
	ID          string
	Title       string
	MemberCount int
}

// GroupParticipant 定义群聊中的当前有效成员。
type GroupParticipant struct {
	IdentityID   string
	DisplayName  string
	AvatarFileID *string
	Role         domain.ConversationParticipantRole
}

// GroupConversation 定义群聊资料和当前有效成员。
type GroupConversation struct {
	ID           string
	Title        string
	Participants []GroupParticipant
}

// GroupTextMessageInput 定义成员发送的群聊文本消息。
type GroupTextMessageInput struct {
	ConversationID  string
	ClientMessageID string
	Body            string
}
