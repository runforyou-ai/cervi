//go:build server

package channel

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// TelegramWebhookInput 定义公开回调完成认证和状态更新所需字段。
type TelegramWebhookInput struct {
	Secret       string
	MyChatMember bool
}

// ReceiveTelegramWebhookAction 认证 Telegram 回调并更新连接状态。
type ReceiveTelegramWebhookAction struct {
	db *bun.DB
}

// NewReceiveTelegramWebhookAction 创建 Telegram Webhook 接收操作。
func NewReceiveTelegramWebhookAction(db *bun.DB) *ReceiveTelegramWebhookAction {
	return &ReceiveTelegramWebhookAction{db: db}
}

// Preflight 在读取请求体前校验渠道和当前 Secret。
func (a *ReceiveTelegramWebhookAction) Preflight(ctx context.Context, channelID, secret string) error {
	if !common.ValidUUID(channelID) {
		return ErrNotFound
	}
	setting, err := loadActiveTelegramWebhookSetting(ctx, a.db, channelID, false)
	if err != nil {
		return err
	}
	return authorizeTelegramWebhook(setting, secret)
}

// Execute 在锁行后重新认证当前代次，并处理支持的 Update。
func (a *ReceiveTelegramWebhookAction) Execute(ctx context.Context, channelID string, input TelegramWebhookInput) error {
	if !common.ValidUUID(channelID) {
		return ErrNotFound
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		setting, err := loadActiveTelegramWebhookSetting(ctx, tx, channelID, true)
		if err != nil {
			return err
		}
		if err := authorizeTelegramWebhook(setting, input.Secret); err != nil {
			return err
		}
		if !input.MyChatMember {
			return ErrTelegramWebhookUnsupported
		}
		result, err := tx.NewUpdate().
			Model(setting).
			Set("webhook_status = ?", domain.TelegramWebhookStatusNormal).
			Set("webhook_connected_at = now()").
			Set("updated_at = now()").
			Where("channel_id = ?", channelID).
			Where("webhook_secret = ?", input.Secret).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update Telegram webhook status: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read Telegram webhook update count: %w", err)
		}
		if rows == 0 {
			return ErrTelegramWebhookUnauthorized
		}
		return nil
	})
	if err != nil {
		return err
	}
	slog.Info("Telegram Webhook 回调已接收", "channel_id", channelID)
	return nil
}

// loadActiveTelegramWebhookSetting 读取启用渠道当前可接收回调的设置。
func loadActiveTelegramWebhookSetting(ctx context.Context, db bun.IDB, channelID string, lock bool) (*servermodels.TelegramChannelSetting, error) {
	setting := &servermodels.TelegramChannelSetting{}
	query := db.NewSelect().
		Model(setting).
		Join("JOIN channels AS c ON c.id = tcs.channel_id").
		Where("tcs.channel_id = ?", channelID).
		Where("c.type = ?", domain.ChannelTypeTelegram).
		Where("c.enabled = TRUE").
		Where("tcs.webhook_secret IS NOT NULL")
	if lock {
		query = query.For("UPDATE OF tcs")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get active Telegram webhook: %w", err)
	}
	return setting, nil
}

// authorizeTelegramWebhook 使用常量时间比较当前 Secret。
func authorizeTelegramWebhook(setting *servermodels.TelegramChannelSetting, secret string) error {
	if setting.WebhookSecret == nil {
		return ErrTelegramWebhookUnauthorized
	}
	expected := sha256.Sum256([]byte(*setting.WebhookSecret))
	provided := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(expected[:], provided[:]) != 1 {
		return ErrTelegramWebhookUnauthorized
	}
	return nil
}
