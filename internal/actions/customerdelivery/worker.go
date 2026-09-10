//go:build server

package customerdelivery

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strconv"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/channelmessage"
	"github.com/runforyou-ai/cervi/internal/actions/channelstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	models "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

const sendTimeout = 20 * time.Second
const leaseDuration = 45 * time.Second
const uncertaintyWindow = 30 * time.Second

// Worker 按渠道身份队头投递消息并恢复中断的发送。
type Worker struct {
	db       *bun.DB
	sender   telegram.TextSender
	enqueuer servertask.Enqueuer
}

// NewWorker 创建持久投递执行器。
func NewWorker(db *bun.DB, sender telegram.TextSender, enqueuer servertask.Enqueuer) *Worker {
	return &Worker{db: db, sender: sender, enqueuer: enqueuer}
}

// Scan 为到期队头补充幂等唤醒，HTTP 发送由独立任务执行。
func (w *Worker) Scan(ctx context.Context, _ struct{}) error {
	var ids []string
	err := w.db.NewSelect().TableExpr("customer_message_deliveries AS d").Column("d.id").
		Join("JOIN channels AS ch ON ch.id = d.channel_id AND ch.organization_id = d.organization_id").
		Where("(d.status IN ('pending','retry_wait') AND d.available_at <= now()) OR (d.status = 'sending' AND d.lease_expires_at <= now()) OR (d.status = 'uncertain' AND d.uncertain_until <= now())").
		Where("d.status IN ('sending', 'uncertain') OR (ch.enabled AND NOT EXISTS (SELECT 1 FROM customer_channel_send_gates AS gate WHERE gate.organization_id = d.organization_id AND gate.channel_id = d.channel_id AND gate.flood_wait_until > now()))").
		Where("NOT EXISTS (SELECT 1 FROM customer_message_deliveries AS earlier WHERE earlier.organization_id = d.organization_id AND earlier.channel_id = d.channel_id AND earlier.contact_channel_identity_id = d.contact_channel_identity_id AND earlier.position < d.position AND earlier.status IN ('pending','retry_wait','sending','uncertain'))").
		OrderExpr("d.updated_at, d.id").Limit(100).Scan(ctx, &ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := w.enqueuer.Enqueue(ctx, SendActionName, Input{DeliveryID: id}, servertask.EnqueueOptions{IdempotencyKey: "cdeliv-item:" + id}); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("客户消息投递扫描失败", "delivery_id", id, "error", err)
		}
	}
	return nil
}

