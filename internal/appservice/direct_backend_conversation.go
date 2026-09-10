//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// conversationOps 持有会话与消息的 Action 和 Query。
type conversationOps struct {
	sendAttachmentMessage           *conversationaction.SendAttachmentMessageAction
	listConversationMessages        *conversationaction.ListConversationMessagesQuery
	updateConversationUnreadMark    *conversationaction.UpdateConversationUnreadMarkAction
	markConversationRead            *conversationaction.MarkConversationReadAction
	conversationNavigation          *conversationaction.GetConversationNavigationStateQuery
	pendingConversationMentions     *conversationaction.ListPendingConversationMentionsQuery
	reviewConversationMention       *conversationaction.MarkConversationMentionReviewedAction
	updateConversationNotifications *conversationaction.UpdateConversationNotificationSettingsAction
	sendCustomerTextMessage         *conversationaction.SendCustomerTextMessageAction
	claimServiceSession             *conversationaction.ClaimServiceSessionAction
	transferServiceSession          *conversationaction.TransferServiceSessionAction
	closeServiceSession             *conversationaction.CloseServiceSessionAction
	reopenServiceSession            *conversationaction.ReopenServiceSessionAction
	sendFirstAgentTextMessage       *conversationaction.SendFirstAgentTextMessageAction
	sendAgentTextMessage            *conversationaction.SendAgentTextMessageAction
	sendFirstDirectTextMessage      *conversationaction.SendFirstDirectTextMessageAction
	findDirectConversation          *conversationaction.FindDirectConversationQuery
	sendDirectTextMessage           *conversationaction.SendDirectTextMessageAction
	createGroupConversation         *conversationaction.CreateGroupConversationAction
	getGroupConversation            *conversationaction.GetGroupConversationQuery
	updateGroupConversation         *conversationaction.UpdateGroupConversationAction
	addGroupConversationMembers     *conversationaction.AddGroupConversationMembersAction
	removeGroupConversationMember   *conversationaction.RemoveGroupConversationMemberAction
	transferGroupConversationOwner  *conversationaction.TransferGroupConversationOwnerAction
	leaveGroupConversation          *conversationaction.LeaveGroupConversationAction
	dissolveGroupConversation       *conversationaction.DissolveGroupConversationAction
	sendGroupTextMessage            *conversationaction.SendGroupTextMessageAction
}

// newConversationOps 创建会话与消息的业务实现依赖。
func newConversationOps(db *bun.DB, agentScheduler conversationaction.AgentMessageScheduler, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer) conversationOps {
	return conversationOps{
		sendAttachmentMessage:           conversationaction.NewSendAttachmentMessageAction(db),
		listConversationMessages:        conversationaction.NewListConversationMessagesQuery(db),
		updateConversationUnreadMark:    conversationaction.NewUpdateConversationUnreadMarkAction(db),
		markConversationRead:            conversationaction.NewMarkConversationReadAction(db),
		conversationNavigation:          conversationaction.NewGetConversationNavigationStateQuery(db),
		pendingConversationMentions:     conversationaction.NewListPendingConversationMentionsQuery(db),
		reviewConversationMention:       conversationaction.NewMarkConversationMentionReviewedAction(db),
		updateConversationNotifications: conversationaction.NewUpdateConversationNotificationSettingsAction(db),
		sendCustomerTextMessage:         conversationaction.NewSendCustomerTextMessageAction(db, taskEnqueuer),
		claimServiceSession:             conversationaction.NewClaimServiceSessionAction(db, agentCoordinator),
		transferServiceSession:          conversationaction.NewTransferServiceSessionAction(db, agentCoordinator, agentScheduler),
		closeServiceSession:             conversationaction.NewCloseServiceSessionAction(db, agentCoordinator),
		reopenServiceSession:            conversationaction.NewReopenServiceSessionAction(db),
		sendFirstAgentTextMessage:       conversationaction.NewSendFirstAgentTextMessageAction(db, agentScheduler),
		sendAgentTextMessage:            conversationaction.NewSendAgentTextMessageAction(db, agentScheduler),
		sendFirstDirectTextMessage:      conversationaction.NewSendFirstDirectTextMessageAction(db),
		findDirectConversation:          conversationaction.NewFindDirectConversationQuery(db),
		sendDirectTextMessage:           conversationaction.NewSendDirectTextMessageAction(db),
		createGroupConversation:         conversationaction.NewCreateGroupConversationAction(db),
		getGroupConversation:            conversationaction.NewGetGroupConversationQuery(db),
		updateGroupConversation:         conversationaction.NewUpdateGroupConversationAction(db),
		addGroupConversationMembers:     conversationaction.NewAddGroupConversationMembersAction(db),
		removeGroupConversationMember:   conversationaction.NewRemoveGroupConversationMemberAction(db, agentCoordinator),
		transferGroupConversationOwner:  conversationaction.NewTransferGroupConversationOwnerAction(db),
		leaveGroupConversation:          conversationaction.NewLeaveGroupConversationAction(db),
		dissolveGroupConversation:       conversationaction.NewDissolveGroupConversationAction(db, agentCoordinator),
		sendGroupTextMessage:            conversationaction.NewSendGroupTextMessageAction(db, agentScheduler),
	}
}

