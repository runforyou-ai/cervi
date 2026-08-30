//go:build server

package appservice

import (
	"context"
	"log/slog"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// LoadInbox 返回当前企业的统一会话工作队列。
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
	avatarFileIDs := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Customer != nil && summary.Customer.ContactAvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Customer.ContactAvatarFileID)
		}
	}
	avatarURLs, err := b.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		slog.Warn("读取收件箱联系人头像失败", "organization_id", identity.Organization.ID, "error", err)
		return Inbox{}, FailedError(meta, cervii18n.ErrorInboxLoadFailed)
	}
	conversations := make([]InboxConversation, 0, len(summaries))
	for _, summary := range summaries {
		conversation := InboxConversation{ID: summary.ID, Type: ConversationType(summary.Type)}
		if summary.Customer != nil {
			conversation.Customer = &CustomerInboxConversation{
				Title: summary.Customer.Title, ContactName: summary.Customer.ContactName,
				ContactAvatarURL: optionalFileURL(avatarURLs, summary.Customer.ContactAvatarFileID),
				ChannelType:      ChannelType(summary.Customer.ChannelType), ChannelName: summary.Customer.ChannelName,
				Preview: summary.Customer.Preview, LastMessageAt: summary.Customer.LastMessageAt,
				ServiceSessionStatus: ServiceSessionStatus(summary.Customer.ServiceSessionStatus),
			}
		}
		if summary.Direct != nil {
			var agentRunStatus *AgentRunStatus
			if summary.Direct.AgentRunStatus != nil {
				status := AgentRunStatus(*summary.Direct.AgentRunStatus)
				agentRunStatus = &status
			}
			conversation.Direct = &DirectInboxConversation{
				PeerIdentityID: summary.Direct.PeerIdentityID, PeerType: OrganizationIdentityType(summary.Direct.PeerType), PeerName: summary.Direct.PeerName,
				Preview: summary.Direct.Preview, LastMessageAt: summary.Direct.LastMessageAt, AgentRunStatus: agentRunStatus,
			}
		}
		conversations = append(conversations, conversation)
	}
	return Inbox{Conversations: conversations}, nil
}
