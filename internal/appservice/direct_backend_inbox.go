//go:build server

package appservice

import (
	"context"
	"log/slog"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// LoadInbox 返回当前企业的客户会话工作队列。
func (b *DirectBackend) LoadInbox(ctx context.Context, meta RequestMeta) (Inbox, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Inbox{}, err
	}
	summaries, err := b.loadInbox.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return Inbox{}, ctx.Err()
		}
		slog.Warn("读取收件箱会话列表失败", "organization_id", identity.Organization.ID, "error", err)
		return Inbox{}, FailedError(meta, cervii18n.ErrorInboxLoadFailed)
	}
	conversations := make([]InboxConversation, 0, len(summaries))
	for _, summary := range summaries {
		conversations = append(conversations, InboxConversation{
			ID:                   summary.ID,
			Title:                summary.Title,
			ContactName:          summary.ContactName,
			ChannelType:          ChannelType(summary.ChannelType),
			ChannelName:          summary.ChannelName,
			Preview:              summary.Preview,
			LastMessageAt:        summary.LastMessageAt,
			ServiceSessionStatus: ServiceSessionStatus(summary.ServiceSessionStatus),
		})
	}
	return Inbox{Conversations: conversations}, nil
}
