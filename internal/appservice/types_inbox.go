package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// ServiceSessionStatus 表示客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusWaiting ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusWaiting)
	ServiceSessionStatusActive  ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusActive)
	ServiceSessionStatusPending ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusPending)
	ServiceSessionStatusClosed  ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusClosed)
)

// ConversationType 表示统一收件箱会话类型。
type ConversationType string

const (
	ConversationTypeCustomer ConversationType = ConversationType(domain.ConversationTypeCustomer)
	ConversationTypeDirect   ConversationType = ConversationType(domain.ConversationTypeDirect)
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
}

// DirectInboxConversation 定义内部单聊摘要。
type DirectInboxConversation struct {
	PeerIdentityID string     `json:"peerIdentityId"`
	PeerName       string     `json:"peerName"`
	Preview        *string    `json:"preview"`
	LastMessageAt  *time.Time `json:"lastMessageAt"`
}

// InboxConversation 定义成员统一收件箱列表项。
type InboxConversation struct {
	ID       string                     `json:"id"`
	Type     ConversationType           `json:"type"`
	Customer *CustomerInboxConversation `json:"customer"`
	Direct   *DirectInboxConversation   `json:"direct"`
}

// Inbox 定义成员收件箱查询结果。
type Inbox struct {
	Conversations []InboxConversation `json:"conversations"`
}
