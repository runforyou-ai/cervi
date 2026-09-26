//go:build server

// 群聊资料、成员管理与群消息发送。
package appservice

import (
	"context"
	"errors"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"log/slog"
	"net/http"
)

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
			MemberPreviewNames: summary.MemberPreviewNames,
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
			AssistantOwnerName: participant.AssistantOwnerName, AssistantOwnerIdentityID: participant.AssistantOwnerIdentityID,
		})
	}
	return GroupConversation{
		ID: record.ID, Title: record.Title, Description: record.Description,
		ImageURL: optionalFileURL(avatarURLs, record.ImageFileID), Status: ConversationStatus(record.Status),
		CreatedAt: record.CreatedAt, Participants: participants,
		Muted: record.Muted, MemberPreviewNames: record.MemberPreviewNames,
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
	if errors.Is(err, fileaction.ErrLinkedImageNotFound) {
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
		if key, ok := assistantConflictKeys[conflictError.Reason]; ok {
			messageKey = key
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
