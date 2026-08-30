//go:build server

package channel

import (
	"context"
	"fmt"
	"log/slog"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// UpdateTelegramChannelStatusAction 修改 Telegram 渠道状态并维护 Webhook。
type UpdateTelegramChannelStatusAction struct {
	db     *bun.DB
	runner *connectiontest.Runner
	api    telegram.BotAPI
}

// NewUpdateTelegramChannelStatusAction 创建 Telegram 渠道状态操作。
func NewUpdateTelegramChannelStatusAction(db *bun.DB, runner *connectiontest.Runner, api telegram.BotAPI) *UpdateTelegramChannelStatusAction {
	return &UpdateTelegramChannelStatusAction{db: db, runner: runner, api: api}
}

// Execute 修改渠道状态，并在事务提交后注册或删除 Webhook。
func (a *UpdateTelegramChannelStatusAction) Execute(ctx context.Context, identity *servermodels.Identity, channelID string, enabled bool) (*MessageChannelRecord, error) {
	if !common.ValidUUID(channelID) {
		return nil, ErrNotFound
	}
	var output *MessageChannelRecord
	err := withTelegramChannelLock(ctx, a.db, channelID, func(conn bun.Conn) error {
		if err := identityaction.Validate(ctx, conn, identity); err != nil {
			return err
		}
		current, err := loadTelegramChannelDetail(ctx, conn, identity.Organization.ID, channelID, false)
		if err != nil {
			return err
		}
		var botIDs []int64
		if current.Connection.BotID != nil {
			botIDs = append(botIDs, *current.Connection.BotID)
		}

		return withTelegramBotLocks(ctx, conn, botIDs, func() error {
			var token string
			var botUsedByOtherChannel bool
			var webhookURL string
			var secret string
			err := conn.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				detail, err := loadTelegramChannelDetail(ctx, tx, identity.Organization.ID, channelID, true)
				if err != nil {
					return err
				}
				token = detail.Connection.BotToken
				if !enabled && detail.Connection.BotID != nil {
					botUsedByOtherChannel, err = telegramBotUsedByOtherChannel(ctx, tx, *detail.Connection.BotID, channelID)
					if err != nil {
						return err
					}
				}
				if enabled {
					if token == "" || detail.Connection.WebhookBaseURL == "" {
						return ErrTelegramConnectionRequired
					}
					webhookURL, err = telegramWebhookURL(detail.Connection.WebhookBaseURL, channelID)
					if err != nil {
						return ErrTelegramConnectionRequired
					}
					secret, err = newTelegramWebhookSecret()
					if err != nil {
						return fmt.Errorf("generate Telegram webhook secret: %w", err)
					}
				}

				channel := &servermodels.Channel{ID: channelID}
				result, err := tx.NewUpdate().
					Model(channel).
					Set("enabled = ?", enabled).
					Set("updated_at = now()").
					Where("id = ?", channelID).
					Where("organization_id = ?", identity.Organization.ID).
					Where("type = ?", domain.ChannelTypeTelegram).
					Exec(ctx)
				if err != nil {
					return err
				}
				rows, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if rows == 0 {
					return ErrNotFound
				}

				setting := &servermodels.TelegramChannelSetting{ChannelID: channelID}
				query := tx.NewUpdate().
					Model(setting).
					Set("updated_at = now()").
					Set("webhook_connected_at = NULL").
					Where("channel_id = ?", channelID).
					Where("organization_id = ?", identity.Organization.ID)
				if enabled {
					query = query.
						Set("webhook_secret = ?", secret).
						Set("webhook_status = ?", domain.TelegramWebhookStatusWaiting)
				} else {
					query = query.
						Set("webhook_secret = NULL").
						Set("webhook_status = NULL")
				}
				result, err = query.Exec(ctx)
				if err != nil {
					return err
				}
				rows, err = result.RowsAffected()
				if err != nil {
					return err
				}
				if rows == 0 {
					return ErrNotFound
				}
				return nil
			})
			if err != nil {
				return err
			}

			if enabled {
				if err := runTelegramSetWebhook(ctx, a.runner, a.api, token, webhookURL, secret); err != nil {
					logTelegramRemoteFailure("注册 Telegram Webhook 失败", channelID, err)
				} else {
					slog.Info("Telegram Webhook 注册成功", "channel_id", channelID)
				}
			} else if token != "" && !botUsedByOtherChannel {
				if err := runTelegramDeleteWebhook(ctx, a.runner, a.api, token); err != nil {
					logTelegramRemoteFailure("删除 Telegram Webhook 失败", channelID, err)
				}
			}
			detail, err := loadTelegramChannelDetail(ctx, conn, identity.Organization.ID, channelID, false)
			if err != nil {
				return err
			}
			output = &detail.MessageChannelRecord
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}
