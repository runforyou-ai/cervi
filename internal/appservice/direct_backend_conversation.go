//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// SendCustomerTextMessage 发送成员客户会话文本消息。
func (b *DirectBackend) SendCustomerTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input CustomerTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendCustomerTextMessage.Execute(ctx, identity, conversationaction.CustomerTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body,
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
	return conversationMessageFromAction(message), nil
}

// ClaimServiceSession 领取或接管客户会话最新处理周期。
func (b *DirectBackend) ClaimServiceSession(ctx context.Context, meta RequestMeta, conversationID string) (CustomerServiceSession, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerServiceSession{}, err
	}
	result, err := b.claimServiceSession.Execute(ctx, identity, conversationID)
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// TransferServiceSession 把当前负责的处理周期转给另一位客服。
func (b *DirectBackend) TransferServiceSession(ctx context.Context, meta RequestMeta, conversationID string, input TransferServiceSessionInput) (CustomerServiceSession, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerServiceSession{}, err
	}
	result, err := b.transferServiceSession.Execute(ctx, identity, conversationaction.TransferServiceSessionInput{ConversationID: conversationID, AssigneeIdentityID: input.AssigneeIdentityID})
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// CloseServiceSession 关闭客户会话最新处理周期。
func (b *DirectBackend) CloseServiceSession(ctx context.Context, meta RequestMeta, conversationID string) (CustomerServiceSession, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerServiceSession{}, err
	}
	result, err := b.closeServiceSession.Execute(ctx, identity, conversationID)
	if err != nil {
		return CustomerServiceSession{}, serviceSessionMutationError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return customerServiceSessionFromAction(result), nil
}

// ReopenServiceSession 重新打开客户会话最新处理周期并分配给当前身份。
func (b *DirectBackend) ReopenServiceSession(ctx context.Context, meta RequestMeta, conversationID string) (CustomerServiceSession, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CustomerServiceSession{}, err
	}
	result, err := b.reopenServiceSession.Execute(ctx, identity, conversationID)
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
func (b *DirectBackend) SendFirstDirectTextMessage(ctx context.Context, meta RequestMeta, input FirstDirectTextMessageInput) (FirstDirectTextMessageResult, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return FirstDirectTextMessageResult{}, err
	}
	result, err := b.sendFirstDirectTextMessage.Execute(ctx, identity, conversationaction.FirstDirectTextMessageInput{
		TargetIdentityID: input.TargetIdentityID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return FirstDirectTextMessageResult{}, directConversationError(ctx, meta, err, identity.Organization.ID, input.TargetIdentityID, "send_first")
	}
	slog.Info("企业成员内部单聊首条文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", result.Conversation.ID,
		"target_identity_id", result.Conversation.PeerIdentityID,
		"message_id", result.Message.ID,
	)
	return FirstDirectTextMessageResult{
		Conversation: directInboxConversationFromSummary(result.Conversation),
		Message:      conversationMessageFromAction(result.Message),
	}, nil
}

// FindDirectConversation 按目标身份查找当前成员的活跃单聊。
func (b *DirectBackend) FindDirectConversation(ctx context.Context, meta RequestMeta, targetIdentityID string) (DirectConversationLookup, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return DirectConversationLookup{}, err
	}
	summary, err := b.findDirectConversation.Execute(ctx, identity, targetIdentityID)
	if err != nil {
		return DirectConversationLookup{}, directConversationError(ctx, meta, err, identity.Organization.ID, targetIdentityID, "find")
	}
	if summary == nil {
		return DirectConversationLookup{}, nil
	}
	conversation := directInboxConversationFromSummary(*summary)
	return DirectConversationLookup{Conversation: &conversation}, nil
}

// directInboxConversationFromSummary 把单聊摘要转换为统一收件箱会话。
func directInboxConversationFromSummary(summary conversationaction.DirectConversationSummary) InboxConversation {
	return InboxConversation{
		ID: summary.ID, Type: ConversationTypeDirect,
		Direct: &DirectInboxConversation{
			PeerIdentityID: summary.PeerIdentityID, PeerType: OrganizationIdentityType(summary.PeerType), PeerName: summary.PeerName,
			Preview: summary.Preview, LastMessageAt: summary.LastMessageAt,
		},
	}
}

// SendDirectTextMessage 发送内部单聊文本消息。
func (b *DirectBackend) SendDirectTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input DirectTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendDirectTextMessage.Execute(ctx, identity, conversationaction.DirectTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body,
	})
	if err != nil {
		return ConversationMessage{}, directConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "send")
	}
	slog.Info("企业成员内部单聊文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return conversationMessageFromAction(message), nil
}

