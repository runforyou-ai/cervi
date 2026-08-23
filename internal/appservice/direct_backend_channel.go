//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListWebsiteChannels 返回当前企业的网站渠道。
func (b *DirectBackend) ListWebsiteChannels(ctx context.Context, meta RequestMeta) (WebsiteChannelList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelList{}, err
	}
	channels, err := b.listWebsiteChannels.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return WebsiteChannelList{}, ctx.Err()
		}
		slog.Warn("读取网站渠道列表失败", "organization_id", identity.Organization.ID, "error", err)
		return WebsiteChannelList{}, FailedError(meta, cervii18n.ErrorChannelListFailed)
	}
	result := make([]WebsiteChannelSummary, 0, len(channels))
	for index := range channels {
		result = append(result, websiteChannelFromModel(&channels[index]))
	}
	return WebsiteChannelList{Channels: result}, nil
}

// GetWebsiteChannel 返回网站渠道详情。
func (b *DirectBackend) GetWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannel, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannel{}, err
	}
	detail, err := b.getWebsiteChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return WebsiteChannel{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelReadFailed)
	}
	return WebsiteChannel{
		WebsiteChannelSummary: websiteChannelFromModel(detail.Channel),
		ChatInterface:         websiteChannelSettingFromModel(&detail.ChatInterface),
		Access:                websiteChannelAccessFromModel(&detail.ChatInterface),
	}, nil
}

// CreateWebsiteChannel 创建网站渠道。
func (b *DirectBackend) CreateWebsiteChannel(ctx context.Context, meta RequestMeta, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.createWebsiteChannel.Execute(ctx, identity, channelInput(input))
	if err != nil {
		return WebsiteChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelCreateFailed)
	}
	slog.Info("网站渠道创建成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID)
	return websiteChannelFromModel(channel), nil
}

// UpdateWebsiteChannel 修改网站渠道基础信息。
func (b *DirectBackend) UpdateWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelInput) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.updateWebsiteChannel.Execute(ctx, identity, channelID, channelInput(input))
	if err != nil {
		return WebsiteChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed)
	}
	slog.Info("网站渠道更新成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID)
	return websiteChannelFromModel(channel), nil
}

// UpdateWebsiteChannelChatInterface 修改网站渠道聊天界面。
func (b *DirectBackend) UpdateWebsiteChannelChatInterface(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelChatInterfaceInput) (WebsiteChannelChatInterface, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelChatInterface{}, err
	}
	setting, err := b.updateWebsiteChannelChatInterface.Execute(ctx, identity, channelID, channelaction.WebsiteChannelChatInterfaceInput{
		Title: input.Title, Subtitle: input.Subtitle, GreetingMessage: input.GreetingMessage, ThemeColor: input.ThemeColor,
	})
	if err != nil {
		return WebsiteChannelChatInterface{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelChatInterfaceUpdateFailed)
	}
	slog.Info("网站渠道聊天界面更新成功", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return websiteChannelSettingFromModel(setting), nil
}

// UpdateWebsiteChannelAccess 修改网站渠道接入方式。
func (b *DirectBackend) UpdateWebsiteChannelAccess(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelAccessInput) (WebsiteChannelAccess, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelAccess{}, err
	}
	setting, err := b.updateWebsiteChannelAccess.Execute(ctx, identity, channelID, channelaction.WebsiteChannelAccessInput{
		AllowedHosts: input.AllowedHosts,
	})
	if err != nil {
		return WebsiteChannelAccess{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelAccessUpdateFailed)
	}
	slog.Info("网站渠道接入方式更新成功", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return websiteChannelAccessFromModel(setting), nil
}

// DeactivateWebsiteChannel 停用网站渠道。
func (b *DirectBackend) DeactivateWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.updateWebsiteChannelStatus.Execute(ctx, identity, channelID, false)
	if err != nil {
		return WebsiteChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed)
	}
	slog.Info("网站渠道状态已更新", "organization_id", identity.Organization.ID, "channel_id", channelID, "enabled", false)
	return websiteChannelFromModel(channel), nil
}