// SendCustomerTextMessage 发送成员客户会话文本消息。
func (o *directOperations) SendCustomerTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input CustomerTextMessageInput) (ConversationMessage, error) {
	message, err := o.sendCustomerTextMessage.Execute(ctx, identity, conversationaction.CustomerTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	if err != nil {
		return ConversationMessage{}, customerTextMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("成员客户文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// ClaimServiceSession 领取或接管客户会话最新处理周期。
func (o *directOperations) ClaimServiceSession(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (CustomerServiceSession, error) {
	result, err := o.claimServiceSession.Execute(ctx, identity, conversationID)
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// TransferServiceSession 把当前负责的处理周期转给另一位客服。
func (o *directOperations) TransferServiceSession(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input TransferServiceSessionInput) (CustomerServiceSession, error) {
	result, err := o.transferServiceSession.Execute(ctx, identity, conversationaction.TransferServiceSessionInput{ConversationID: conversationID, AssigneeIdentityID: input.AssigneeIdentityID})
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// CloseServiceSession 关闭客户会话最新处理周期。
func (o *directOperations) CloseServiceSession(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (CustomerServiceSession, error) {
	result, err := o.closeServiceSession.Execute(ctx, identity, conversationID)
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// ReopenServiceSession 重新打开客户会话最新处理周期并分配给当前身份。
func (o *directOperations) ReopenServiceSession(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (CustomerServiceSession, error) {
	result, err := o.reopenServiceSession.Execute(ctx, identity, conversationID)
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// customerServiceSessionFromAction 转换客服处理周期命令结果。
func customerServiceSessionFromAction(result conversationaction.ServiceSessionResult) CustomerServiceSession {
	var assignee *InboxAssignee
	if result.Assignee != nil {
		assignee = &InboxAssignee{IdentityID: result.Assignee.IdentityID, Type: OrganizationIdentityType(result.Assignee.Type), DisplayName: result.Assignee.DisplayName}
	}
	return CustomerServiceSession{ID: result.ID, Status: ServiceSessionStatus(result.Status), Assignee: assignee, ClosedAt: result.ClosedAt}
}

// serviceSessionMutationError 转换客服处理周期命令错误。
func serviceSessionMutationError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		messageKey := cervii18n.ErrorServiceSessionNotReplyable
		switch conflictError.Reason {
		case conversationaction.ConflictReasonServiceSessionOwned:
			messageKey = cervii18n.ErrorServiceSessionOwned
		case conversationaction.ConflictReasonServiceSessionAlreadyOpen:
			messageKey = cervii18n.ErrorServiceSessionAlreadyOpen
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("客服处理周期操作失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorServiceSessionUpdateFailed)
}

// SendFirstDirectTextMessage 发送首条单聊消息并按需创建长期会话。
func (o *directOperations) SendFirstDirectTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input FirstDirectTextMessageInput) (FirstDirectTextMessageResult, error) {
	result, err := o.sendFirstDirectTextMessage.Execute(ctx, identity, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: input.TargetIdentityID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return FirstDirectTextMessageResult{}, individualConversationError(ctx, meta, err, identity.Organization.ID, input.TargetIdentityID, "send_first")
	}
	slog.Info("企业成员内部单聊首条文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", result.Conversation.ID,
		"target_identity_id", result.Conversation.PeerIdentityID,
		"message_id", result.Message.ID,
	)
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{result.Message}, result.Conversation.PeerAvatarFileID)
	if err != nil {
		slog.Warn("读取已保存单聊首条消息头像失败", "organization_id", identity.Organization.ID, "conversation_id", result.Conversation.ID, "message_id", result.Message.ID, "error", err)
	}
	return FirstDirectTextMessageResult{
		Conversation: directInboxConversationFromSummary(result.Conversation, avatarURLs),
		Message:      conversationMessageFromAction(result.Message, avatarURLs),
	}, nil
}

// FindDirectConversation 按目标身份查找当前成员的活跃单聊。
func (o *directOperations) FindDirectConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, targetIdentityID string) (DirectConversationLookup, error) {
	summary, err := o.findDirectConversation.Execute(ctx, identity, targetIdentityID)
	if err != nil {
		return DirectConversationLookup{}, individualConversationError(ctx, meta, err, identity.Organization.ID, targetIdentityID, "find")
	}
	if summary == nil {
		return DirectConversationLookup{}, nil
	}
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, nil, summary.PeerAvatarFileID)
	if err != nil {
		return DirectConversationLookup{}, individualConversationError(ctx, meta, err, identity.Organization.ID, targetIdentityID, "find")
	}
	conversation := directInboxConversationFromSummary(*summary, avatarURLs)
	return DirectConversationLookup{Conversation: &conversation}, nil
}

// directInboxConversationFromSummary 把单聊摘要转换为统一收件箱会话。
func directInboxConversationFromSummary(summary conversationaction.DirectConversationSummary, avatarURLs map[string]string) InboxConversation {
	return InboxConversation{
		ID: summary.ID, Type: ConversationTypeDirect, LastActivityAt: summary.LastActivityAt,
		Direct: &DirectInboxConversation{
			PeerIdentityID: summary.PeerIdentityID, PeerType: OrganizationIdentityType(summary.PeerType), PeerName: summary.PeerName, PeerAvatarURL: optionalFileURL(avatarURLs, summary.PeerAvatarFileID),
			Preview: summary.Preview, PreviewSenderIdentityType: (*OrganizationIdentityType)(summary.PreviewSenderIdentityType), LastMessageAt: summary.LastMessageAt,
		},
	}
}

// SendDirectTextMessage 发送内部单聊文本消息。
func (o *directOperations) SendDirectTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input DirectTextMessageInput) (ConversationMessage, error) {
	message, err := o.sendDirectTextMessage.Execute(ctx, identity, conversationaction.InternalTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
	})
	if err != nil {
		return ConversationMessage{}, individualConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send")
	}
	slog.Info("企业成员内部单聊文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// CreateGroupConversation 创建包含有效企业成员的企业内部群聊。
func (o *directOperations) CreateGroupConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input GroupConversationInput) (InboxConversation, error) {
	summary, err := o.createGroupConversation.Execute(ctx, identity, conversationaction.GroupConversationInput{
		Title: input.Title, Description: input.Description, ImageFileID: input.ImageFileID,
		MemberIdentityIDs: input.MemberIdentityIDs,
	})
	if err != nil {
		return InboxConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, "", "create")
	}
	slog.Info("企业内部群聊已创建",
		"organization_id", identity.Organization.ID,
		"conversation_id", summary.ID,
		"member_count", summary.MemberCount,
	)
	imageFileIDs := make([]string, 0, 1)
	if summary.ImageFileID != nil {
		imageFileIDs = append(imageFileIDs, *summary.ImageFileID)
	}
	imageURLs, imageErr := o.activeFileURLs(ctx, identity, imageFileIDs)
	if imageErr != nil {
		slog.Warn("读取新建群聊图片失败", "organization_id", identity.Organization.ID, "conversation_id", summary.ID, "error", imageErr)
	}
	return InboxConversation{
		ID: summary.ID, Type: ConversationTypeGroup,
		Group: &GroupInboxConversation{
			Title: summary.Title, ImageURL: optionalFileURL(imageURLs, summary.ImageFileID),
			Status: ConversationStatus(summary.Status), MemberCount: summary.MemberCount,
		},
	}, nil
}

// GetGroupConversation 返回当前成员可见的群聊资料和有效成员。
func (o *directOperations) GetGroupConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (GroupConversation, error) {
	record, err := o.getGroupConversation.Execute(ctx, identity, conversationID)
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "get")
	}
	result, err := o.groupConversationFromAction(ctx, identity, record)
	if err != nil {
		slog.Warn("读取群聊图片或成员头像失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return GroupConversation{}, FailedError(meta, cervii18n.ErrorGroupConversationReadFailed)
	}
	return result, nil
}

// UpdateGroupConversation 修改群聊资料。
func (o *directOperations) UpdateGroupConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input GroupConversationProfileInput) (GroupConversation, error) {
	record, err := o.updateGroupConversation.Execute(ctx, identity, conversationaction.GroupConversationProfileInput{
		ConversationID: conversationID, Title: input.Title, Description: input.Description, ImageFileID: input.ImageFileID,
	})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "update")
	}
	slog.Info("企业群聊资料已修改", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID)
	return o.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// AddGroupConversationMembers 批量增加群聊成员。
func (o *directOperations) AddGroupConversationMembers(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input GroupConversationMembersInput) (GroupConversation, error) {
	record, err := o.addGroupConversationMembers.Execute(ctx, identity, conversationaction.GroupConversationMembersInput{ConversationID: conversationID, MemberIdentityIDs: input.MemberIdentityIDs})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "add_members")
	}
	slog.Info("企业群聊成员已增加", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "added_count", len(input.MemberIdentityIDs))
	return o.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// RemoveGroupConversationMember 移除单个群聊成员。
func (o *directOperations) RemoveGroupConversationMember(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input GroupConversationMemberInput) (GroupConversation, error) {
	record, err := o.removeGroupConversationMember.Execute(ctx, identity, conversationaction.GroupConversationMemberInput{ConversationID: conversationID, MemberIdentityID: input.MemberIdentityID})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "remove_member")
	}
	slog.Info("企业群聊成员已移除", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "member_identity_id", input.MemberIdentityID)
	return o.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// TransferGroupConversationOwner 转让群主。
func (o *directOperations) TransferGroupConversationOwner(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input GroupConversationOwnerInput) (GroupConversation, error) {
	record, err := o.transferGroupConversationOwner.Execute(ctx, identity, conversationaction.GroupConversationOwnerInput{ConversationID: conversationID, OwnerIdentityID: input.OwnerIdentityID})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "transfer_owner")
	}
	slog.Info("企业群聊群主已转让", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "owner_identity_id", input.OwnerIdentityID)
	return o.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// LeaveGroupConversation 退出普通成员参与的群聊。