// CreateGroupConversation 创建只包含有效真人成员的企业内部群聊。
func (b *DirectBackend) CreateGroupConversation(ctx context.Context, meta RequestMeta, input GroupConversationInput) (InboxConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return InboxConversation{}, err
	}
	summary, err := b.createGroupConversation.Execute(ctx, identity, conversationaction.GroupConversationInput{
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
	imageURLs, imageErr := b.activeFileURLs(ctx, identity, imageFileIDs)
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
func (b *DirectBackend) GetGroupConversation(ctx context.Context, meta RequestMeta, conversationID string) (GroupConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return GroupConversation{}, err
	}
	record, err := b.getGroupConversation.Execute(ctx, identity, conversationID)
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "get")
	}
	result, err := b.groupConversationFromAction(ctx, identity, record)
	if err != nil {
		slog.Warn("读取群聊图片或成员头像失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return GroupConversation{}, FailedError(meta, cervii18n.ErrorGroupConversationReadFailed)
	}
	return result, nil
}

// UpdateGroupConversation 修改群聊资料。
func (b *DirectBackend) UpdateGroupConversation(ctx context.Context, meta RequestMeta, conversationID string, input GroupConversationProfileInput) (GroupConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return GroupConversation{}, err
	}
	record, err := b.updateGroupConversation.Execute(ctx, identity, conversationaction.GroupConversationProfileInput{
		ConversationID: conversationID, Title: input.Title, Description: input.Description, ImageFileID: input.ImageFileID,
	})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "update")
	}
	slog.Info("企业群聊资料已修改", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID)
	return b.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// AddGroupConversationMembers 批量增加群聊成员。
func (b *DirectBackend) AddGroupConversationMembers(ctx context.Context, meta RequestMeta, conversationID string, input GroupConversationMembersInput) (GroupConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return GroupConversation{}, err
	}
	record, err := b.addGroupConversationMembers.Execute(ctx, identity, conversationaction.GroupConversationMembersInput{ConversationID: conversationID, MemberIdentityIDs: input.MemberIdentityIDs})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "add_members")
	}
	slog.Info("企业群聊成员已增加", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "added_count", len(input.MemberIdentityIDs))
	return b.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// RemoveGroupConversationMember 移除单个群聊成员。
func (b *DirectBackend) RemoveGroupConversationMember(ctx context.Context, meta RequestMeta, conversationID string, input GroupConversationMemberInput) (GroupConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return GroupConversation{}, err
	}
	record, err := b.removeGroupConversationMember.Execute(ctx, identity, conversationaction.GroupConversationMemberInput{ConversationID: conversationID, MemberIdentityID: input.MemberIdentityID})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "remove_member")
	}
	slog.Info("企业群聊成员已移除", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "member_identity_id", input.MemberIdentityID)
	return b.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// TransferGroupConversationOwner 转让群主。
func (b *DirectBackend) TransferGroupConversationOwner(ctx context.Context, meta RequestMeta, conversationID string, input GroupConversationOwnerInput) (GroupConversation, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return GroupConversation{}, err
	}
	record, err := b.transferGroupConversationOwner.Execute(ctx, identity, conversationaction.GroupConversationOwnerInput{ConversationID: conversationID, OwnerIdentityID: input.OwnerIdentityID})
	if err != nil {
		return GroupConversation{}, groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "transfer_owner")
	}
	slog.Info("企业群聊群主已转让", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID, "owner_identity_id", input.OwnerIdentityID)
	return b.groupConversationMutationResult(ctx, meta, identity, record, conversationID)
}

// LeaveGroupConversation 退出群聊并按需转让群主。
func (b *DirectBackend) LeaveGroupConversation(ctx context.Context, meta RequestMeta, conversationID string, input GroupConversationLeaveInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	err = b.leaveGroupConversation.Execute(ctx, identity, conversationaction.GroupConversationLeaveInput{ConversationID: conversationID, SuccessorIdentityID: input.SuccessorIdentityID})
	if err != nil {
		return groupConversationError(ctx, meta, err, identity.Organization.ID, conversationID, "leave")
	}
	slog.Info("企业群聊退出或解散操作已完成", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "operator_identity_id", identity.OrganizationIdentity.ID)
	return nil
}

