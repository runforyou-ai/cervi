//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strconv"
	"strings"

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
	listServiceQueueTeams        *inboxaction.ListServiceQueueTeamsQuery
}

// newInboxOps 创建收件箱与客户投递的业务实现依赖。
func newInboxOps(db *bun.DB, taskEnqueuer servertask.TxEnqueuer) inboxOps {
	return inboxOps{
		customerDeliveries:           deliveryaction.NewManager(db, taskEnqueuer),
		loadInbox:                    inboxaction.NewLoadInboxQuery(db),
		listCustomerServiceAssignees: inboxaction.NewListCustomerServiceAssigneesQuery(db),
		listServiceQueueTeams:        inboxaction.NewListServiceQueueTeamsQuery(db),
	}
}

// inboxLoadInput 把传输契约中的会话筛选转换为收件箱查询条件。
func inboxLoadInput(query InboxQuery) inboxaction.LoadInput {
	kinds := make([]domain.ConversationType, 0, len(query.Kinds))
	for _, kind := range query.Kinds {
		kinds = append(kinds, domain.ConversationType(kind))
	}
	return inboxaction.LoadInput{
		Partition: domain.InboxPartition(query.Partition), Scope: domain.InboxScope(query.Scope),
		PendingKind: domain.InboxPendingKind(query.PendingKind), QueueFilter: domain.CustomerQueueFilter(query.QueueFilter), QueueTeamID: query.QueueTeamID,
		ChannelID: query.ChannelID, Audience: domain.ServiceAudience(query.Audience), ServiceStatus: domain.ServiceSessionStatus(query.ServiceStatus),
		AssigneeFilter: domain.InboxAssigneeFilter(query.AssigneeFilter), AssigneeIdentityID: query.AssigneeIdentityID, Kinds: kinds,
		Search: query.Search, SearchRange: inboxaction.SearchRange(query.SearchRange),
	}
}

