//go:build server

// 客户消息发送与客服周期操作。
package appservice

import (
	"context"
	"errors"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	translationaction "github.com/runforyou-ai/cervi/internal/actions/translation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"log/slog"
)

// SendCustomerTextMessage 发送成员客户会话文本消息。
func (o *directOperations) SendCustomerTextMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input CustomerTextMessageInput) (ConversationMessage, error) {
	// 翻译发送先按发送编号沿用已发出的译文，重试直接返回已保存的结果；未发出时，预览过的译文须仍是当前回复语言，未预览则把回复译为客户语言，客户语言与客服语言相同时按原文发送。
	var translation *conversationaction.OutgoingTranslation
	if input.Translation != nil || input.Translate {
		saved, err := o.sendCustomerTextMessage.SavedTranslation(ctx, identity, input.ClientMessageID)
		if err != nil {
			return ConversationMessage{}, customerTextMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
		}
		translation = saved
	}
	if translation == nil && input.Translation != nil {
		if err := o.translator.ValidateReplyLanguage(ctx, identity, conversationID, input.Translation.Language); err != nil {
			return ConversationMessage{}, translationError(ctx, meta, err, cervii18n.ErrorTranslationFailed, identity.Organization.ID, conversationID)
		}
		translation = &conversationaction.OutgoingTranslation{
			Language: input.Translation.Language, SourceLanguage: translationaction.ViewerLanguage(identity.User), Body: input.Translation.Body,
		}
	} else if translation == nil && input.Translate {
		translated, err := o.translator.TranslateReply(ctx, identity, conversationID, input.Body)
		if err != nil {
			return ConversationMessage{}, translationError(ctx, meta, err, cervii18n.ErrorTranslationFailed, identity.Organization.ID, conversationID)
		}
		if translated != nil {
			translation = &conversationaction.OutgoingTranslation{Language: translated.Language, SourceLanguage: translated.SourceLanguage, Body: translated.Body}
		}
	}
	message, err := o.sendCustomerTextMessage.Execute(ctx, identity, conversationaction.CustomerTextMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, Body: input.Body, ReplyToMessageID: input.ReplyToMessageID,
		Visibility: domain.MessageVisibility(input.Visibility), MentionIdentityIDs: input.MentionIdentityIDs, Translation: translation,
	})
	if err != nil {
		return ConversationMessage{}, customerTextMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("成员客户文本消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"visibility", message.Visibility,
		"sender_identity_id", identity.OrganizationIdentity.ID,
	)
	return o.conversationMessageWithAvatar(ctx, identity, message), nil
}

// SendCustomerAttachmentMessage 发送客户会话附件消息。
func (o *directOperations) SendCustomerAttachmentMessage(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input CustomerAttachmentMessageInput) (ConversationMessage, error) {
	message, err := o.sendCustomerAttachmentMessage.Execute(ctx, identity, conversationaction.CustomerAttachmentMessageInput{
		ConversationID: conversationID, ClientMessageID: input.ClientMessageID, FileID: input.FileID, Body: input.Body,
		ReplyToMessageID: input.ReplyToMessageID, ImageWidth: input.ImageWidth, ImageHeight: input.ImageHeight,
	})
	if err != nil {
		return ConversationMessage{}, customerTextMessageError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("成员客户附件消息已保存",
		"organization_id", identity.Organization.ID,
		"conversation_id", conversationID,
		"message_id", message.ID,
		"file_id", input.FileID,
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

// TransferServiceSession 把当前负责的处理周期转给成员、团队队列或公共队列。
func (o *directOperations) TransferServiceSession(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input TransferServiceSessionInput) (CustomerServiceSession, error) {
	result, err := o.transferServiceSession.Execute(ctx, identity, conversationaction.TransferServiceSessionInput{
		ConversationID: conversationID, TargetKind: domain.ServiceSessionTargetKind(input.Kind),
		TeamID: input.TeamID, IdentityID: input.IdentityID,
	})
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
		case conversationaction.ConflictReasonCustomerHandlingRequired:
			messageKey = cervii18n.ErrorCustomerHandlingRequired
		case conversationaction.ConflictReasonServiceSessionOwned:
			messageKey = cervii18n.ErrorServiceSessionOwned
		case conversationaction.ConflictReasonServiceSessionAlreadyOpen:
			messageKey = cervii18n.ErrorServiceSessionAlreadyOpen
		case conversationaction.ConflictReasonTransferTeamUnavailable:
			messageKey = cervii18n.ErrorTransferTeamUnavailable
		}
		return ConflictError(meta, messageKey, conflictError.Reason)
	}
	slog.Warn("客服处理周期操作失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorServiceSessionUpdateFailed)
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
	if errors.Is(err, fileaction.ErrFileNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if validationError, ok := errors.AsType[*conversationaction.ValidationError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, conversationMessageValidationKeys))
	}
	if conflictError, ok := errors.AsType[*conversationaction.ConflictError](err); ok {
		return ConflictError(meta, customerReplyConflictMessageKey(conflictError.Reason), conflictError.Reason)
	}
	slog.Warn("发送成员客户消息失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorMessageSendFailed)
}

// customerReplyConflictMessageKey 返回对客回复资格冲突的本地化文案键。
func customerReplyConflictMessageKey(reason string) cervii18n.Key {
	switch reason {
	case conversationaction.ConflictReasonCustomerHandlingRequired:
		return cervii18n.ErrorCustomerHandlingRequired
	case conversationaction.ConflictReasonServiceSessionOwned:
		return cervii18n.ErrorServiceSessionOwned
	case conversationaction.ConflictReasonServiceSessionNotReplyable:
		return cervii18n.ErrorServiceSessionNotReplyable
	case conversationaction.ConflictReasonChannelOutboundUnavailable:
		return cervii18n.ErrorChannelOutboundUnavailable
	case conversationaction.ConflictReasonChannelOutboundUnsupported:
		return cervii18n.ErrorChannelOutboundUnsupported
	case conversationaction.ConflictReasonReplyTargetInvalid:
		return cervii18n.ErrorReplyTargetInvalid
	case conversationaction.ConflictReasonChannelAttachmentUnsupported:
		return cervii18n.ErrorChannelAttachmentUnsupported
	case conversationaction.ConflictReasonAttachmentTooLarge:
		return cervii18n.ErrorAttachmentTooLarge
	case conversationaction.ConflictReasonCaptionTooLong:
		return cervii18n.ErrorAttachmentCaptionTooLong
	case conversationaction.ConflictReasonTranslationTooLong:
		return cervii18n.ErrorTranslationTooLong
	case conversationaction.ConflictReasonNoteMentionTargetInvalid:
		return cervii18n.ErrorNoteMentionTargetInvalid
	}
	return cervii18n.ErrorMessageConflict
}
