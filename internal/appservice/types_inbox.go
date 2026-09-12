package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// ServiceSessionStatus 表示客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusOpen   ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusOpen)
	ServiceSessionStatusClosed ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusClosed)
)

// AgentRunStatus 表示会话中 Agent 最近一次运行状态。
type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusQueued)
	AgentRunStatusRunning   AgentRunStatus = AgentRunStatus(domain.AgentRunStatusRunning)
	AgentRunStatusSucceeded AgentRunStatus = AgentRunStatus(domain.AgentRunStatusSucceeded)
	AgentRunStatusFailed    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusFailed)
	AgentRunStatusCancelled AgentRunStatus = AgentRunStatus(domain.AgentRunStatusCancelled)
)

// InboxScope 表示统一收件箱读取范围。
type InboxScope string

const (
	InboxScopeAll      InboxScope = InboxScope(domain.InboxScopeAll)
	InboxScopeCustomer InboxScope = InboxScope(domain.InboxScopeCustomer)
	InboxScopeInternal InboxScope = InboxScope(domain.InboxScopeInternal)
)

// CustomerInboxView 表示客户会话的处理归属视图。
type CustomerInboxView string

const (
	CustomerInboxViewQueue     CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewQueue)
	CustomerInboxViewMine      CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewMine)
	CustomerInboxViewCoworkers CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewCoworkers)
)

// InboxQuery 定义与分页边界无关的会话筛选。
type InboxQuery struct {
	Scope              InboxScope           `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView    `json:"customerView" query:"customerView"`
	AssigneeIdentityID string               `json:"assigneeIdentityId" query:"assigneeIdentityId"`
	ChannelID          string               `json:"channelId" query:"channelId"`
	ServiceStatus      ServiceSessionStatus `json:"serviceStatus" query:"serviceStatus"`
	Kinds              []ConversationType   `json:"kinds" query:"kinds"`
}

// LoadInboxInput 定义统一收件箱筛选和分页边界。
type LoadInboxInput struct {
	Scope              InboxScope           `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView    `json:"customerView" query:"customerView"`
	AssigneeIdentityID string               `json:"assigneeIdentityId" query:"assigneeIdentityId"`
	ChannelID          string               `json:"channelId" query:"channelId"`
	ServiceStatus      ServiceSessionStatus `json:"serviceStatus" query:"serviceStatus"`
	Kinds              []ConversationType   `json:"kinds" query:"kinds"`
	Cursor             string               `json:"cursor" query:"cursor"`
	BeforeCursor       string               `json:"beforeCursor" query:"beforeCursor"`
	Limit              int                  `json:"limit" query:"limit,default=50"`
}

// query 返回不含分页边界的会话筛选。
func (input LoadInboxInput) query() InboxQuery {
	return InboxQuery{
		Scope: input.Scope, CustomerView: input.CustomerView, AssigneeIdentityID: input.AssigneeIdentityID,
		ChannelID: input.ChannelID, ServiceStatus: input.ServiceStatus, Kinds: input.Kinds,
	}
}

// InboxAssignee 定义客户会话负责人摘要。
type InboxAssignee struct {
	IdentityID  string                   `json:"identityId"`
	Type        OrganizationIdentityType `json:"type"`
	DisplayName string                   `json:"displayName"`
	AvatarURL   string                   `json:"avatarUrl"`
}

// CustomerServiceAssigneeList 定义客服筛选候选列表。
type CustomerServiceAssigneeList struct {
	Assignees []InboxAssignee `json:"assignees"`
}

// InboxChannel 定义收件箱渠道筛选候选。
type InboxChannel struct {
	ID      string      `json:"id"`
	Type    ChannelType `json:"type"`
	Name    string      `json:"name"`
	Enabled bool        `json:"enabled"`
}

// InboxChannelList 定义收件箱渠道筛选候选列表。
type InboxChannelList struct {
	Channels []InboxChannel `json:"channels"`
}

// ConversationType 表示统一收件箱会话类型。
type ConversationType string

const (
	ConversationTypeCustomer ConversationType = ConversationType(domain.ConversationTypeCustomer)
	ConversationTypeDirect   ConversationType = ConversationType(domain.ConversationTypeDirect)
	ConversationTypeAgent    ConversationType = ConversationType(domain.ConversationTypeAgent)
	ConversationTypeGroup    ConversationType = ConversationType(domain.ConversationTypeGroup)
)

// CustomerInboxConversation 定义客户会话摘要。
type CustomerInboxConversation struct {
	Title                     string                    `json:"title"`
	ContactName               *string                   `json:"contactName"`
	ContactAvatarURL          string                    `json:"contactAvatarUrl"`
	ChannelType               ChannelType               `json:"channelType"`
	ChannelName               string                    `json:"channelName"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
	ServiceSessionStatus      ServiceSessionStatus      `json:"serviceSessionStatus"`
	ServiceSessionID          string                    `json:"serviceSessionId"`
	Assignee                  *InboxAssignee            `json:"assignee"`
}

// DirectInboxConversation 定义内部单聊摘要。
type DirectInboxConversation struct {
	PeerIdentityID            string                    `json:"peerIdentityId"`
	PeerType                  OrganizationIdentityType  `json:"peerType"`
	PeerName                  string                    `json:"peerName"`
	PeerAvatarURL             string                    `json:"peerAvatarUrl"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
}