// Execute 串行化配置与投递，在短事务之外调用 Telegram。
//
// 所属企业由投递记录确定，认领之后的查询都按该企业限定。
func (w *Worker) Execute(ctx context.Context, input Input) error {
	var channelID string
	err := w.db.NewSelect().Model((*models.CustomerMessageDelivery)(nil)).Column("channel_id").Where("id = ?", input.DeliveryID).Scan(ctx, &channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return channelstate.TryTelegramLock(ctx, w.db, channelID, func(conn bun.Conn) error {
		delivery, token, recipient, body, err := w.claim(ctx, conn, input.DeliveryID)
		if err != nil || delivery == nil {
			return err
		}
		sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		messageID, sendErr := w.sender.SendText(sendCtx, token, telegram.TextMessage{ChatID: recipient, Body: body, ReplyMessageID: delivery.ReplyProviderMessageID})
		cancel()
		// 请求结束或服务关闭后使用独立上下文保存平台结果。
		saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer saveCancel()
		return w.finish(saveCtx, conn, delivery, recipient, messageID, sendErr)
	})
}

// claim 只认领身份管道的最小非终态投递。
func (w *Worker) claim(ctx context.Context, conn bun.Conn, id string) (*models.CustomerMessageDelivery, string, string, string, error) {
	var claimed *models.CustomerMessageDelivery
	var token, recipient, body string
	err := conn.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		delivery := &models.CustomerMessageDelivery{}
		if err := tx.NewSelect().Model(delivery).Where("cmd.id = ?", id).Scan(ctx); err != nil {
			return err
		}
		// 身份行统一协调消息生产、扫描和人工重新排队。
		if _, err := tx.ExecContext(ctx, "SELECT id FROM contact_channel_identities WHERE id = ? AND organization_id = ? FOR UPDATE", delivery.ContactChannelIdentityID, delivery.OrganizationID); err != nil {
			return err
		}
		if err := tx.NewSelect().Model(delivery).WherePK().Where("cmd.organization_id = ?", delivery.OrganizationID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		now := time.Now().UTC()
		switch delivery.Status {
		case domain.CustomerDeliverySending:
			if delivery.LeaseExpiresAt != nil && !delivery.LeaseExpiresAt.After(now) {
				until := now.Add(uncertaintyWindow)
				delivery.Status, delivery.UncertainUntil, delivery.LastError = domain.CustomerDeliveryUncertain, &until, "unknown_result"
				slog.Warn("客户消息发送认领过期，等待人工确认", "delivery_id", delivery.ID, "channel_id", delivery.ChannelID)
				delivery.LeaseWorker, delivery.LeaseExpiresAt = nil, nil
				return saveDelivery(ctx, tx, delivery)
			}
			return nil
		case domain.CustomerDeliveryUncertain:
			if delivery.UncertainUntil != nil && !delivery.UncertainUntil.After(now) {
				delivery.Status = domain.CustomerDeliveryNeedsReview
				return saveDelivery(ctx, tx, delivery)
			}
			return nil
		case domain.CustomerDeliveryPending, domain.CustomerDeliveryRetryWait:
			if delivery.AvailableAt.After(now) {
				return nil
			}
		default:
			return nil
		}
		blocked, err := tx.NewSelect().TableExpr("customer_message_deliveries").
			Where("organization_id = ? AND channel_id = ? AND contact_channel_identity_id = ? AND position < ?", delivery.OrganizationID, delivery.ChannelID, delivery.ContactChannelIdentityID, delivery.Position).
			Where("status IN ('pending','retry_wait','sending','uncertain')").Exists(ctx)
		if err != nil || blocked {
			return err
		}
		var route struct {
			Enabled   bool    `bun:"enabled"`
			BotID     *int64  `bun:"bot_id"`
			Token     *string `bun:"bot_token"`
			Recipient string  `bun:"external_id"`
			Body      string  `bun:"body"`
		}
		if err := tx.NewSelect().TableExpr("channels AS ch").ColumnExpr("ch.enabled, tcs.bot_id, tcs.bot_token, cci.external_id, msg.body").
			Join("JOIN telegram_channel_settings AS tcs ON tcs.channel_id = ch.id AND tcs.organization_id = ch.organization_id").
			Join("JOIN contact_channel_identities AS cci ON cci.channel_id = ch.id AND cci.organization_id = ch.organization_id AND cci.id = ?", delivery.ContactChannelIdentityID).
			Join("JOIN messages AS msg ON msg.id = ? AND msg.organization_id = ch.organization_id AND msg.conversation_id = ?", delivery.MessageID, delivery.ConversationID).
			Where("ch.id = ? AND ch.organization_id = ? AND ch.type = ?", delivery.ChannelID, delivery.OrganizationID, domain.ChannelTypeTelegram).Scan(ctx, &route); err != nil {
			return err
		}
		if route.BotID == nil || *route.BotID != delivery.BotID {
			delivery.Status, delivery.LastError = domain.CustomerDeliveryFailed, "bot_changed"
			slog.Warn("客户消息因机器人变化停止投递", "delivery_id", delivery.ID, "channel_id", delivery.ChannelID)
			return saveDelivery(ctx, tx, delivery)
		}
		if !route.Enabled {
			if delivery.LastError != "channel_disabled" {
				slog.Info("客户消息因渠道停用暂停投递", "delivery_id", delivery.ID, "channel_id", delivery.ChannelID)
			}
			delivery.LastError = "channel_disabled"
			return saveDelivery(ctx, tx, delivery)
		}
		if route.Token == nil || *route.Token == "" {
			delivery.Status, delivery.LastError = domain.CustomerDeliveryFailed, "invalid_token"
			slog.Warn("客户消息因缺少机器人凭据停止投递", "delivery_id", delivery.ID, "channel_id", delivery.ChannelID)
			return saveDelivery(ctx, tx, delivery)
		}
		blocked, err = tx.NewSelect().TableExpr("customer_channel_send_gates").Where("organization_id = ? AND channel_id = ? AND flood_wait_until > now()", delivery.OrganizationID, delivery.ChannelID).Exists(ctx)
		if err != nil || blocked {
			return err
		}
		blocked, err = tx.NewSelect().TableExpr("customer_message_deliveries").Where("organization_id = ? AND channel_id = ? AND status = 'sending'", delivery.OrganizationID, delivery.ChannelID).Exists(ctx)
		if err != nil || blocked {
			return err
		}
		worker, expires := uuid.NewV7().String(), now.Add(leaseDuration)
		delivery.Status, delivery.LeaseWorker, delivery.LeaseExpiresAt = domain.CustomerDeliverySending, &worker, &expires
		delivery.Attempt++
		delivery.LastError = ""
		if err := saveDelivery(ctx, tx, delivery); err != nil {
			return err
		}
		claimed, token, recipient, body = delivery, *route.Token, route.Recipient, route.Body
		return nil
	})
	return claimed, token, recipient, body, err
}

