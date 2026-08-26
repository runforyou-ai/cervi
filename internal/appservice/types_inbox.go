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

// InboxConversation 定义成员收件箱中的客户会话列表项。
type InboxConversation struct {
	ID                   string               `json:"id"`
	Title                string               `json:"title"`
	ContactName          *string              `json:"contactName"`
	ChannelType          ChannelType          `json:"channelType"`
	ChannelName          string               `json:"channelName"`
	Preview              string               `json:"preview"`
	LastMessageAt        time.Time            `json:"lastMessageAt"`
	ServiceSessionStatus ServiceSessionStatus `json:"serviceSessionStatus"`
}

// Inbox 定义成员收件箱查询结果。
type Inbox struct {
	Conversations []InboxConversation `json:"conversations"`
}
