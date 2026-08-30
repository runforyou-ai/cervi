//go:build server

package channel

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定消息渠道。
	ErrNotFound = errors.New("message channel not found")
	// ErrTelegramConnectionRequired 表示启用 Telegram 渠道前尚未保存连接配置。
	ErrTelegramConnectionRequired = errors.New("Telegram channel connection required")
	// ErrTelegramBotReuseConfirmationRequired 表示保存前需要确认复用其他渠道的 Bot。
	ErrTelegramBotReuseConfirmationRequired = errors.New("Telegram bot reuse confirmation required")
	// ErrTelegramWebhookUnauthorized 表示 Telegram Webhook Secret 不匹配。
	ErrTelegramWebhookUnauthorized = errors.New("Telegram webhook unauthorized")
	// ErrTelegramWebhookUnsupported 表示 Telegram Update 尚未支持处理。
	ErrTelegramWebhookUnsupported = errors.New("Telegram webhook update unsupported")
)
