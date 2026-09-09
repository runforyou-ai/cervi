//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// LoadInbox 返回当前企业的统一会话工作队列。
func (b *DirectBackend) LoadInbox(ctx context.Context, meta RequestMeta, input LoadInboxInput) (Inbox, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Inbox{}, err
	}
	page, unreadCounts, err := b.loadInbox.Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScope(input.Scope), CustomerView: domain.CustomerInboxView(input.CustomerView), AssigneeIdentityID: input.AssigneeIdentityID, Cursor: input.Cursor, Limit: input.Limit})
	if err != nil {
		if ctx.Err() != nil {
			return Inbox{}, ctx.Err()
		}
		if errors.Is(err, inboxaction.ErrCursorInvalid) {
			return Inbox{}, InvalidError(meta, cervii18n.ErrorInboxCursorInvalid, nil).WithReason("inbox_cursor_invalid")
		}
		if errors.Is(err, inboxaction.ErrQueryInvalid) {
			return Inbox{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
		}
		slog.Warn("读取收件箱会话列表失败", "organization_id", identity.Organization.ID, "error", err)
		return Inbox{}, FailedError(meta, cervii18n.ErrorInboxLoadFailed)
	}
	conversations, err := b.inboxConversationsFromActions(ctx, meta, identity, page.Conversations)
	if err != nil {
		return Inbox{}, err
	}
	return Inbox{Conversations: conversations, NextCursor: page.NextCursor, HasMore: page.HasMore, UnreadCount: unreadCounts.Unread, AttentionUnreadCount: unreadCounts.Attention}, nil
}

// inboxConversationsFromActions 为会话摘要统一解析头像并转换传输契约。
func (b *DirectBackend) inboxConversationsFromActions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, summaries []inboxaction.ConversationSummary) ([]InboxConversation, error) {
	avatarFileIDs := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Group != nil && summary.Group.ImageFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Group.ImageFileID)
		}
		if summary.Direct != nil && summary.Direct.PeerAvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Direct.PeerAvatarFileID)
		}
		if summary.Agent != nil && summary.Agent.AgentAvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *summary.Agent.AgentAvatarFileID)
		}
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
		slog.Warn("读取收件箱会话图片失败", "organization_id", identity.Organization.ID, "error", err)
		return nil, FailedError(meta, cervii18n.ErrorInboxLoadFailed)
	}
	conversations := make([]InboxConversation, 0, len(summaries))
	for _, summary := range summaries {
		conversation := inboxConversationFromAction(summary, avatarURLs)
		conversations = append(conversations, conversation)
	}
	return conversations, nil
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

// inboxConversationFromAction 转换完整会话摘要并填充头像地址。
func inboxConversationFromAction(summary inboxaction.ConversationSummary, avatarURLs map[string]string) InboxConversation {
	conversation := InboxConversation{ID: summary.ID, LastActivityAt: summary.LastActivityAt, Type: ConversationType(summary.Type), UnreadCount: summary.UnreadCount, MentionedUnreadCount: summary.MentionedUnreadCount, Muted: summary.Muted, MarkedUnread: summary.MarkedUnread, LastMessageID: summary.LastMessageID, LastMessageType: (*MessageType)(summary.LastMessageType), LastReadMessageID: summary.LastReadMessageID}
	if summary.Customer != nil {
		var assignee *InboxAssignee
		if summary.Customer.Assignee != nil {
			assignee = &InboxAssignee{IdentityID: summary.Customer.Assignee.IdentityID, Type: OrganizationIdentityType(summary.Customer.Assignee.Type), DisplayName: summary.Customer.Assignee.DisplayName, AvatarURL: optionalFileURL(avatarURLs, summary.Customer.Assignee.AvatarFileID)}
		}
		conversation.Customer = &CustomerInboxConversation{
			Title: summary.Customer.Title, ContactName: summary.Customer.ContactName,
			ContactAvatarURL: optionalFileURL(avatarURLs, summary.Customer.ContactAvatarFileID),
			ChannelType:      ChannelType(summary.Customer.ChannelType), ChannelName: summary.Customer.ChannelName,
			Preview: summary.Customer.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Customer.PreviewSenderIdentityType), LastMessageAt: summary.Customer.LastMessageAt,
			ServiceSessionID: summary.Customer.ServiceSessionID, ServiceSessionStatus: ServiceSessionStatus(summary.Customer.ServiceSessionStatus), Assignee: assignee,
		}
	}
	if summary.Direct != nil {
		conversation.Direct = &DirectInboxConversation{
			PeerIdentityID: summary.Direct.PeerIdentityID, PeerType: OrganizationIdentityType(summary.Direct.PeerType), PeerName: summary.Direct.PeerName, PeerAvatarURL: optionalFileURL(avatarURLs, summary.Direct.PeerAvatarFileID),
			Preview: summary.Direct.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Direct.PreviewSenderIdentityType), LastMessageAt: summary.Direct.LastMessageAt,
		}
	}
	if summary.Agent != nil {
		var agentRunStatus *AgentRunStatus
		if summary.Agent.AgentRunStatus != nil {
			status := AgentRunStatus(*summary.Agent.AgentRunStatus)
			agentRunStatus = &status
		}
		conversation.Agent = &AgentInboxConversation{
			Title: summary.Agent.Title, AgentIdentityID: summary.Agent.AgentIdentityID, AgentName: summary.Agent.AgentName, AgentAvatarURL: optionalFileURL(avatarURLs, summary.Agent.AgentAvatarFileID),
			Preview: summary.Agent.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Agent.PreviewSenderIdentityType), LastMessageAt: summary.Agent.LastMessageAt, AgentRunStatus: agentRunStatus,
		}
	}
	if summary.Group != nil {
		conversation.Group = &GroupInboxConversation{
			Title: summary.Group.Title, ImageURL: optionalFileURL(avatarURLs, summary.Group.ImageFileID),
			Status: ConversationStatus(summary.Group.Status), Preview: summary.Group.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Group.PreviewSenderIdentityType),
			LastMessageAt: summary.Group.LastMessageAt, MemberCount: summary.Group.MemberCount,
		}
	}
	return conversation
}

