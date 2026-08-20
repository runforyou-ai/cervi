//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

func (b *DirectBackend) channelMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, channelFieldKeys(validationError.Fields))
	}
	return b.channelError(ctx, meta, err, failureKey)
}

func (b *DirectBackend) channelError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return localizedError(meta, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
	}
	if errors.Is(err, channelaction.ErrNotFound) {
		return localizedError(meta, http.StatusNotFound, "CHANNEL_NOT_FOUND", cervii18n.ErrorChannelNotFound, nil)
	}
	slog.Warn("网站渠道操作失败", "failure", failureKey, "error", err)
	return localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
}

func websiteChannelFromModel(channel *servermodels.Channel) WebsiteChannelSummary {
	return WebsiteChannelSummary{
		ID: channel.ID, OrganizationID: channel.OrganizationID, CreatedByUserID: channel.CreatedByUserID,
		Type: ChannelType(channel.Type), Name: channel.Name, Description: channel.Description, DefaultLocale: Locale(channel.DefaultLocale),
		CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt, DeletedAt: channel.DeletedAt,
	}
}

func websiteChannelSettingFromModel(setting *servermodels.WebsiteChannelSetting) WebsiteChannelChatInterface {
	return WebsiteChannelChatInterface{Title: setting.ChatTitle, Subtitle: setting.ChatSubtitle, GreetingMessage: setting.GreetingMessage, ThemeColor: setting.ThemeColor}
}

func channelInput(input WebsiteChannelInput) channelaction.WebsiteChannelInput {
	return channelaction.WebsiteChannelInput{Name: input.Name, Description: input.Description, DefaultLocale: domain.Locale(input.DefaultLocale)}
}

func channelFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		channelaction.ValidationNameRequired: cervii18n.FieldChannelNameRequired, channelaction.ValidationNameTooLong: cervii18n.FieldChannelNameTooLong,
		channelaction.ValidationDescriptionTooLong: cervii18n.FieldChannelDescriptionTooLong, channelaction.ValidationDefaultLocaleInvalid: cervii18n.FieldChannelDefaultLocaleInvalid,
		channelaction.ValidationChatTitleRequired: cervii18n.FieldChannelChatTitleRequired, channelaction.ValidationChatTitleTooLong: cervii18n.FieldChannelChatTitleTooLong,
		channelaction.ValidationChatSubtitleTooLong: cervii18n.FieldChannelChatSubtitleTooLong, channelaction.ValidationGreetingTooLong: cervii18n.FieldChannelGreetingTooLong,
		channelaction.ValidationThemeColorInvalid: cervii18n.FieldChannelThemeColorInvalid,
	}
	return translateValidationFields(fields, keys)
}