// AgentInboxConversation 定义 AI 聊天摘要。
type AgentInboxConversation struct {
	Title                     string                    `json:"title"`
	AgentIdentityID           string                    `json:"agentIdentityId"`
	AgentName                 string                    `json:"agentName"`
	AgentAvatarURL            string                    `json:"agentAvatarUrl"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
	AgentRunStatus            *AgentRunStatus           `json:"agentRunStatus"`
}

// GroupInboxConversation 定义企业群聊摘要。
type GroupInboxConversation struct {
	Title                     string                    `json:"title"`
	ImageURL                  string                    `json:"imageUrl"`
	Status                    ConversationStatus        `json:"status"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
	MemberCount               int                       `json:"memberCount"`
}

// InboxConversation 定义成员统一收件箱列表项。
type InboxConversation struct {
	// PositionCursor 保存列表窗口和匹配锚点的查询位置。
	PositionCursor       string                     `json:"positionCursor"`
	LastActivityAt       *time.Time                 `json:"lastActivityAt"`
	LastMessageType      *MessageType               `json:"lastMessageType"`
	ID                   string                     `json:"id"`
	Type                 ConversationType           `json:"type"`
	UnreadCount          int                        `json:"unreadCount"`
	MentionedUnreadCount int                        `json:"mentionedUnreadCount"`
	MarkedUnread         bool                       `json:"markedUnread"`
	Muted                bool                       `json:"muted"`
	LastMessageID        *string                    `json:"lastMessageId"`
	LastReadMessageID    *string                    `json:"lastReadMessageId"`
	Agent                *AgentInboxConversation    `json:"agent"`
	Customer             *CustomerInboxConversation `json:"customer"`
	Direct               *DirectInboxConversation   `json:"direct"`
	Group                *GroupInboxConversation    `json:"group"`
}

// Inbox 定义成员收件箱查询结果。
type Inbox struct {
	StartCursor          string              `json:"startCursor"`
	EndCursor            string              `json:"endCursor"`
	HasBefore            bool                `json:"hasBefore"`
	Conversations        []InboxConversation `json:"conversations"`
	NextCursor           string              `json:"nextCursor"`
	HasMore              bool                `json:"hasMore"`
	UnreadCount          int                 `json:"unreadCount"`
	AttentionUnreadCount int                 `json:"attentionUnreadCount"`
}

// InboxConversationAvailability 表示指定会话的阅读和列表资格。
type InboxConversationAvailability string

const (
	InboxConversationMatching     InboxConversationAvailability = "matching"
	InboxConversationOutsideQuery InboxConversationAvailability = "outside_query"
	InboxConversationUnavailable  InboxConversationAvailability = "unavailable"
)

// ReadInboxConversationsInput 指定待核对的会话及完整列表筛选。
type ReadInboxConversationsInput struct {
	ConversationIDs []string   `json:"conversationIds"`
	Query           InboxQuery `json:"query"`
}

// InboxConversationResult 返回匹配状态，不可用时保留请求 ID 和资格。
type InboxConversationResult struct {
	ID           string                        `json:"id"`
	Availability InboxConversationAvailability `json:"availability"`
	Conversation *InboxConversation            `json:"conversation"`
}

// InboxConversationResults 按请求顺序返回每项结果。
type InboxConversationResults struct {
	Results []InboxConversationResult `json:"results"`
}

// InboxContextInput 按会话当前位置或原查询位置读取邻域，前后数量零值均为二十五。
type InboxContextInput struct {
	Query        InboxQuery `json:"query"`
	AnchorID     string     `json:"anchorId"`
	AnchorCursor string     `json:"anchorCursor"`
	BeforeLimit  int        `json:"beforeLimit"`
	AfterLimit   int        `json:"afterLimit"`
}

// InboxWindowInput 指定同一查询已加载范围的首尾游标，包含两侧边界。
type InboxWindowInput struct {
	Query       InboxQuery `json:"query"`
	StartCursor string     `json:"startCursor"`
	EndCursor   string     `json:"endCursor"`
}

// InboxWindow 保存连续范围和双向续读位置，空范围仍可保留原边界。
type InboxWindow struct {
	Conversations []InboxConversation `json:"conversations"`
	StartCursor   string              `json:"startCursor"`
	EndCursor     string              `json:"endCursor"`
	HasBefore     bool                `json:"hasBefore"`
	HasAfter      bool                `json:"hasAfter"`
}

// InboxContext 独立返回锚点资格，列表只渲染 Window，Anchor 可与窗口行重叠。
type InboxContext struct {
	Anchor InboxConversationResult `json:"anchor"`
	Window InboxWindow             `json:"window"`
}
