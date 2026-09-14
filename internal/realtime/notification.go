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
)

// Kind 定义通知种类。
type Kind string

const (
	KindConversationChanged      Kind = "conversation_changed"
	KindConversationStateChanged Kind = "conversation_state_changed"
	KindIdentityProfileChanged   Kind = "identity_profile_changed"
)

// Notification 表示发往单个受众的变更通知，只携带会话 ID 与版本，不含业务内容。
type Notification struct {
	OrganizationID string
	AudienceKind   AudienceKind
	AudienceID     string
	Kind           Kind
	ConversationID string
	Version        int64
}

// UserConversationChanged 构造发往用户受众的会话变更通知。
func UserConversationChanged(organizationID, userID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindConversationChanged, ConversationID: conversationID, Version: version}
}

// UserConversationStateChanged 构造发往本人受众的个人会话状态通知。
func UserConversationStateChanged(organizationID, userID, conversationID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindConversationStateChanged, ConversationID: conversationID, Version: version}
}

// UserIdentityProfileChanged 构造发往本人受众的身份资料通知。
func UserIdentityProfileChanged(organizationID, userID string, version int64) Notification {
	return Notification{OrganizationID: organizationID, AudienceKind: AudienceUser, AudienceID: userID, Kind: KindIdentityProfileChanged, Version: version}
}

type batchKey struct{}

// mergeKey 标识可合并的通知：同一受众、同一种类、同一会话。
type mergeKey struct {
	organizationID string
	audienceKind   AudienceKind
	audienceID     string
	kind           Kind
	conversationID string
}

// batch 按登记顺序保存一次事务内合并后的通知。
type batch struct {
	order []mergeKey
	items map[mergeKey]Notification
}

// RunInTx 执行写事务，事务内通过 Notify 登记的通知在提交成功后交给发布器，回滚则丢弃。
func RunInTx(ctx context.Context, db *bun.DB, fn func(context.Context, bun.Tx) error) error {
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
	key := mergeKey{notification.OrganizationID, notification.AudienceKind, notification.AudienceID, notification.Kind, notification.ConversationID}
	current, exists := pending.items[key]
	if !exists {
		pending.order = append(pending.order, key)
	}
	if !exists || notification.Version > current.Version {
		pending.items[key] = notification
	}
}
