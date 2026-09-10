//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// inboxOps 持有收件箱与客户投递的 Action 和 Query。
type inboxOps struct {
	customerDeliveries           *deliveryaction.Manager
	loadInbox                    *inboxaction.LoadInboxQuery
	listCustomerServiceAssignees *inboxaction.ListCustomerServiceAssigneesQuery
}

// newInboxOps 创建收件箱与客户投递的业务实现依赖。
func newInboxOps(db *bun.DB, taskEnqueuer servertask.TxEnqueuer) inboxOps {
	return inboxOps{
		customerDeliveries:           deliveryaction.NewManager(db, taskEnqueuer),
		loadInbox:                    inboxaction.NewLoadInboxQuery(db),
		listCustomerServiceAssignees: inboxaction.NewListCustomerServiceAssigneesQuery(db),
	}
}

// LoadInbox 返回当前企业的统一会话工作队列。
func (o *directOperations) LoadInbox(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input LoadInboxInput) (Inbox, error) {
	page, unreadCounts, err := o.loadInbox.Execute(ctx, identity, inboxaction.LoadInput{Scope: domain.InboxScope(input.Scope), CustomerView: domain.CustomerInboxView(input.CustomerView), AssigneeIdentityID: input.AssigneeIdentityID, Cursor: input.Cursor, BeforeCursor: input.BeforeCursor, Limit: input.Limit})
	if err != nil {
		return Inbox{}, inboxReadError(ctx, meta, identity.Organization.ID, "列表", err)
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, page.Conversations)
	if err != nil {
		return Inbox{}, err
	}
	return Inbox{StartCursor: page.StartCursor, EndCursor: page.EndCursor, HasBefore: page.HasBefore, Conversations: conversations, NextCursor: page.NextCursor, HasMore: page.HasMore, UnreadCount: unreadCounts.Unread, AttentionUnreadCount: unreadCounts.Attention}, nil
}

// inboxConversationsFromActions 为会话摘要统一解析头像并转换传输契约。
func (o *directOperations) inboxConversationsFromActions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, summaries []inboxaction.ConversationSummary) ([]InboxConversation, error) {
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
	avatarURLs, err := o.activeFileURLs(ctx, identity, avatarFileIDs)
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
func (o *directOperations) ListCustomerServiceAssignees(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (CustomerServiceAssigneeList, error) {
	items, err := o.listCustomerServiceAssignees.Execute(ctx, identity)
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
	avatarURLs, err := o.activeFileURLs(ctx, identity, avatarFileIDs)
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
	conversation := InboxConversation{PositionCursor: summary.PositionCursor, ID: summary.ID, LastActivityAt: summary.LastActivityAt, Type: ConversationType(summary.Type), UnreadCount: summary.UnreadCount, MentionedUnreadCount: summary.MentionedUnreadCount, Muted: summary.Muted, MarkedUnread: summary.MarkedUnread, LastMessageID: summary.LastMessageID, LastMessageType: (*MessageType)(summary.LastMessageType), LastReadMessageID: summary.LastReadMessageID}
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

// GetInboxConversation 按编号读取当前用户可见的独立会话摘要。
func (o *directOperations) GetInboxConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (InboxConversation, error) {
	results, err := o.loadInbox.ReadByIDs(ctx, identity, []string{conversationID}, nil)
	if err != nil {
		return InboxConversation{}, inboxReadError(ctx, meta, identity.Organization.ID, "独立摘要", err)
	}
	if results[0].Conversation == nil {
		return InboxConversation{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, []inboxaction.ConversationSummary{*results[0].Conversation})
	if err != nil {
		return InboxConversation{}, err
	}
	return conversations[0], nil
}

// ReadInboxConversations 在每项中区分匹配、筛选外可读及不可用的会话。
func (o *directOperations) ReadInboxConversations(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ReadInboxConversationsInput) (InboxConversationResults, error) {
	results, err := o.loadInbox.ReadByIDs(ctx, identity, input.ConversationIDs, &inboxaction.LoadInput{Scope: domain.InboxScope(input.Query.Scope), CustomerView: domain.CustomerInboxView(input.Query.CustomerView), AssigneeIdentityID: input.Query.AssigneeIdentityID})
	if err != nil {
		return InboxConversationResults{}, inboxReadError(ctx, meta, identity.Organization.ID, "批量摘要", err)
	}
	summaries := make([]inboxaction.ConversationSummary, 0, len(results))
	for _, result := range results {
		if result.Conversation != nil {
			summaries = append(summaries, *result.Conversation)
		}
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, summaries)
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

// inboxReadError 统一转换收件箱读取错误，并记录失败的查询入口。
func inboxReadError(ctx context.Context, meta RequestMeta, organizationID, operation string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, inboxaction.ErrCursorInvalid) {
		return InvalidError(meta, cervii18n.ErrorInboxCursorInvalid, nil).WithReason("inbox_cursor_invalid")
	}
	if errors.Is(err, inboxaction.ErrQueryInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("读取收件箱失败", "organization_id", organizationID, "operation", operation, "error", err)
	return FailedError(meta, cervii18n.ErrorInboxLoadFailed)
}
