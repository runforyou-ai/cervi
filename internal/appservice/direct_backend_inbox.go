//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// LoadInbox 返回当前企业的统一会话工作队列。
func (b *DirectBackend) LoadInbox(ctx context.Context, meta RequestMeta, input LoadInboxInput) (Inbox, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Inbox{}, err
	}
	summaries, err := b.loadInbox.Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScope(input.Scope), CustomerView: domain.CustomerInboxView(input.CustomerView), AssigneeIdentityID: input.AssigneeIdentityID})
	if err != nil {
		if ctx.Err() != nil {
			return Inbox{}, ctx.Err()
		}
		if errors.Is(err, inboxaction.ErrQueryInvalid) {
			return Inbox{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
		}
		slog.Warn("读取收件箱会话列表失败", "organization_id", identity.Organization.ID, "error", err)
		return Inbox{}, FailedError(meta, cervii18n.ErrorInboxLoadFailed)
	}
	avatarFileIDs := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Customer == nil {
			continue
		}
		if summary.Customer.ContactAvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Customer.ContactAvatarFileID)
		}
		if summary.Customer.Assignee != nil && summary.Customer.Assignee.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Customer.Assignee.AvatarFileID)
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
			var assignee *InboxAssignee
			if summary.Customer.Assignee != nil {
				assignee = &InboxAssignee{IdentityID: summary.Customer.Assignee.IdentityID, Type: OrganizationIdentityType(summary.Customer.Assignee.Type), DisplayName: summary.Customer.Assignee.DisplayName, AvatarURL: optionalFileURL(avatarURLs, summary.Customer.Assignee.AvatarFileID)}
			}
			conversation.Customer = &CustomerInboxConversation{
				Title: summary.Customer.Title, ContactName: summary.Customer.ContactName,
				ContactAvatarURL: optionalFileURL(avatarURLs, summary.Customer.ContactAvatarFileID),
				ChannelType:      ChannelType(summary.Customer.ChannelType), ChannelName: summary.Customer.ChannelName,
				Preview: summary.Customer.Preview, LastMessageAt: summary.Customer.LastMessageAt,
				ServiceSessionID: summary.Customer.ServiceSessionID, ServiceSessionStatus: ServiceSessionStatus(summary.Customer.ServiceSessionStatus), Assignee: assignee,
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
		if summary.Group != nil {
			conversation.Group = &GroupInboxConversation{
				Title: summary.Group.Title, Status: ConversationStatus(summary.Group.Status), Preview: summary.Group.Preview,
				LastMessageAt: summary.Group.LastMessageAt, MemberCount: summary.Group.MemberCount,
			}
		}
		conversations = append(conversations, conversation)
	}
	return Inbox{Conversations: conversations}, nil
}

// ListCustomerServiceAssignees 返回有效真人和 AI 客服。
func (b *DirectBackend) ListCustomerServiceAssignees(ctx context.Context, meta RequestMeta) (CustomerServiceAssigneeList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerServiceAssigneeList{}, err
	}
	items, err := b.listCustomerServiceAssignees.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerServiceAssigneeList{}, ctx.Err()
		}
		slog.Warn("读取客服候选失败", "organization_id", identity.Organization.ID, "error", err)
		return CustomerServiceAssigneeList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	avatarFileIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *item.AvatarFileID)
		}
	}
	avatarURLs, err := b.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		slog.Warn("读取客服候选头像失败", "organization_id", identity.Organization.ID, "error", err)
		return CustomerServiceAssigneeList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	assignees := make([]InboxAssignee, 0, len(items))
	for _, item := range items {
		assignees = append(assignees, InboxAssignee{IdentityID: item.IdentityID, Type: OrganizationIdentityType(item.Type), DisplayName: item.DisplayName, AvatarURL: optionalFileURL(avatarURLs, item.AvatarFileID)})
	}
	return CustomerServiceAssigneeList{Assignees: assignees}, nil
}
