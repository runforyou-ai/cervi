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

// AgentRunStatus 表示 Agent 单聊当前最近一次运行状态。
type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusQueued)
	AgentRunStatusRunning   AgentRunStatus = AgentRunStatus(domain.AgentRunStatusRunning)
	AgentRunStatusSucceeded AgentRunStatus = AgentRunStatus(domain.AgentRunStatusSucceeded)
	AgentRunStatusFailed    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusFailed)
)

// InboxScope 表示统一收件箱读取范围。
type InboxScope string

const (
	InboxScopeAll      InboxScope = InboxScope(domain.InboxScopeAll)
	InboxScopeCustomer InboxScope = InboxScope(domain.InboxScopeCustomer)
	InboxScopeInternal InboxScope = InboxScope(domain.InboxScopeInternal)
)

// CustomerInboxView 表示客户会话队列视图。
type CustomerInboxView string

const (
	CustomerInboxViewQueue     CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewQueue)
	CustomerInboxViewMine      CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewMine)
	CustomerInboxViewCoworkers CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewCoworkers)
	CustomerInboxViewClosed    CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewClosed)
)

// LoadInboxInput 定义统一收件箱查询条件。
type LoadInboxInput struct {
	Scope              InboxScope        `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView `json:"customerView" query:"customerView"`
	AssigneeIdentityID string            `json:"assigneeIdentityId" query:"assigneeIdentityId"`
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

// ConversationType 表示统一收件箱会话类型。
type ConversationType string

const (
	ConversationTypeCustomer ConversationType = ConversationType(domain.ConversationTypeCustomer)
	ConversationTypeDirect   ConversationType = ConversationType(domain.ConversationTypeDirect)
	ConversationTypeGroup    ConversationType = ConversationType(domain.ConversationTypeGroup)
)

// CustomerInboxConversation 定义客户会话摘要。
type CustomerInboxConversation struct {
	Title                string               `json:"title"`
	ContactName          *string              `json:"contactName"`
	ContactAvatarURL     string               `json:"contactAvatarUrl"`
	ChannelType          ChannelType          `json:"channelType"`
	ChannelName          string               `json:"channelName"`
	Preview              *string              `json:"preview"`
	LastMessageAt        *time.Time           `json:"lastMessageAt"`
	ServiceSessionStatus ServiceSessionStatus `json:"serviceSessionStatus"`
	ServiceSessionID     string               `json:"serviceSessionId"`
	Assignee             *InboxAssignee       `json:"assignee"`
}

// DirectInboxConversation 定义内部单聊摘要。
type DirectInboxConversation struct {
	PeerIdentityID string                   `json:"peerIdentityId"`
	PeerType       OrganizationIdentityType `json:"peerType"`
	PeerName       string                   `json:"peerName"`
	PeerAvatarURL  string                   `json:"peerAvatarUrl"`
	Preview        *string                  `json:"preview"`
	LastMessageAt  *time.Time               `json:"lastMessageAt"`
	AgentRunStatus *AgentRunStatus          `json:"agentRunStatus"`
}

// GroupInboxConversation 定义企业群聊摘要。
type GroupInboxConversation struct {
	Title         string             `json:"title"`
	ImageURL      string             `json:"imageUrl"`
	Status        ConversationStatus `json:"status"`
	Preview       *string            `json:"preview"`
	LastMessageAt *time.Time         `json:"lastMessageAt"`
	MemberCount   int                `json:"memberCount"`
}

// InboxConversation 定义成员统一收件箱列表项。
type InboxConversation struct {
	ID                   string                     `json:"id"`
	Type                 ConversationType           `json:"type"`
	UnreadCount          int                        `json:"unreadCount"`
	MentionedUnreadCount int                        `json:"mentionedUnreadCount"`
	MarkedUnread         bool                       `json:"markedUnread"`
	Muted                bool                       `json:"muted"`
	LastMessageID        *string                    `json:"lastMessageId"`
	LastReadMessageID    *string                    `json:"lastReadMessageId"`
	Customer             *CustomerInboxConversation `json:"customer"`
	Direct               *DirectInboxConversation   `json:"direct"`
	Group                *GroupInboxConversation    `json:"group"`
}

// Inbox 定义成员收件箱查询结果。
type Inbox struct {
	Conversations        []InboxConversation `json:"conversations"`
	UnreadCount          int                 `json:"unreadCount"`
	AttentionUnreadCount int                 `json:"attentionUnreadCount"`
}