// LoadInbox 返回当前企业的统一会话工作队列。
func (o *directOperations) LoadInbox(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input LoadInboxInput) (Inbox, error) {
	loadInput := inboxLoadInput(input.query())
	loadInput.Cursor, loadInput.BeforeCursor, loadInput.Limit = input.Cursor, input.BeforeCursor, input.Limit
	page, unreadCounts, err := o.loadInbox.Execute(ctx, identity, loadInput)
	if err != nil {
		return Inbox{}, inboxReadError(ctx, meta, identity.Organization.ID, "列表", err)
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, page.Conversations)
	if err != nil {
		return Inbox{}, err
	}
	return Inbox{
		StartCursor: page.StartCursor, EndCursor: page.EndCursor, HasBefore: page.HasBefore,
		PinOrderVersion: strconv.FormatInt(page.PinOrderVersion, 10), Conversations: conversations,
		NextCursor: page.NextCursor, HasMore: page.HasMore,
		UnreadCount: unreadCounts.Unread, AttentionUnreadCount: unreadCounts.Attention,
		PendingCount: unreadCounts.Pending, PendingUnreadCount: unreadCounts.PendingUnread,
	}, nil
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

// ListServiceQueueTeams 返回可作为客服队列的团队，本人所在团队排在前面。
func (o *directOperations) ListServiceQueueTeams(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (ServiceQueueTeamList, error) {
	items, err := o.listServiceQueueTeams.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return ServiceQueueTeamList{}, ctx.Err()
		}
		slog.Warn("读取客服队列团队失败", "organization_id", identity.Organization.ID, "error", err)
		return ServiceQueueTeamList{}, FailedError(meta, cervii18n.ErrorTeamListFailed)
	}
	teams := make([]ServiceQueueTeam, 0, len(items))
	for _, item := range items {
		teams = append(teams, ServiceQueueTeam{ID: item.ID, Name: item.Name, Mine: item.Mine, Available: item.Available})
	}
	return ServiceQueueTeamList{Teams: teams}, nil
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
	conversation := InboxConversation{PositionCursor: summary.PositionCursor, ID: summary.ID, LastActivityAt: summary.LastActivityAt, Type: ConversationType(summary.Type), UnreadCount: summary.UnreadCount, MentionedUnreadCount: summary.MentionedUnreadCount, Muted: summary.Muted, MarkedUnread: summary.MarkedUnread, Pinned: summary.Pinned, LastMessageID: summary.LastMessageID, LastMessageType: (*MessageType)(summary.LastMessageType), LastReadMessageID: summary.LastReadMessageID}
	if summary.Pending != nil {
		conversation.Pending = &InboxPendingItem{Kind: InboxPendingKind(summary.Pending.Kind), Since: summary.Pending.Since, Mentioned: summary.Pending.Mentioned}
	}
	if summary.Customer != nil {
		var assignee *InboxAssignee
		if summary.Customer.Assignee != nil {
			assignee = &InboxAssignee{IdentityID: summary.Customer.Assignee.IdentityID, Type: OrganizationIdentityType(summary.Customer.Assignee.Type), DisplayName: summary.Customer.Assignee.DisplayName, AvatarURL: optionalFileURL(avatarURLs, summary.Customer.Assignee.AvatarFileID)}
		}
		// 不支持外发附件时不给出字节上限，客户端据此关闭入口。
		attachmentSupported := domain.ChannelSupportsOutboundAttachment(summary.Customer.ChannelType)
		attachmentByteLimit := int64(0)
		if attachmentSupported {
			attachmentByteLimit = domain.ChannelAttachmentLimit(summary.Customer.ChannelType)
		}
		conversation.Customer = &CustomerInboxConversation{
			Title: summary.Customer.Title, ContactName: summary.Customer.ContactName, ContactChatSubjectID: summary.Customer.ContactChatSubjectID,
			AssigneeChatSubjectID: summary.Customer.AssigneeChatSubjectID,
			ContactAvatarURL:      optionalFileURL(avatarURLs, summary.Customer.ContactAvatarFileID),
			ChannelType:           ChannelType(summary.Customer.ChannelType), ChannelName: summary.Customer.ChannelName,
			Preview: summary.Customer.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Customer.PreviewSenderIdentityType),
			PreviewVisibility: (*MessageVisibility)(summary.Customer.PreviewVisibility), LastMessageAt: summary.Customer.LastMessageAt,
			ServiceSessionID: summary.Customer.ServiceSessionID, ServiceSessionStatus: ServiceSessionStatus(summary.Customer.ServiceSessionStatus), Assignee: assignee,
			TeamID: summary.Customer.TeamID, TeamName: summary.Customer.TeamName,
			AttachmentSupported:    attachmentSupported,
			AttachmentByteLimit:    attachmentByteLimit,
			AttachmentCaptionLimit: domain.ChannelCaptionLimit(summary.Customer.ChannelType),
			UnansweredMentionCount: summary.Customer.UnansweredMentionCount,
		}
	}
	if summary.Direct != nil {
		conversation.Direct = &DirectInboxConversation{
			PeerIdentityID: summary.Direct.PeerIdentityID, PeerType: OrganizationIdentityType(summary.Direct.PeerType), PeerName: summary.Direct.PeerName, PeerAvatarURL: optionalFileURL(avatarURLs, summary.Direct.PeerAvatarFileID), PeerStatus: UserStatus(summary.Direct.PeerStatus), PeerWorkStatus: WorkStatus(summary.Direct.PeerWorkStatus),
			Preview: summary.Direct.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Direct.PreviewSenderIdentityType), LastMessageAt: summary.Direct.LastMessageAt,
		}
	}
	if summary.Agent != nil {
		var agentRunStatus *AgentRunStatus
		if summary.Agent.AgentRunStatus != nil {
			status := AgentRunStatus(*summary.Agent.AgentRunStatus)
			agentRunStatus = &status
		}
		// 只有助理携带在线状态。
		var assistantPresence *AssistantPresence
		if summary.Agent.AssistantPresence != "" {
			presence := AssistantPresence(summary.Agent.AssistantPresence)
			assistantPresence = &presence
		}
		conversation.Agent = &AgentInboxConversation{
			Title: summary.Agent.Title, AgentIdentityID: summary.Agent.AgentIdentityID, AgentName: summary.Agent.AgentName, AgentAvatarURL: optionalFileURL(avatarURLs, summary.Agent.AgentAvatarFileID), AgentStatus: UserStatus(summary.Agent.AgentStatus),
			AgentType: OrganizationIdentityType(summary.Agent.AgentType), AssistantPresence: assistantPresence,
			Preview: summary.Agent.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Agent.PreviewSenderIdentityType), LastMessageAt: summary.Agent.LastMessageAt, AgentRunStatus: agentRunStatus,
		}
	}
	if summary.Group != nil {
		conversation.Group = &GroupInboxConversation{
			Title: summary.Group.Title, ImageURL: optionalFileURL(avatarURLs, summary.Group.ImageFileID),
			Status: ConversationStatus(summary.Group.Status), Preview: summary.Group.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.Group.PreviewSenderIdentityType),
			LastMessageAt: summary.Group.LastMessageAt, MemberCount: summary.Group.MemberCount,
			MemberPreviewNames: summary.Group.MemberPreviewNames,
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

// ReadConversationAttention 读取会话摘要及已知消息之后计入本人提醒的未读消息。
func (o *directOperations) ReadConversationAttention(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationAttentionInput) (ConversationAttention, error) {
	attention, err := o.loadInbox.ReadAttention(ctx, identity, conversationID, input.AfterMessageID)
	if err != nil {
		return ConversationAttention{}, inboxReadError(ctx, meta, identity.Organization.ID, "提醒消息", err)
	}
	if attention == nil {
		return ConversationAttention{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, []inboxaction.ConversationSummary{attention.Conversation})
	if err != nil {
		return ConversationAttention{}, err
	}
	output := ConversationAttention{Conversation: conversations[0], Messages: make([]ConversationAttentionMessage, 0, len(attention.Messages))}
	for _, message := range attention.Messages {
		output.Messages = append(output.Messages, ConversationAttentionMessage{
			ID: message.ID, Type: MessageType(message.Type), Visibility: MessageVisibility(message.Visibility), Body: message.Body,
			AttachmentName: message.AttachmentName, SenderName: message.SenderName, SenderIdentityType: (*OrganizationIdentityType)(message.SenderIdentityType),
		})
	}
	return output, nil
}

// ReadInboxConversations 在每项中区分匹配、筛选外可读及不可用的会话。
func (o *directOperations) ReadInboxConversations(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ReadInboxConversationsInput) (InboxConversationResults, error) {
	// 未指定范围与搜索词时只核对阅读资格。
	var query *inboxaction.LoadInput
	if input.Query.Scope != "" || input.Query.Search != "" {
		loadInput := inboxLoadInput(input.Query)
		query = &loadInput
	}
	results, err := o.loadInbox.ReadByIDs(ctx, identity, input.ConversationIDs, query)
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

// ListInboxChannels 返回渠道筛选候选，包含已停用渠道。
func (o *directOperations) ListInboxChannels(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (InboxChannelList, error) {
	records, err := o.listMessageChannels.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return InboxChannelList{}, ctx.Err()
		}
		slog.Warn("读取收件箱渠道候选失败", "organization_id", identity.Organization.ID, "error", err)
		return InboxChannelList{}, FailedError(meta, cervii18n.ErrorChannelListFailed)
	}
	channels := make([]InboxChannel, 0, len(records))
	for _, record := range records {
		channels = append(channels, InboxChannel{ID: record.ID, Type: ChannelType(record.Type), Name: record.Name, Enabled: record.Enabled})
	}
	// 候选按渠道类型的既定顺序排列，同类型内按名称排序。
	order := domain.MessageChannelTypes()
	slices.SortStableFunc(channels, func(left, right InboxChannel) int {
		if position := slices.Index(order, domain.ChannelType(left.Type)) - slices.Index(order, domain.ChannelType(right.Type)); position != 0 {
			return position
		}
		return strings.Compare(left.Name, right.Name)
	})
	return InboxChannelList{Channels: channels}, nil
}

// GetSyncHeads 返回当前用户可见会话数量、版本校验和与身份资料版本。
func (o *directOperations) GetSyncHeads(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (SyncHeads, error) {
	heads, err := o.loadInbox.SyncHeads(ctx, identity)
	if err != nil {
		return SyncHeads{}, inboxReadError(ctx, meta, identity.Organization.ID, "同步探针", err)
	}
	return SyncHeads{
		ConversationCount: heads.ConversationCount, ConversationChecksum: heads.ConversationChecksum,
		IdentityProfileVersion: strconv.FormatInt(heads.IdentityProfileVersion, 10),
		PinOrderVersion:        strconv.FormatInt(heads.PinOrderVersion, 10),
	}, nil
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

// SearchInbox 按范围检索会话、消息和人员，并统一解析会话图片与人员头像。
func (o *directOperations) SearchInbox(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input InboxSearchInput) (InboxSearchResult, error) {
	list := inboxLoadInput(InboxQuery{
		Scope: input.Scope, PendingKind: input.PendingKind, QueueFilter: input.QueueFilter, QueueTeamID: input.QueueTeamID,
		ChannelID: input.ChannelID, Audience: input.Audience, ServiceStatus: input.ServiceStatus,
		AssigneeFilter: input.AssigneeFilter, AssigneeIdentityID: input.AssigneeIdentityID, Kinds: input.Kinds,
	})
	result, err := o.loadInbox.Search(ctx, identity, inboxaction.SearchInput{
		Text: input.Query, Range: inboxaction.SearchRange(input.Range), List: list, ConversationID: input.ConversationID,
	})
	if err != nil {
		if ctx.Err() != nil {
			return InboxSearchResult{}, ctx.Err()
		}
		if errors.Is(err, inboxaction.ErrConversationUnavailable) {
			return InboxSearchResult{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
		}
		if errors.Is(err, inboxaction.ErrQueryInvalid) {
			return InboxSearchResult{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
		}
		slog.Warn("检索收件箱失败", "organization_id", identity.Organization.ID, "range", input.Range, "error", err)
		return InboxSearchResult{}, FailedError(meta, cervii18n.ErrorInboxSearchFailed)
	}
	// 会话结果与各条消息的所在会话共用一次图片解析，转换后按原顺序拆回。
	summaries := slices.Clone(result.Conversations)
	for _, message := range result.Messages {
		summaries = append(summaries, message.Conversation)
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, summaries)
	if err != nil {
		return InboxSearchResult{}, err
	}
	avatarFileIDs := make([]string, 0, len(result.People))
	for _, person := range result.People {
		if person.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *person.AvatarFileID)
		}
	}
	avatarURLs, err := o.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		slog.Warn("读取检索人员头像失败", "organization_id", identity.Organization.ID, "error", err)
		return InboxSearchResult{}, FailedError(meta, cervii18n.ErrorInboxSearchFailed)
	}
	output := InboxSearchResult{
		Conversations: conversations[:len(result.Conversations)],
		Messages:      make([]InboxSearchMessage, 0, len(result.Messages)),
		People:        make([]InboxSearchPerson, 0, len(result.People)),
	}
	for index, message := range result.Messages {
		excerpt := make([]InboxSearchSegment, 0, len(message.Excerpt))
		for _, segment := range message.Excerpt {
			excerpt = append(excerpt, InboxSearchSegment{Text: segment.Text, Match: segment.Match})
		}
		output.Messages = append(output.Messages, InboxSearchMessage{
			ID: message.ID, Type: MessageType(message.Type), SenderName: message.SenderName, OriginatedAt: message.OriginatedAt,
			Excerpt: excerpt, Conversation: conversations[len(result.Conversations)+index],
		})
	}
	for _, person := range result.People {
		item := InboxSearchPerson{
			Kind: InboxSearchPersonKind(person.Kind), ID: person.ID, UserID: person.UserID, AgentID: person.AgentID, DisplayName: person.DisplayName,
			AvatarURL: optionalFileURL(avatarURLs, person.AvatarFileID), ConversationID: person.ConversationID,
		}
		if person.Kind == inboxaction.SearchPersonMember {
			identityType := OrganizationIdentityType(person.IdentityType)
			item.IdentityType = &identityType
		}
		output.People = append(output.People, item)
	}
	return output, nil
}
