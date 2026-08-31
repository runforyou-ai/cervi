//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

const telegramBotReuseConfirmationReason = "telegram_bot_reuse_confirmation_required"

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

// GetTelegramChannel 返回 Telegram 渠道详情。
func (b *DirectBackend) GetTelegramChannel(ctx context.Context, meta RequestMeta, channelID string) (TelegramChannel, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return TelegramChannel{}, err
	}
	detail, err := b.getTelegramChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return TelegramChannel{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelReadFailed, identity.Organization.ID, channelID)
	}
	return telegramChannelFromRecord(detail), nil
}

// TestTelegramChannelConnection 测试 Telegram 草稿 Token。
func (b *DirectBackend) TestTelegramChannelConnection(ctx context.Context, meta RequestMeta, channelID string, input TelegramChannelConnectionTestInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	err = b.testTelegramConnection.Execute(ctx, identity, channelID, channelaction.TelegramChannelConnectionTestInput{BotToken: input.BotToken})
	if err == nil {
		return nil
	}
	return b.telegramConnectionError(ctx, meta, err, cervii18n.ErrorTelegramConnectionTestFailed, identity.Organization.ID, channelID)
}

// SaveTelegramChannelConnection 保存 Telegram 机器人和 Webhook 设置。
func (b *DirectBackend) SaveTelegramChannelConnection(ctx context.Context, meta RequestMeta, channelID string, input TelegramChannelConnectionInput) (TelegramChannel, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return TelegramChannel{}, err
	}
	detail, err := b.saveTelegramConnection.Execute(ctx, identity, channelID, channelaction.TelegramChannelConnectionInput{
		BotToken: input.BotToken, WebhookBaseURL: input.WebhookBaseURL, ConfirmBotReuse: input.ConfirmBotReuse,
	})
	if err != nil {
		return TelegramChannel{}, b.telegramConnectionError(ctx, meta, err, cervii18n.ErrorTelegramConnectionSaveFailed, identity.Organization.ID, channelID)
	}
	slog.Info("Telegram 渠道连接已保存", "organization_id", identity.Organization.ID, "channel_id", channelID)
	return telegramChannelFromRecord(detail), nil
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
	current, err := b.getMessageChannel.Execute(ctx, identity, channelID)
	if err != nil {
		return MessageChannelSummary{}, b.channelError(ctx, meta, err, cervii18n.ErrorChannelUpdateFailed, identity.Organization.ID, channelID)
	}
	var channel *channelaction.MessageChannelRecord
	if current.Type == string(domain.ChannelTypeTelegram) {
		channel, err = b.updateTelegramChannelStatus.Execute(ctx, identity, channelID, enabled)
	} else {
		channel, err = b.updateMessageChannelStatus.Execute(ctx, identity, channelID, enabled)
	}
	if err != nil {
		if errors.Is(err, channelaction.ErrTelegramConnectionRequired) {
			return MessageChannelSummary{}, InvalidError(meta, cervii18n.ErrorTelegramConnectionRequired, nil)
		}
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
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
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

// telegramChannelFromRecord 转换 Telegram 渠道详情。
func telegramChannelFromRecord(detail *channelaction.TelegramChannelDetail) TelegramChannel {
	connection := detail.Connection
	var botID *string
	if connection.BotID != nil {
		value := strconv.FormatInt(*connection.BotID, 10)
		botID = &value
	}
	var status *TelegramWebhookStatus
	if connection.WebhookStatus != nil {
		value := TelegramWebhookStatus(*connection.WebhookStatus)
		status = &value
	}
	return TelegramChannel{
		MessageChannelSummary: messageChannelFromRecord(&detail.MessageChannelRecord),
		Connection: TelegramChannelConnection{
			BotToken: connection.BotToken, BotID: botID, BotUsername: connection.BotUsername,
			BotDisplayName: connection.BotDisplayName, WebhookURL: connection.WebhookURL,
			WebhookSecret: connection.WebhookSecret, WebhookStatus: status,
		},
	}
}

// telegramConnectionError 转换 Telegram 连接校验和外部访问错误。
func (b *DirectBackend) telegramConnectionError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, channelID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, channelFieldKeys(validationError.Fields))
	}
	if errors.Is(err, channelaction.ErrNotFound) || errors.Is(err, common.ErrIdentityInvalid) {
		return b.channelError(ctx, meta, err, failureKey, organizationID, channelID)
	}
	if errors.Is(err, channelaction.ErrTelegramBotReuseConfirmationRequired) {
		return ConflictError(meta, cervii18n.FieldTelegramBotInUse, telegramBotReuseConfirmationReason)
	}
	_, kind, classified := connectiontest.Details(err)
	if !classified {
		return b.channelError(ctx, meta, err, failureKey, organizationID, channelID)
	}
	switch kind {
	case connectiontest.FailureInvalidConfig, connectiontest.FailureUnauthorized, connectiontest.FailureForbidden, connectiontest.FailureNotFound:
		return InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"botToken": cervii18n.FieldTelegramBotTokenInvalid})
	default:
		return UnavailableError(meta, failureKey, nil)
	}
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
		channelaction.ValidationTypeInvalid:            cervii18n.FieldChannelTypeInvalid,
		channelaction.ValidationNameRequired:           cervii18n.FieldChannelNameRequired,
		channelaction.ValidationNameTooLong:            cervii18n.FieldChannelNameTooLong,
		channelaction.ValidationDescriptionTooLong:     cervii18n.FieldChannelDescriptionTooLong,
		channelaction.ValidationDefaultLocaleInvalid:   cervii18n.FieldChannelDefaultLocaleInvalid,
		channelaction.ValidationRoutingTargetInvalid:   cervii18n.FieldChannelRoutingTargetInvalid,
		channelaction.ValidationChatTitleRequired:      cervii18n.FieldChannelChatTitleRequired,
		channelaction.ValidationChatTitleTooLong:       cervii18n.FieldChannelChatTitleTooLong,
		channelaction.ValidationChatSubtitleTooLong:    cervii18n.FieldChannelChatSubtitleTooLong,
		channelaction.ValidationGreetingTooLong:        cervii18n.FieldChannelGreetingTooLong,
		channelaction.ValidationThemeColorInvalid:      cervii18n.FieldChannelThemeColorInvalid,
		channelaction.ValidationAllowedHostsTooMany:    cervii18n.FieldChannelAllowedHostsTooMany,
		channelaction.ValidationAllowedHostInvalid:     cervii18n.FieldChannelAllowedHostInvalid,
		channelaction.ValidationTelegramTokenRequired:  cervii18n.FieldTelegramBotTokenRequired,
		channelaction.ValidationTelegramTokenTooLong:   cervii18n.FieldTelegramBotTokenTooLong,
		channelaction.ValidationTelegramTokenInvalid:   cervii18n.FieldTelegramBotTokenInvalid,
		channelaction.ValidationTelegramBaseURLInvalid: cervii18n.FieldTelegramWebhookBaseURLInvalid,
	}
	return translateValidationFields(fields, keys)
}