func (o *directOperations) LeaveGroupConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) error {
	err := o.leaveGroupConversation.Execute(ctx, identity, conversationID)
	if err != nil {
		return groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "leave")
	}
	slog.Info("企业群聊退出操作已完成", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID)
	return nil
}

// DissolveGroupConversation 解散群聊并返回只读资料。
func (o *directOperations) DissolveGroupConversation(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (GroupConversation, error) {
	record, err := o.dissolveGroupConversation.Execute(ctx, identity, conversationID)
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "dissolve")
	}
	slog.Info("企业群聊解散操作已完成", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID)
	return o.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// groupConversationMutationResult 转换群聊管理命令结果。
func (o *directOperations) groupConversationMutationResult(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, record conversationaction.GroupConversation, conversationID string) (GroupConversation, error) {
	result, err := o.groupConversationFromAction(ctx, identity, record)
	if err != nil {
		slog.Warn("读取群聊管理结果图片失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return GroupConversation{}, FailedError(meta, cervii18n.ErrorGroupConversationReadFailed)
	}
	return result, nil
}

// groupConversationFromAction 转换群聊资料并生成群图片和成员头像地址。
func (o *directOperations) groupConversationFromAction(ctx context.Context, identity *servermodels.Identity, record conversationaction.GroupConversation) (GroupConversation, error) {
	avatarFileIDs := make([]string, 0, len(record.Participants)+1)
	if record.ImageFileID != nil {
		avatarFileIDs = append(avatarFileIDs, *record.ImageFileID)
	}
	for _, participant := range record.Participants {
		if participant.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *participant.AvatarFileID)
		}
	}
	avatarURLs, err := o.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		return GroupConversation{}, err
	}
	participants := make([]GroupParticipant, 0, len(record.Participants))
	for _, participant := range record.Participants {
		participants = append(participants, GroupParticipant{
			ChatSubjectID: participant.ChatSubjectID, IdentityType: OrganizationIdentityType(participant.IdentityType), IdentityID: participant.IdentityID, DisplayName: participant.DisplayName,
			AvatarURL: optionalFileURL(avatarURLs, participant.AvatarFileID), Role: GroupParticipantRole(participant.Role),
		})
	}
	return GroupConversation{
		ID: record.ID, Title: record.Title, Description: record.Description,
		ImageURL: optionalFileURL(avatarURLs, record.ImageFileID), Status: ConversationStatus(record.Status),
		CreatedAt: record.CreatedAt, Participants: participants,
		Muted: record.Muted,
	}, nil
}

