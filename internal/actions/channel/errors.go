//go:build server

package channel

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定消息渠道。
	ErrNotFound = errors.New("message channel not found")
	// ErrTelegramBotReuseConfirmationRequired 表示保存前需要确认复用其他渠道的 Bot。
	ErrTelegramBotReuseConfirmationRequired = errors.New("Telegram bot reuse confirmation required")
	// ErrTelegramWebhookUnauthorized 表示 Telegram Webhook Secret 不匹配。
	ErrTelegramWebhookUnauthorized = errors.New("Telegram webhook unauthorized")
)
