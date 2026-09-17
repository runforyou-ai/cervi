//go:build server

// Package realtime 在写事务内登记受众通知，并在事务提交成功后发布到 NATS。
package realtime

import (
	"context"

	"github.com/uptrace/bun"
)

// AudienceKind 定义通知受众种类。
type AudienceKind string

const (
	AudienceUser             AudienceKind = "user"
	AudienceCustomerInbox    AudienceKind = "customer_inbox"
	AudienceVisitorDirectory AudienceKind = "visitor_directory"
	AudienceWebsiteChannel   AudienceKind = "website_channel"
)

// Kind 定义通知种类。
type Kind string

const (
	KindConversationChanged      Kind = "conversation_changed"
	KindConversationRemoved      Kind = "conversation_removed"
	KindConversationStateChanged Kind = "conversation_state_changed"
	KindIdentityProfileChanged   Kind = "identity_profile_changed"
	KindSessionLoggedOut         Kind = "session_logged_out"
	KindUserDisabled             Kind = "user_disabled"
	KindChannelDisabled          Kind = "channel_disabled"
)

// Notification 表示发往单个受众的变更通知或撤销控制，载荷含通知种类、会话 ID、版本与登录会话 ID，零值字段省略。
type Notification struct {
	OrganizationID string
	AudienceKind   AudienceKind
	AudienceID     string
	Kind           Kind
	ConversationID string
	Version        int64
	TokenSessionID string
}

// UserConversationChanged 构造发往用户受众的会话变更通知。
func UserConversationChanged(organizationID, userID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindConversationChanged, ConversationID: conversationID, Version: version}
}

// CustomerInboxConversationChanged 构造发往企业客服共享受众的客户会话变更通知。
func CustomerInboxConversationChanged(organizationID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceCustomerInbox, AudienceID: organizationID, Kind: KindConversationChanged, ConversationID: conversationID, Version: version}
}

// VisitorDirectoryConversationChanged 构造发往网站渠道身份受众的客户线程变更通知，受众 ID 为渠道身份记录 ID。
func VisitorDirectoryConversationChanged(organizationID, channelIdentityID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceVisitorDirectory, AudienceID: channelIdentityID, Kind: KindConversationChanged, ConversationID: conversationID, Version: version}
}

// UserConversationRemoved 构造发往失去会话阅读资格用户的会话失权通知，载荷不含版本。
func UserConversationRemoved(organizationID, userID, conversationID string) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindConversationRemoved, ConversationID: conversationID}
}

// UserConversationStateChanged 构造发往本人受众的个人会话状态通知。
func UserConversationStateChanged(organizationID, userID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindConversationStateChanged, ConversationID: conversationID, Version: version}
}

// UserIdentityProfileChanged 构造发往本人受众的身份资料通知。
func UserIdentityProfileChanged(organizationID, userID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindIdentityProfileChanged, Version: version}
}

// WebsiteChannelDisabled 构造网站渠道停用撤销控制，Gateway 据此结束该渠道全部访客事件流；受众 ID 为渠道 ID。
func WebsiteChannelDisabled(organizationID, channelID string) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceWebsiteChannel, AudienceID: channelID, Kind: KindChannelDisabled}
}

// UserSessionLoggedOut 构造登出撤销控制，Gateway 据此关闭该登录会话的连接。
func UserSessionLoggedOut(organizationID, userID, tokenSessionID string) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindSessionLoggedOut, TokenSessionID: tokenSessionID}
}

// UserDisabled 构造账号停用撤销控制，Gateway 据此关闭该用户的全部连接。
func UserDisabled(organizationID, userID string) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindUserDisabled}
}

type batchKey struct{}

// mergeKey 标识可合并的通知：同一受众、同一种类、同一会话、同一登录会话。
type mergeKey struct {
	organizationID string
	audienceKind   AudienceKind
	audienceID     string
	kind           Kind
	conversationID string
	tokenSessionID string
}

// batch 按登记顺序保存一次事务内合并后的通知。
type batch struct {
	order []mergeKey
	items map[mergeKey]Notification
}

// RunInTx 在 *bun.DB 或 bun.Conn 上执行写事务，事务内通过 Notify 登记的通知在提交成功后交给发布器，回滚则丢弃。
func RunInTx(ctx context.Context, db bun.IDB, fn func(context.Context, bun.Tx) error) error {
	pending := &batch{items: map[mergeKey]Notification{}}
	if err := db.RunInTx(context.WithValue(ctx, batchKey{}, pending), nil, fn); err != nil {
		return err
	}
	if publisher := active.Load(); publisher != nil && len(pending.order) > 0 {
		notifications := make([]Notification, 0, len(pending.order))
		for _, key := range pending.order {
			notifications = append(notifications, pending.items[key])
		}
		publisher.enqueue(notifications)
	}
	return nil
}

// Notify 在当前 RunInTx 事务内登记通知，同一受众、种类和会话只保留最高版本；调用方必须处于 RunInTx 内。
func Notify(ctx context.Context, notification Notification) {
	pending, ok := ctx.Value(batchKey{}).(*batch)
	if !ok {
		panic("realtime: Notify called outside realtime.RunInTx")
	}
	key := mergeKey{notification.OrganizationID, notification.AudienceKind, notification.AudienceID, notification.Kind, notification.ConversationID, notification.TokenSessionID}
	current, exists := pending.items[key]
	if !exists {
		pending.order = append(pending.order, key)
	}
	if !exists || notification.Version > current.Version {
		pending.items[key] = notification
	}
}
