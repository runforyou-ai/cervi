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
)

// ListMessageChannels 返回当前企业的消息渠道。
func (b *DirectBackend) ListMessageChannels(ctx context.Context, meta RequestMeta) (MessageChannelList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MessageChannelList{}, err
	}
	channels, err := b.listMessageChannels.Execute(ctx, identity)
	if err != nil {
		return MessageChannelList{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelListFailed, identity.Organization.ID, "")
	}
	result := make([]MessageChannelSummary, 0, len(channels))
	for index := range channels {
		result = append(result, messageChannelFromRecord(&channels[index]))
	}
	return MessageChannelList{Channels: result}, nil
}

// GetWebsiteChannel 返回网站渠道详情。
func (b *DirectBackend) GetWebsiteChannel(ctx context.Context, meta RequestMeta, channelID string) (WebsiteChannel, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannel{}, err
	}
	detail, err := b.getWebsiteChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return WebsiteChannel{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelReadFailed, identity.Organization.ID, channelID)
	}
	return WebsiteChannel{
		MessageChannelSummary: messageChannelFromRecord(&detail.MessageChannelRecord),
		ChatInterface:         websiteChannelSettingFromRecord(&detail.ChatInterface),
		Access:                websiteChannelAccessFromRecord(&detail.ChatInterface),
	}, nil
}

// GetMessageChannel 返回消息渠道基础信息。
func (b *DirectBackend) GetMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MessageChannelSummary{}, err
	}
	channel, err := b.getMessageChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return MessageChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelReadFailed, identity.Organization.ID, channelID)
	}
	return messageChannelFromRecord(channel), nil
}

// CreateMessageChannel 创建消息渠道。
func (b *DirectBackend) CreateMessageChannel(ctx context.Context, meta RequestMeta, input CreateMessageChannelInput) (MessageChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MessageChannelSummary{}, err
	}
	channel, err := b.createMessageChannel.Execute(ctx, identity, createChannelInput(input))
	if err != nil {
		return MessageChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("消息渠道创建成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID, "channel_type", channel.Type)
	return messageChannelFromRecord(channel), nil
}

// UpdateMessageChannel 修改消息渠道基础信息。
func (b *DirectBackend) UpdateMessageChannel(ctx context.Context, meta RequestMeta, channelID string, input MessageChannelInput) (MessageChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MessageChannelSummary{}, err
	}
	channel, err := b.updateMessageChannel.Execute(ctx, identity, channelID, channelInput(input))
	if err != nil {
		return MessageChannelSummary{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed, identity.Organization.ID, channelID)
	}
	slog.Info("消息渠道更新成功", "organization_id", identity.Organization.ID, "channel_id", channel.ID, "channel_type", channel.Type)
	return messageChannelFromRecord(channel), nil
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
		return WebsiteChannelChatInterface{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelChatInterfaceUpdateFailed, identity.Organization.ID, channelID)
	}
	slog.Info("网站渠道聊天界面更新成功", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return websiteChannelSettingFromRecord(setting), nil
}

// UpdateWebsiteChannelAccess 修改网站渠道允许使用的网站。
func (b *DirectBackend) UpdateWebsiteChannelAccess(ctx context.Context, meta RequestMeta, channelID string, input WebsiteChannelAccessInput) (WebsiteChannelAccess, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return WebsiteChannelAccess{}, err
	}
	setting, err := b.updateWebsiteChannelAccess.Execute(ctx, identity, channelID, channelaction.WebsiteChannelAccessInput{
		AllowedHosts: input.AllowedHosts,
	})
	if err != nil {
		return WebsiteChannelAccess{}, b.channelMutationError(ctx, meta, err, cervii18n.ErrorChannelAccessUpdateFailed, identity.Organization.ID, channelID)
	}
	slog.Info("网站渠道允许使用的网站更新成功", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return websiteChannelAccessFromRecord(setting), nil
}

// DeactivateMessageChannel 停用消息渠道。
func (b *DirectBackend) DeactivateMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	return b.setMessageChannelEnabled(ctx, meta, channelID, false)
}

// ActivateMessageChannel 启用消息渠道。
func (b *DirectBackend) ActivateMessageChannel(ctx context.Context, meta RequestMeta, channelID string) (MessageChannelSummary, error) {
	return b.setMessageChannelEnabled(ctx, meta, channelID, true)
}

// setMessageChannelEnabled 修改消息渠道的启用状态。
func (b *DirectBackend) setMessageChannelEnabled(ctx context.Context, meta RequestMeta, channelID string, enabled bool) (MessageChannelSummary, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MessageChannelSummary{}, err
	}
	channel, err := b.updateMessageChannelStatus.Execute(ctx, identity, channelID, enabled)
	if err != nil {
		return MessageChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed, identity.Organization.ID, channelID)
	}
	slog.Info("消息渠道状态已更新", "organization_id", identity.Organization.ID, "channel_id", channel.ID, "channel_type", channel.Type, "enabled", enabled)
	return messageChannelFromRecord(channel), nil
}