// groupConversationMutationResult 转换群聊管理命令结果。
func (b *DirectBackend) groupConversationMutationResult(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, record conversationaction.GroupConversation, conversationID string) (GroupConversation, error) {
	result, err := b.groupConversationFromAction(ctx, identity, record)
	if err != nil {
		slog.Warn("读取群聊管理结果图片失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return GroupConversation{}, FailedError(meta, cervii18n.ErrorGroupConversationReadFailed)
	}
	return result, nil
}

// groupConversationFromAction 转换群聊资料并生成群图片和成员头像地址。
func (b *DirectBackend) groupConversationFromAction(ctx context.Context, identity *servermodels.Identity, record conversationaction.GroupConversation) (GroupConversation, error) {
	avatarFileIDs := make([]string, 0, len(record.Participants)+1)
	if record.ImageFileID != nil {
		avatarFileIDs = append(avatarFileIDs, *record.ImageFileID)
	}
	for _, participant := range record.Participants {
		if participant.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *participant.AvatarFileID)
		}
	}
	avatarURLs, err := b.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		return GroupConversation{}, err
	}
	participants := make([]GroupParticipant, 0, len(record.Participants))
	for _, participant := range record.Participants {
		participants = append(participants, GroupParticipant{
			ChatSubjectID: participant.ChatSubjectID, IdentityID: participant.IdentityID, DisplayName: participant.DisplayName,
			AvatarURL: optionalFileURL(avatarURLs, participant.AvatarFileID), Role: GroupParticipantRole(participant.Role),
		})
	}
	return GroupConversation{
		ID: record.ID, Title: record.Title, Description: record.Description,
		ImageURL: optionalFileURL(avatarURLs, record.ImageFileID), Status: ConversationStatus(record.Status),
		CreatedAt: record.CreatedAt, Participants: participants,
	}, nil
}

// SendGroupTextMessage 发送企业内部群聊文本消息。
func (b *DirectBackend) SendGroupTextMessage(ctx context.Context, meta RequestMeta, conversationID string, input GroupTextMessageInput) (ConversationMessage, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessage{}, err
	}
	message, err := b.sendGroupTextMessage.Execute(ctx, identity, conversationaction.GroupTextMessageInput{
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
	return conversationMessageFromAction(message), nil
}

// ListConversationMessages 返回成员可见的会话消息。
func (b *DirectBackend) ListConversationMessages(ctx context.Context, meta RequestMeta, conversationID string, input ConversationMessageListInput) (ConversationMessageList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationMessageList{}, err
	}
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

	history, err := b.listConversationMessages.Execute(ctx, identity, actionInput)
	if err != nil {
		return ConversationMessageList{}, conversationMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return conversationMessageListFromAction(conversationID, history), nil
}

// MarkConversationRead 单调推进当前用户的原生会话已读水位。
func (b *DirectBackend) MarkConversationRead(ctx context.Context, meta RequestMeta, conversationID string, input MarkConversationReadInput) (ConversationReadState, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationReadState{}, err
	}
	state, err := b.markConversationRead.Execute(ctx, identity, conversationID, input.LastReadMessageID, input.ClearUnreadMark)
	if err != nil {
		return ConversationReadState{}, conversationReadError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return ConversationReadState{LastReadMessageID: state.LastReadMessageID, LastReadAt: state.LastReadAt}, nil
}

// UpdateConversationUnreadMark 保存个人未读标记，不改变已读和提及查看水位。
func (b *DirectBackend) UpdateConversationUnreadMark(ctx context.Context, meta RequestMeta, conversationID string, input ConversationUnreadMarkInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.updateConversationUnreadMark.Execute(ctx, identity, conversationID, input.MarkedUnread); err != nil {
		return conversationReadError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	if input.MarkedUnread {
		slog.Info("会话已标为未读", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "user_id", identity.User.ID)
	}
	return nil
}

// UpdateConversationNotificationSettings 保存当前用户的原生会话提醒设置。
func (b *DirectBackend) UpdateConversationNotificationSettings(ctx context.Context, meta RequestMeta, conversationID string, input ConversationNotificationSettingsInput) (ConversationNotificationSettings, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ConversationNotificationSettings{}, err
	}
	settings, err := b.updateConversationNotifications.Execute(ctx, identity, conversationID, input.Muted)
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
func conversationMessageFromAction(message conversationaction.ConversationMessage) ConversationMessage {
	sender := conversationMessageSenderFromAction(message.Sender)
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
			ID: message.ReplyTo.ID, Body: message.ReplyTo.Body, Deleted: message.ReplyTo.Deleted,
			Sender: conversationMessageSenderFromAction(message.ReplyTo.Sender),
		}
	}
	mentions := make([]ConversationMessageMention, 0, len(message.Mentions))
	for _, mention := range message.Mentions {
		mentions = append(mentions, ConversationMessageMention{
			ChatSubjectID: mention.ChatSubjectID, Kind: ChatSubjectKind(mention.Kind),
			SourceID: mention.SourceID, DisplayName: mention.DisplayName,
		})
	}
	return ConversationMessage{
		ID: message.ID, Type: MessageType(message.Type), Body: message.Body,
		OriginatedAt: message.OriginatedAt, SourceOrder: message.SourceOrder, CreatedAt: message.CreatedAt, GroupMessageSequence: groupMessageSequenceString(message.GroupMessageSequence),
		Sender: sender, SessionStart: sessionStart, SystemEvent: systemEvent,
		ReplyTo: replyTo, Mentions: mentions, MentionAll: message.MentionAll,
	}
}