// ActivateWebsiteChannel 启用网站渠道。
func (b *DirectBackend) ActivateWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelSummary{}, err
	}
	channel, err := b.updateWebsiteChannelStatus.Execute(ctx, identity, channelID, true)
	if err != nil {
		return WebsiteChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed)
	}
	slog.Info("网站渠道状态已更新", "organization_id", identity.Organization.ID, "channel_id", channel.ID, "enabled", true)
	return websiteChannelFromModel(channel), nil
}

// ListChannels 返回当前企业的渠道选择项。
func (b *DirectBackend) ListChannels(ctx context.Context, meta RequestMeta) (ChannelList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ChannelList{}, err
	}
	channels, err := b.listChannels.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return ChannelList{}, ctx.Err()
		}
		slog.Warn("读取渠道列表失败", "organization_id", identity.Organization.ID, "error", err)
		return ChannelList{}, FailedError(meta, cervii18n.ErrorChannelSummaryListFailed)
	}
	result := make([]ChannelSummary, 0, len(channels))
	for _, channel := range channels {
		result = append(result, ChannelSummary{ID: channel.ID, Type: ChannelType(channel.Type), Name: channel.Name})
	}
	return ChannelList{Channels: result}, nil
}

// channelMutationError 转换渠道写入校验和操作错误。
func (b *DirectBackend) channelMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, channelFieldKeys(validationError.Fields))
	}
	return b.channelError(ctx, meta, err, failureKey)
}

// channelError 转换渠道读取和状态修改错误。
func (b *DirectBackend) channelError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, channelaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	slog.Warn("网站渠道操作失败", "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

func websiteChannelFromModel(channel *servermodels.Channel) WebsiteChannelSummary {
	return WebsiteChannelSummary{
		ID: channel.ID, OrganizationID: channel.OrganizationID, CreatedByUserID: channel.CreatedByUserID,
		Type: ChannelType(channel.Type), Name: channel.Name, Description: channel.Description, DefaultLocale: Locale(channel.DefaultLocale), Enabled: channel.Enabled,
		CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt,
	}
}

func websiteChannelSettingFromModel(setting *servermodels.WebsiteChannelSetting) WebsiteChannelChatInterface {
	return WebsiteChannelChatInterface{Title: setting.ChatTitle, Subtitle: setting.ChatSubtitle, GreetingMessage: setting.GreetingMessage, ThemeColor: setting.ThemeColor}
}

// websiteChannelAccessFromModel 把存储设置转换为接入方式契约。
func websiteChannelAccessFromModel(setting *servermodels.WebsiteChannelSetting) WebsiteChannelAccess {
	allowedHosts := setting.AllowedEmbedHosts
	if allowedHosts == nil {
		allowedHosts = []string{}
	}
	return WebsiteChannelAccess{AllowedHosts: allowedHosts}
}

func channelInput(input WebsiteChannelInput) channelaction.WebsiteChannelInput {
	return channelaction.WebsiteChannelInput{Type: domain.ChannelType(input.Type), Name: input.Name, Description: input.Description, DefaultLocale: domain.Locale(input.DefaultLocale)}
}

func channelFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		channelaction.ValidationTypeInvalid:          cervii18n.FieldChannelTypeInvalid,
		channelaction.ValidationNameRequired:         cervii18n.FieldChannelNameRequired,
		channelaction.ValidationNameTooLong:          cervii18n.FieldChannelNameTooLong,
		channelaction.ValidationDescriptionTooLong:   cervii18n.FieldChannelDescriptionTooLong,
		channelaction.ValidationDefaultLocaleInvalid: cervii18n.FieldChannelDefaultLocaleInvalid,
		channelaction.ValidationChatTitleRequired:    cervii18n.FieldChannelChatTitleRequired,
		channelaction.ValidationChatTitleTooLong:     cervii18n.FieldChannelChatTitleTooLong,
		channelaction.ValidationChatSubtitleTooLong:  cervii18n.FieldChannelChatSubtitleTooLong,
		channelaction.ValidationGreetingTooLong:      cervii18n.FieldChannelGreetingTooLong,
		channelaction.ValidationThemeColorInvalid:    cervii18n.FieldChannelThemeColorInvalid,
		channelaction.ValidationAllowedHostsTooMany:  cervii18n.FieldChannelAllowedHostsTooMany,
		channelaction.ValidationAllowedHostInvalid:   cervii18n.FieldChannelAllowedHostInvalid,
	}
	return translateValidationFields(fields, keys)
}