// ListChannelOptions 返回当前企业的渠道选择项。
func (b *DirectBackend) ListChannelOptions(ctx context.Context, meta RequestMeta) (ChannelOptionList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return ChannelOptionList{}, err
	}
	channels, err := b.listChannelOptions.Execute(ctx, identity)
	if err != nil {
		return ChannelOptionList{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelSummaryListFailed, identity.Organization.ID, "")
	}
	result := make([]ChannelOption, 0, len(channels))
	for _, channel := range channels {
		result = append(result, ChannelOption{ID: channel.ID, Type: ChannelType(channel.Type), Name: channel.Name})
	}
	return ChannelOptionList{Channels: result}, nil
}

// channelMutationError 转换渠道写入校验和操作错误。
func (b *DirectBackend) channelMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, channelID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, channelFieldKeys(validationError.Fields))
	}
	return b.channelError(ctx, meta, err, failureKey, organizationID, channelID)
}

// channelError 转换渠道读取和状态修改错误。
func (b *DirectBackend) channelError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, channelID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, channelaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if channelID != "" {
		attributes = append(attributes, "channel_id", channelID)
	}
	slog.Warn("消息渠道操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// messageChannelFromRecord 转换消息渠道传输结构。
func messageChannelFromRecord(channel *channelaction.MessageChannelRecord) MessageChannelSummary {
	return MessageChannelSummary{
		ID: channel.ID, OrganizationID: channel.OrganizationID, CreatedByUserID: channel.CreatedByUserID,
		Type: ChannelType(channel.Type), Name: channel.Name, Description: channel.Description, DefaultLocale: Locale(channel.DefaultLocale), Enabled: channel.Enabled,
		NewConversationTarget: channelRoutingTargetFromRecord(channel.InitialRoutingTargetType, channel.InitialRoutingTargetID),
		FallbackTarget:        channelRoutingTargetFromRecord(channel.FallbackRoutingTargetType, channel.FallbackRoutingTargetID),
		CreatedAt:             channel.CreatedAt, UpdatedAt: channel.UpdatedAt,
	}
}

// websiteChannelSettingFromRecord 转换网站渠道聊天界面设置。
func websiteChannelSettingFromRecord(setting *channelaction.WebsiteChannelSettingRecord) WebsiteChannelChatInterface {
	return WebsiteChannelChatInterface{Title: setting.ChatTitle, Subtitle: setting.ChatSubtitle, GreetingMessage: setting.GreetingMessage, ThemeColor: setting.ThemeColor}
}

// websiteChannelAccessFromRecord 转换网站渠道允许使用的网站。
func websiteChannelAccessFromRecord(setting *channelaction.WebsiteChannelSettingRecord) WebsiteChannelAccess {
	return WebsiteChannelAccess{AllowedHosts: setting.AllowedEmbedHosts}
}

// channelInput 转换消息渠道修改输入。
func channelInput(input MessageChannelInput) channelaction.MessageChannelInput {
	return channelaction.MessageChannelInput{
		Name: input.Name, Description: input.Description, DefaultLocale: domain.Locale(input.DefaultLocale),
		NewConversationTarget: channelRoutingTargetInput(input.NewConversationTarget),
		FallbackTarget:        channelRoutingTargetInput(input.FallbackTarget),
	}
}

// createChannelInput 转换消息渠道创建输入。
func createChannelInput(input CreateMessageChannelInput) channelaction.CreateMessageChannelInput {
	return channelaction.CreateMessageChannelInput{
		MessageChannelInput: channelInput(input.MessageChannelInput),
		Type:                domain.ChannelType(input.Type),
	}
}

// channelRoutingTargetInput 转换渠道会话流转目标输入。
func channelRoutingTargetInput(target ChannelRoutingTarget) channelaction.RoutingTarget {
	return channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetType(target.Type), ID: target.ID}
}

// channelRoutingTargetFromRecord 转换渠道记录中的会话流转目标。
func channelRoutingTargetFromRecord(targetType string, targetID *string) ChannelRoutingTarget {
	id := ""
	if targetID != nil {
		id = *targetID
	}
	return ChannelRoutingTarget{Type: ChannelRoutingTargetType(targetType), ID: id}
}

// channelFieldKeys 把渠道校验错误码映射为本地化文案键。
func channelFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		channelaction.ValidationTypeInvalid:          cervii18n.FieldChannelTypeInvalid,
		channelaction.ValidationNameRequired:         cervii18n.FieldChannelNameRequired,
		channelaction.ValidationNameTooLong:          cervii18n.FieldChannelNameTooLong,
		channelaction.ValidationDescriptionTooLong:   cervii18n.FieldChannelDescriptionTooLong,
		channelaction.ValidationDefaultLocaleInvalid: cervii18n.FieldChannelDefaultLocaleInvalid,
		channelaction.ValidationRoutingTargetInvalid: cervii18n.FieldChannelRoutingTargetInvalid,
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
