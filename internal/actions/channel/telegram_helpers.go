//go:build server

package channel

import (
	"context"
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const telegramAdapterName = "telegram_bot_api"

// withTelegramChannelLock 在专用数据库连接上串行执行单个渠道的完整远端生命周期。
func withTelegramChannelLock(ctx context.Context, db *bun.DB, channelID string, execute func(bun.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire Telegram channel connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended(?, 0))", channelID); err != nil {
		return fmt.Errorf("lock Telegram channel: %w", err)
	}
	defer releaseTelegramChannelLock(conn, channelID)
	return execute(conn)
}

// releaseTelegramChannelLock 释放会话锁，失败时丢弃底层连接避免锁泄漏进连接池。
func releaseTelegramChannelLock(conn bun.Conn, channelID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtextextended(?, 0))", channelID); err == nil {
		return
	}
	slog.Error("释放 Telegram 渠道锁失败", "channel_id", channelID)
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}

// withTelegramBotLocks 按固定顺序锁定 Bot，串行化跨渠道的 Webhook 生命周期。
func withTelegramBotLocks(ctx context.Context, conn bun.Conn, botIDs []int64, execute func() error) error {
	unique := make(map[int64]struct{}, len(botIDs))
	ordered := make([]int64, 0, len(botIDs))
	for _, botID := range botIDs {
		if botID <= 0 {
			continue
		}
		if _, exists := unique[botID]; exists {
			continue
		}
		unique[botID] = struct{}{}
		ordered = append(ordered, botID)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	for index, botID := range ordered {
		key := strconv.FormatInt(botID, 10)
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended(?, 1))", key); err != nil {
			releaseTelegramBotLocks(conn, ordered[:index])
			return fmt.Errorf("lock Telegram bot: %w", err)
		}
	}
	defer releaseTelegramBotLocks(conn, ordered)
	return execute()
}

// releaseTelegramBotLocks 逆序释放 Bot 会话锁，失败时丢弃底层连接。
func releaseTelegramBotLocks(conn bun.Conn, botIDs []int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, botID := range slices.Backward(botIDs) {
		key := strconv.FormatInt(botID, 10)
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtextextended(?, 1))", key); err != nil {
			slog.Error("释放 Telegram Bot 锁失败", "bot_id", botID)
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			return
		}
	}
}

// runTelegramGetMe 使用通用连接探测语义读取机器人身份。
func runTelegramGetMe(ctx context.Context, runner *connectiontest.Runner, api telegram.BotAPI, token string) (telegram.Bot, error) {
	bot := telegram.Bot{}
	err := runner.Run(ctx, telegramTarget(), connectiontest.ProbeFunc(func(testCtx context.Context) error {
		var err error
		bot, err = api.GetMe(testCtx, token)
		return err
	}))
	return bot, err
}

// runTelegramSetWebhook 使用独立超时注册 Webhook。
func runTelegramSetWebhook(ctx context.Context, runner *connectiontest.Runner, api telegram.BotAPI, token, webhookURL, secret string) error {
	return runner.Run(ctx, telegramTarget(), connectiontest.ProbeFunc(func(testCtx context.Context) error {
		return api.SetWebhook(testCtx, token, telegram.Webhook{URL: webhookURL, Secret: secret})
	}))
}

// runTelegramDeleteWebhook 使用独立超时清理 Webhook。
func runTelegramDeleteWebhook(ctx context.Context, runner *connectiontest.Runner, api telegram.BotAPI, token string) error {
	return runner.Run(ctx, telegramTarget(), connectiontest.ProbeFunc(func(testCtx context.Context) error {
		return api.DeleteWebhook(testCtx, token)
	}))
}

// telegramTarget 返回可安全记录的 Telegram 探测目标。
func telegramTarget() connectiontest.Target {
	return connectiontest.Target{
		Category: connectiontest.CategoryTelegram,
		Adapter:  telegramAdapterName,
		Location: connectiontest.LocationServer,
	}
}

// newTelegramWebhookSecret 生成 Telegram 允许字符范围内的随机 Secret。
func newTelegramWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// telegramBotDisplayName 合并 getMe 返回的机器人姓名。
func telegramBotDisplayName(bot telegram.Bot) string {
	return strings.TrimSpace(strings.TrimSpace(bot.FirstName) + " " + strings.TrimSpace(bot.LastName))
}

// telegramBotUsedByOtherChannel 判断 Bot 是否仍被另一个渠道引用。
func telegramBotUsedByOtherChannel(ctx context.Context, db bun.IDB, botID int64, channelID string) (bool, error) {
	used, err := db.NewSelect().
		Model((*servermodels.TelegramChannelSetting)(nil)).
		Where("tcs.bot_id = ?", botID).
		Where("tcs.channel_id <> ?", channelID).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check Telegram bot reuse: %w", err)
	}
	return used, nil
}

// optionalTelegramString 把空字符串转换为数据库空值。
func optionalTelegramString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