// GetInboxConversation 独立读取当前用户可见的会话，不依赖列表筛选或分页。
func (b *DirectBackend) GetInboxConversation(ctx context.Context, meta RequestMeta, conversationID string) (InboxConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return InboxConversation{}, err
	}
	results, err := b.loadInbox.ReadByIDs(ctx, identity, []string{conversationID}, nil)
	if err != nil {
		return InboxConversation{}, inboxReadError(ctx, meta, identity.Organization.ID, err)
	}
	if results[0].Conversation == nil {
		return InboxConversation{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
	}
	conversations, err := b.inboxConversationsFromActions(ctx, meta, identity, []inboxaction.ConversationSummary{*results[0].Conversation})
	if err != nil {
		return InboxConversation{}, err
	}
	return conversations[0], nil
}

// ReadInboxConversations 在每项中区分匹配、筛选外可读及不可用的会话。
func (b *DirectBackend) ReadInboxConversations(ctx context.Context, meta RequestMeta, input ReadInboxConversationsInput) (InboxConversationResults, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return InboxConversationResults{}, err
	}
	results, err := b.loadInbox.ReadByIDs(ctx, identity, input.ConversationIDs, &inboxaction.LoadInput{Scope: domain.InboxScope(input.Query.Scope), CustomerView: domain.CustomerInboxView(input.Query.CustomerView), AssigneeIdentityID: input.Query.AssigneeIdentityID})
	if err != nil {
		return InboxConversationResults{}, inboxReadError(ctx, meta, identity.Organization.ID, err)
	}
	summaries := make([]inboxaction.ConversationSummary, 0, len(results))
	for _, result := range results {
		if result.Conversation != nil {
			summaries = append(summaries, *result.Conversation)
		}
	}
	conversations, err := b.inboxConversationsFromActions(ctx, meta, identity, summaries)
	if err != nil {
		return InboxConversationResults{}, err
	}
	output := InboxConversationResults{Results: make([]InboxConversationResult, 0, len(results))}
	readable := 0
	for _, result := range results {
		item := InboxConversationResult{ID: result.ID, Availability: InboxConversationUnavailable}
		if result.Conversation != nil {
			item.Conversation = &conversations[readable]
			readable++
			item.Availability = InboxConversationOutsideQuery
			if result.MatchesQuery {
				item.Availability = InboxConversationMatching
			}
		}
		output.Results = append(output.Results, item)
	}
	return output, nil
}

// inboxReadError 将会话摘要查询错误转换为本地化应用服务错误。
func inboxReadError(ctx context.Context, meta RequestMeta, organizationID string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, inboxaction.ErrQueryInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("读取独立会话摘要失败", "organization_id", organizationID, "error", err)
	return FailedError(meta, cervii18n.ErrorInboxLoadFailed)
}