// finish 保存带认领标识的平台结果，未知结果绝不自动重发。
func (w *Worker) finish(ctx context.Context, conn bun.Conn, delivery *models.CustomerMessageDelivery, recipient string, messageID int64, sendErr error) error {
	return conn.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// 与入站共用渠道身份锁，使映射写入和迟到引用关联串行提交。
		if _, err := tx.ExecContext(ctx, "SELECT id FROM contact_channel_identities WHERE id = ? AND organization_id = ? FOR UPDATE", delivery.ContactChannelIdentityID, delivery.OrganizationID); err != nil {
			return err
		}
		worker := delivery.LeaseWorker
		current := &models.CustomerMessageDelivery{}
		if err := tx.NewSelect().Model(current).Where("cmd.organization_id = ? AND cmd.id = ?", delivery.OrganizationID, delivery.ID).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if current.Status != domain.CustomerDeliverySending || current.LeaseWorker == nil || worker == nil || *current.LeaseWorker != *worker {
			return nil
		}
		now := time.Now().UTC()
		current.LeaseWorker, current.LeaseExpiresAt = nil, nil
		current.Status = domain.CustomerDeliverySent
		if sendErr == nil && messageID > 0 {
			current.ProviderMessageID, current.SentAt = &messageID, &now
			// 使用本次实际发送的聊天目标建立平台映射。
			if err := channelmessage.Record(ctx, tx, &models.ChannelMessage{
				MessageID: current.MessageID, OrganizationID: current.OrganizationID, ConversationID: current.ConversationID,
				ChannelID: current.ChannelID, ProviderAccountID: strconv.FormatInt(current.BotID, 10), ProviderConversationID: recipient, ProviderMessageID: strconv.FormatInt(messageID, 10),
			}); err != nil {
				return err
			}
		} else {
			var failure *telegram.SendError
			if !errors.As(sendErr, &failure) {
				failure = &telegram.SendError{Code: "unknown_result"}
			}
			current.LastError = failure.Code
			switch failure.Code {
			case "rate_limited":
				current.Status, current.AvailableAt = domain.CustomerDeliveryRetryWait, now.Add(failure.RetryAfter)
				if _, err := tx.ExecContext(ctx, `INSERT INTO customer_channel_send_gates (channel_id, organization_id, flood_wait_until)
     VALUES (?, ?, ?) ON CONFLICT (channel_id) DO UPDATE SET flood_wait_until = GREATEST(customer_channel_send_gates.flood_wait_until, EXCLUDED.flood_wait_until), updated_at = now()`, current.ChannelID, current.OrganizationID, current.AvailableAt); err != nil {
					return err
				}
			case "invalid_token", "invalid_recipient", "invalid_message", "recipient_unavailable", "message_rejected":
				current.Status = domain.CustomerDeliveryFailed
			default:
				until := now.Add(uncertaintyWindow)
				current.Status, current.UncertainUntil = domain.CustomerDeliveryUncertain, &until
			}
		}
		if err := saveDelivery(ctx, tx, current); err != nil {
			return err
		}
		slog.Info("客户消息投递结果已保存", "delivery_id", current.ID, "channel_id", current.ChannelID, "status", current.Status, "attempt", current.Attempt, "error_code", current.LastError)
		return nil
	})
}

// saveDelivery 更新投递状态与运行字段。
func saveDelivery(ctx context.Context, db bun.IDB, delivery *models.CustomerMessageDelivery) error {
	delivery.UpdatedAt = time.Now().UTC()
	_, err := db.NewUpdate().Model(delivery).WherePK().Where("organization_id = ?", delivery.OrganizationID).Exec(ctx)
	return err
}