// SendGroupTextMessage 发送企业内部群聊文本消息。
func (o *directOperations) SendGroupTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input GroupTextMessageInput) (ConversationMessage, error) {
	message, err := o.sendGroupTextMessage.Execute(ctx, identity, conversationaction.GroupTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body,
		ReplyToMessageID: input.ReplyToMessageID, MentionSubjectIDs: input.MentionSubjectIDs, MentionAll: input.MentionAll,
	})
	if err != nil {
		return ConversationMessage{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send")
	}
	slog.Info("企业内部群聊文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// ListConversationMessages 返回成员可见的会话消息。
func (o *directOperations) ListConversationMessages(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationMessageListInput) (ConversationMessageList, error) {
	actionInput := conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID}
	if input.Before != "" && input.After != "" {
		return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"cursor": cervii18n.FieldMessageCursorInvalid})
	}
	if input.Before != "" {
		point, valid := decodeConversationMessageCursor(input.Before, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"before": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.Before = &point
	}
	if input.After != "" {
		point, valid := decodeConversationMessageCursor(input.After, conversationID)
		if !valid {
			return ConversationMessageList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"after": cervii18n.FieldMessageCursorInvalid})
		}
		actionInput.After = &point
	}

	history, err := o.listConversationMessages.Execute(ctx, identity, actionInput)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return o.conversationMessageListFromAction(ctx, meta, identity, conversationID, history)
}

