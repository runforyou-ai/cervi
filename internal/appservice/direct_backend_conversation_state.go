//go:build server

// 会话已读、输入状态、未读标记、置顶与通知设置。
package appservice

import (
	"context"
	"errors"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"log/slog"
	"net/http"
	"strconv"
)

// MarkConversationRead 单调推进当前用户的会话已读水位。
func (o *directOperations) MarkConversationRead(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input MarkConversationReadInput) (ConversationReadState, error) {
	state, err := o.markConversationRead.Execute(ctx, identity, conversationID, input.LastReadMessageID, input.ClearUnreadMark)
	if err != nil {
		return ConversationReadState{}, conversationReadError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	return ConversationReadState{ReadSeq: strconv.FormatInt(state.ReadSeq, 10), LastReadMessageID: state.LastReadMessageID, LastReadAt: state.LastReadAt}, nil
}

// ReportConversationTyping 按会话类型校验发送资格后发布当前用户的输入状态。
func (o *directOperations) ReportConversationTyping(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationTypingInput) error {
	err := o.reportConversationTyping.Execute(ctx, identity, conversationID, input.Active)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, conversationaction.ErrConversationNotFound) {
		return NotFoundError(meta, cervii18n.ErrorConversationNotFound)
	}
	slog.Warn("发布会话输入状态失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
	return UnavailableError(meta, cervii18n.ErrorServerUnavailable, nil).WithStatus(http.StatusServiceUnavailable)
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

// UpdateConversationPin 保存个人置顶事实与置顶顺序，并返回写入后的顺序版本。
func (o *directOperations) UpdateConversationPin(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string, input ConversationPinInput) (ConversationPinState, error) {
	expectedVersion, err := strconv.ParseInt(input.ExpectedPinOrderVersion, 10, 64)
	if input.ExpectedPinOrderVersion == "" || err != nil {
		return ConversationPinState{}, InvalidError(meta, cervii18n.ErrorValidationFailed,
			map[string]cervii18n.Key{"expectedPinOrderVersion": cervii18n.FieldConversationPinTargetInvalid})
	}
	state, err := o.updateConversationPin.Execute(ctx, identity, conversationaction.ConversationPinInput{
		ConversationID: conversationID, Pinned: input.Pinned, NeighborID: input.NeighborID,
		Position: domain.ConversationPinPosition(input.Position), ExpectedPinOrderVersion: expectedVersion,
	})
	if err != nil {
		return ConversationPinState{}, conversationPinError(ctx, meta, err, identity.Organization.ID, conversationID)
	}
	slog.Info("会话置顶已保存", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "user_id", identity.User.ID, "pinned", state.Pinned)
	return ConversationPinState{Pinned: state.Pinned, PinOrderVersion: strconv.FormatInt(state.PinOrderVersion, 10)}, nil
}

// conversationPinError 转换个人置顶写入错误，顺序版本过期与邻居失效都要求客户端整区重读。
func conversationPinError(ctx context.Context, meta RequestMeta, err error, organizationID, conversationID string) error {
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
		return ConflictError(meta, cervii18n.ErrorConversationPinOrderStale, conflictError.Reason)
	}
	slog.Warn("更新会话置顶失败", "organization_id", organizationID, "conversation_id", conversationID, "error", err)
	return FailedError(meta, cervii18n.ErrorConversationPinUpdateFailed)
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