// conversationMessageSenderFromAction 转换消息发送主体。
func conversationMessageSenderFromAction(sender *conversationaction.ConversationMessageSender) *ConversationMessageSender {
	if sender == nil {
		return nil
	}
	return &ConversationMessageSender{
		ChatSubjectID: sender.ChatSubjectID, Kind: ChatSubjectKind(sender.Kind),
		SourceID: sender.SourceID, DisplayName: sender.DisplayName,
	}
}

// directConversationError 转换内部单聊命令错误。
func directConversationError(ctx context.Context, meta RequestMeta, err error, organizationID, targetID, operation string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
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
		return ConflictError(meta, cervii18n.ErrorDirectMessageConflict, conflictError.Reason)
	}
	slog.Warn("内部单聊操作失败", "organization_id", organizationID, "target_id", targetID, "operation", operation, "error", err)
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
		messageKey := cervii18n.ErrorGroupMessageConflict
		switch conflictError.Reason {
		case conversationaction.ConflictReasonGroupMemberAlreadyActive:
			messageKey = cervii18n.ErrorGroupMemberAlreadyActive
		case conversationaction.ConflictReasonGroupMemberNotActive:
			messageKey = cervii18n.ErrorGroupMemberNotActive
		case conversationaction.ConflictReasonGroupOwnerCannotBeRemoved:
			messageKey = cervii18n.ErrorGroupOwnerCannotBeRemoved
		case conversationaction.ConflictReasonGroupSuccessorRequired:
			messageKey = cervii18n.ErrorGroupSuccessorRequired
		case conversationaction.ConflictReasonGroupReplyTargetInvalid:
			messageKey = cervii18n.ErrorGroupReplyTargetInvalid
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

// encodeConversationMessageCursor 编码绑定会话的成员消息游标。
func encodeConversationMessageCursor(conversationID string, point conversationaction.MessageCursorPoint) string {
	if point.GroupMessageSequence != nil {
		return conversationID + ".group." + strconv.FormatInt(*point.GroupMessageSequence, 10) + "." + point.ID
	}
	return conversationID + "." + strconv.FormatInt(point.OriginatedAt.UnixNano(), 10) + "." + strconv.FormatInt(point.SourceOrder, 10) + "." + point.ID
}

// decodeConversationMessageCursor 解码并校验成员消息游标所属会话。
func decodeConversationMessageCursor(value, conversationID string) (conversationaction.MessageCursorPoint, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || parts[0] != conversationID || !common.ValidUUID(parts[3]) {
		return conversationaction.MessageCursorPoint{}, false
	}
	if parts[1] == "group" {
		sequence, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || sequence <= 0 {
			return conversationaction.MessageCursorPoint{}, false
		}
		return conversationaction.MessageCursorPoint{ID: parts[3], GroupMessageSequence: &sequence}, true
	}
	originatedAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || originatedAt <= 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	sourceOrder, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || sourceOrder < 0 {
		return conversationaction.MessageCursorPoint{}, false
	}
	return conversationaction.MessageCursorPoint{OriginatedAt: time.Unix(0, originatedAt).UTC(), SourceOrder: sourceOrder, ID: parts[3]}, true
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
	if errors.Is(err, conversationaction.ErrMentionProgressChanged) {
		return ConflictError(meta, cervii18n.ErrorConversationMentionProgressChanged, "mention_progress_changed")
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
		messageKey := cervii18n.ErrorCustomerMessageConflict
		switch conflictError.Reason {
		case conversationaction.ConflictReasonServiceSessionOwned:
			messageKey = cervii18n.ErrorServiceSessionOwned
		case conversationaction.ConflictReasonServiceSessionNotReplyable:
			messageKey = cervii18n.ErrorServiceSessionNotReplyable
		case conversationaction.ConflictReasonChannelOutboundUnsupported:
			messageKey = cervii18n.ErrorChannelOutboundUnsupported
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("发送成员客户消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorCustomerMessageSendFailed)
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
	conversationaction.ValidationGroupSuccessorIDInvalid:  cervii18n.FieldGroupSuccessorIDInvalid,
}

// conversationMessageListFromAction 共用成员消息窗口及游标转换。
func conversationMessageListFromAction(conversationID string, history conversationaction.ConversationMessageHistory) ConversationMessageList {
	result := ConversationMessageList{HasEarlier: history.HasEarlier, HasLater: history.HasLater, Messages: make([]ConversationMessage, 0, len(history.Messages))}
	for _, message := range history.Messages {
		result.Messages = append(result.Messages, conversationMessageFromAction(message))
	}
	if history.Before != nil {
		value := encodeConversationMessageCursor(conversationID, *history.Before)
		result.Before = &value
	}
	if history.After != nil {
		value := encodeConversationMessageCursor(conversationID, *history.After)
		result.After = &value
	}
	return result
}