// MarkConversationRead 单调推进当前用户的会话已读水位。
func (o *directOperations) MarkConversationRead(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input MarkConversationReadInput) (ConversationReadState, error) {
	state, err := o.markConversationRead.Execute(ctx, identity, conversationID, input.LastReadMessageID, input.ClearUnreadMark)
	if err != nil {
		return ConversationReadState{}, conversationReadError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return ConversationReadState{ReadSeq: strconv.FormatInt(state.ReadSeq, 10), LastReadMessageID: state.LastReadMessageID, LastReadAt: state.LastReadAt}, nil
}

// UpdateConversationUnreadMark 保存个人未读标记并保留已读和提及查看水位。
func (o *directOperations) UpdateConversationUnreadMark(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationUnreadMarkInput) error {
	if err := o.updateConversationUnreadMark.Execute(ctx, identity, conversationID, input.MarkedUnread); err != nil {
		return conversationReadError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	if input.MarkedUnread {
		slog.Info("会话已标为未读", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "user_id", identity.User.ID)
	}
	return nil
}

// UpdateConversationNotificationSettings 保存当前用户的原生会话提醒设置。
func (o *directOperations) UpdateConversationNotificationSettings(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationNotificationSettingsInput) (ConversationNotificationSettings, error) {
	settings, err := o.updateConversationNotifications.Execute(ctx, identity, conversationID, input.Muted)
	if err != nil {
		return ConversationNotificationSettings{}, conversationNotificationSettingsError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("会话提醒设置已保存", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "user_id", identity.User.ID, "muted", settings.Muted)
	return ConversationNotificationSettings{Muted: settings.Muted}, nil
}

// conversationNotificationSettingsError 转换会话提醒设置更新错误。
func conversationNotificationSettingsError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	slog.Warn("更新会话提醒设置失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationNotifyUpdateFailed)
}

// conversationReadError 转换会话阅读状态更新错误。
func conversationReadError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	slog.Warn("更新会话阅读状态失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationReadUpdateFailed)
}

// conversationMessageFromAction 转换成员会话消息契约。
func conversationMessageFromAction(message conversationaction.ConversationMessage, avatarURLs map[string]string) ConversationMessage {
	sender := conversationMessageSenderFromAction(message.Sender, avatarURLs)
	var sessionStart *ConversationMessageSessionStart
	if message.SessionStart != nil {
		sessionStart = &ConversationMessageSessionStart{
			Sequence:  message.SessionStart.Sequence,
			StartedAt: message.SessionStart.StartedAt,
			Status:    ServiceSessionStatus(message.SessionStart.Status),
		}
	}
	var systemEvent *ConversationSystemEvent
	if message.SystemEvent != nil {
		targets := make([]ConversationSystemEventParticipant, 0, len(message.SystemEvent.Targets))
		for _, target := range message.SystemEvent.Targets {
			targets = append(targets, ConversationSystemEventParticipant{IdentityID: target.IdentityID, DisplayName: target.DisplayName})
		}
		systemEvent = &ConversationSystemEvent{
			Type: ConversationSystemEventType(message.SystemEvent.Type),
			Actor: ConversationSystemEventParticipant{
				IdentityID: message.SystemEvent.Actor.IdentityID, DisplayName: message.SystemEvent.Actor.DisplayName,
			},
			Targets: targets, PreviousTitle: message.SystemEvent.PreviousTitle, Title: message.SystemEvent.Title,
		}
	}
	var replyTo *ConversationMessageReference
	if message.ReplyTo != nil {
		replyTo = &ConversationMessageReference{
			ExternalSenderName: message.ReplyTo.ExternalSenderName, ID: message.ReplyTo.ID, Type: MessageType(message.ReplyTo.Type), Body: message.ReplyTo.Body, Deleted: message.ReplyTo.Deleted,
			Sender: conversationMessageSenderFromAction(message.ReplyTo.Sender, avatarURLs),
		}
	}
	mentions := make([]ConversationMessageMention, 0, len(message.Mentions))
	for _, mention := range message.Mentions {
		mentions = append(mentions, ConversationMessageMention{
			ChatSubjectID: mention.ChatSubjectID, Kind: ChatSubjectKind(mention.Kind),
			SourceID: mention.SourceID, DisplayName: mention.DisplayName,
		})
	}
	var attachment *MessageAttachment
	if message.Attachment != nil {
		attachment = &MessageAttachment{File: File{ID: message.Attachment.ID, Name: message.Attachment.Name, ContentType: message.Attachment.ContentType, ByteSize: message.Attachment.ByteSize}, UploadStatus: AttachmentUploadStatus(message.Attachment.UploadStatus), ImageWidth: message.Attachment.ImageWidth, ImageHeight: message.Attachment.ImageHeight}
	}
	return ConversationMessage{
		CanReply:        !message.ReplyUnavailable && (message.Type == domain.MessageTypeText || message.Type == domain.MessageTypeAttachment),
		ClientMessageID: message.ClientMessageID,
		Attachment:      attachment,
		AgentProcess:    conversationAgentProcessFromAction(message.AgentProcess),
		ID:              message.ID, Type: MessageType(message.Type), Body: message.Body,
		OriginatedAt: message.OriginatedAt, SourceOrder: message.SourceOrder, CreatedAt: message.CreatedAt, MessageSeq: strconv.FormatInt(message.MessageSeq, 10),
		Sender: sender, SessionStart: sessionStart, SystemEvent: systemEvent,
		ReplyTo: replyTo, Mentions: mentions, MentionAll: message.MentionAll,
	}
}

// conversationMessageSenderFromAction 转换消息发送主体。
func conversationMessageSenderFromAction(sender *conversationaction.ConversationMessageSender, avatarURLs map[string]string) *ConversationMessageSender {
	if sender == nil {
		return nil
	}
	return &ConversationMessageSender{
		ChatSubjectID: sender.ChatSubjectID, Kind: ChatSubjectKind(sender.Kind),
		SourceID: sender.SourceID, DisplayName: sender.DisplayName,
		AvatarURL: optionalFileURL(avatarURLs, sender.AvatarFileID), IdentityType: (*OrganizationIdentityType)(sender.IdentityType),
	}
}

// individualConversationError 转换真人单聊和 AI 聊天的目标及消息错误。
func individualConversationError(ctx context.Context, meta RequestMeta, err error, organizationID, targetID, operation string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrAgentTargetNotFound) {
		return NotFoundError(meta, cervii18n.ErrorAgentNotFound)
	}
	if errors.Is(err, conversationaction.ErrDirectTargetNotFound) {
		return NotFoundError(meta, cervii18n.ErrorDirectTargetNotFound)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		if conflictError.Reason == conversationaction.ConflictReasonReplyTargetInvalid {
			return ConflictError(meta, cervii18n.ErrorReplyTargetInvalid, conflictError.Reason)
		}
		return ConflictError(meta, cervii18n.ErrorMessageConflict, conflictError.Reason)
	}
	slog.Warn("双方聊天操作失败", "organization_id", organizationID, "target_id", targetID, "operation", operation, "error", err)
	if operation == "find" {
		return FailedError(meta, cervii18n.ErrorDirectConversationLookupFailed)
	}
	return FailedError(meta, cervii18n.ErrorDirectMessageSendFailed)
}

// groupConversationError 转换企业群聊命令错误。
func groupConversationError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID, operation string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		return NotFoundError(meta, cervii18n.ErrorGroupMemberNotFound)
	}
	if errors.Is(err, conversationaction.ErrGroupImageFileNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if errors.Is(err, conversationaction.ErrGroupOwnerRequired) {
		return FailedError(meta, cervii18n.ErrorGroupOwnerRequired).WithStatus(http.StatusForbidden)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		messageKey := cervii18n.ErrorMessageConflict
		switch conflictError.Reason {
		case conversationaction.ConflictReasonGroupMemberAlreadyActive:
			messageKey = cervii18n.ErrorGroupMemberAlreadyActive
		case conversationaction.ConflictReasonGroupMemberNotActive:
			messageKey = cervii18n.ErrorGroupMemberNotActive
		case conversationaction.ConflictReasonGroupOwnerCannotBeRemoved:
			messageKey = cervii18n.ErrorGroupOwnerCannotBeRemoved
		case conversationaction.ConflictReasonGroupOwnerCannotLeave:
			messageKey = cervii18n.ErrorGroupOwnerCannotLeave
		case conversationaction.ConflictReasonReplyTargetInvalid:
			messageKey = cervii18n.ErrorReplyTargetInvalid
		case conversationaction.ConflictReasonGroupMentionTargetInvalid:
			messageKey = cervii18n.ErrorGroupMentionTargetInvalid
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("企业群聊命令失败", "organization_id", organizationID, "conversation_id", conversationID, "operation", operation, "error", err)
	switch operation {
	case "create":
		return FailedError(meta, cervii18n.ErrorGroupConversationCreateFailed)
	case "get":
		return FailedError(meta, cervii18n.ErrorGroupConversationReadFailed)
	case "leave":
		return FailedError(meta, cervii18n.ErrorGroupConversationLeaveFailed)
	case "send":
		return FailedError(meta, cervii18n.ErrorGroupMessageSendFailed)
	default:
		return FailedError(meta, cervii18n.ErrorGroupConversationUpdateFailed)
	}
}

// encodeConversationMessageCursor 编码绑定会话的消息序号与定位编号。
func encodeConversationMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	return conversationID + "." + strconv.FormatInt(point.MessageSeq, 10) + "." + point.ID
}

// decodeConversationMessageCursor 校验消息游标的会话、序号和定位编号。
func decodeConversationMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 || parts[0] != conversationID || !common.ValidUUID(parts[2]) {
		return conversationaction.MessageCursorPoint{}, false
	}
	sequence, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || sequence <= 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	return conversationaction.MessageCursorPoint{ID: parts[2], MessageSeq: sequence}, true
}

// conversationMessageError 转换成员消息读取错误。
func conversationMessageError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound).WithReason("conversation_unavailable")
	}
	if errors.Is(err, conversationaction.ErrMessageUnavailable) {
		return NotFoundError(meta, cervii18n.ErrorConversationMessageUnavailable).WithReason("message_unavailable")
	}
	if errors.Is(err, conversationaction.ErrMentionTargetInvalid) {
		return InvalidError(meta, cervii18n.ErrorConversationMentionTargetInvalid, nil)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	slog.Warn("读取会话消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationMessageListFailed)
}

// customerTextMessageError 转换成员客户消息发送错误。
func customerTextMessageError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		messageKey := cervii18n.ErrorMessageConflict
		switch conflictError.Reason {
		case conversationaction.ConflictReasonServiceSessionOwned:
			messageKey = cervii18n.ErrorServiceSessionOwned
		case conversationaction.ConflictReasonServiceSessionNotReplyable:
			messageKey = cervii18n.ErrorServiceSessionNotReplyable
		case conversationaction.ConflictReasonChannelOutboundUnavailable:
			messageKey = cervii18n.ErrorChannelOutboundUnavailable
		case conversationaction.ConflictReasonChannelOutboundUnsupported:
			messageKey = cervii18n.ErrorChannelOutboundUnsupported
		case conversationaction.ConflictReasonReplyTargetInvalid:
			messageKey = cervii18n.ErrorReplyTargetInvalid
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("发送成员客户消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorMessageSendFailed)
}

var conversationMessageValidationKeys = map[conversationaction.ValidationCode]cervii18n.Key{
	conversationaction.ValidationConversationIDInvalid:    cervii18n.FieldConversationIDInvalid,
	conversationaction.ValidationClientMessageIDInvalid:   cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationLastReadMessageIDInvalid: cervii18n.FieldClientMessageIDInvalid,
	conversationaction.ValidationReplyToMessageIDInvalid:  cervii18n.FieldReplyToMessageIDInvalid,
	conversationaction.ValidationMentionSubjectIDsInvalid: cervii18n.FieldMentionSubjectIDsInvalid,
	conversationaction.ValidationBodyRequired:             cervii18n.FieldMessageBodyRequired,
	conversationaction.ValidationBodyTooLong:              cervii18n.FieldMessageBodyTooLong,
	conversationaction.ValidationCursorInvalid:            cervii18n.FieldMessageCursorInvalid,
	conversationaction.ValidationTargetIdentityIDInvalid:  cervii18n.FieldTargetIdentityIDInvalid,
	conversationaction.ValidationGroupTitleRequired:       cervii18n.FieldGroupTitleRequired,
	conversationaction.ValidationGroupTitleTooLong:        cervii18n.FieldGroupTitleTooLong,
	conversationaction.ValidationGroupDescriptionTooLong:  cervii18n.FieldGroupDescriptionTooLong,
	conversationaction.ValidationGroupImageFileIDInvalid:  cervii18n.FieldGroupImageFileIDInvalid,
	conversationaction.ValidationGroupMembersRequired:     cervii18n.FieldGroupMembersRequired,
	conversationaction.ValidationGroupMembersTooMany:      cervii18n.FieldGroupMembersTooMany,
	conversationaction.ValidationGroupMemberIDsInvalid:    cervii18n.FieldGroupMemberIDsInvalid,
	conversationaction.ValidationGroupMemberIDInvalid:     cervii18n.FieldGroupMemberIDInvalid,
	conversationaction.ValidationGroupOwnerIDInvalid:      cervii18n.FieldGroupOwnerIDInvalid,
}

// conversationMessageListFromAction 共用成员消息窗口及游标转换。
func (o *directOperations) conversationMessageListFromAction(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, history conversationaction.ConversationMessageHistory) (ConversationMessageList, error) {
	agentAvatarFileIDs := make([]*string, 0, len(history.PendingAgents)+1)
	if history.LatestAgentRun != nil {
		agentAvatarFileIDs = append(agentAvatarFileIDs, history.LatestAgentRun.AgentAvatarFileID)
	}
	for _, agent := range history.PendingAgents {
		agentAvatarFileIDs = append(agentAvatarFileIDs, agent.AvatarFileID)
	}
	avatarURLs, err := o.conversationAvatarURLs(ctx, identity, history.Messages, agentAvatarFileIDs...)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	result := ConversationMessageList{HasEarlier: history.HasEarlier, HasLater: history.HasLater, Messages: make([]ConversationMessage, 0, len(history.Messages))}
	if run := history.LatestAgentRun; run != nil {
		result.LatestAgentRun = &ConversationAgentRun{ID: run.ID, AgentName: run.AgentName, AgentAvatarURL: optionalFileURL(avatarURLs, run.AgentAvatarFileID), Status: AgentRunStatus(run.Status), ErrorCode: run.ErrorCode, LastError: run.LastError}
	}
	result.PendingAgents = make([]ConversationPendingAgent, 0, len(history.PendingAgents))
	for _, agent := range history.PendingAgents {
		result.PendingAgents = append(result.PendingAgents, ConversationPendingAgent{
			IdentityID: agent.IdentityID, DisplayName: agent.DisplayName, AvatarURL: optionalFileURL(avatarURLs, agent.AvatarFileID),
		})
	}
	for _, message := range history.Messages {
		result.Messages = append(result.Messages, conversationMessageFromAction(message, avatarURLs))
	}
	if history.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *history.Before)
		result.Before = &value
	}
	if history.After != nil {
		value := encodeConversationMessageCursor(conversationID, *history.After)
		result.After = &value
	}
	return result, nil
}

// conversationAvatarURLs 批量解析消息发送者、引用发送者、单聊目标和运行中 Agent 的头像。
func (o *directOperations) conversationAvatarURLs(ctx context.Context, identity *servermodels.Identity, messages []conversationaction.ConversationMessage, extraFileIDs ...*string) (map[string]string, error) {
	fileIDs := make([]string, 0, len(messages)+len(extraFileIDs))
	for _, fileID := range extraFileIDs {
		if fileID != nil {
			fileIDs = append(fileIDs, *fileID)
		}
	}
	for _, message := range messages {
		if message.Sender != nil && message.Sender.AvatarFileID != nil {
			fileIDs = append(fileIDs, *message.Sender.AvatarFileID)
		}
		if message.ReplyTo != nil && message.ReplyTo.Sender != nil && message.ReplyTo.Sender.AvatarFileID != nil {
			fileIDs = append(fileIDs, *message.ReplyTo.Sender.AvatarFileID)
		}
	}
	return o.activeFileURLs(ctx, identity, fileIDs)
}

// conversationMessageWithAvatar 转换发送结果并补充头像地址。
func (o *directOperations) conversationMessageWithAvatar(ctx context.Context, identity *servermodels.Identity, message conversationaction.ConversationMessage) ConversationMessage {
	urls, err := o.conversationAvatarURLs(ctx, identity, []conversationaction.ConversationMessage{message})
	if err != nil {
		slog.Warn("读取已保存消息头像失败", "organization_id", identity.Organization.ID, "message_id", message.ID, "error", err)
	}
	return conversationMessageFromAction(message, urls)
}
