//go:build server

package customerdelivery

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	models "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// Manager 读取企业会话投递并执行人工处理。
type Manager struct {
	db       *bun.DB
	enqueuer servertask.TxEnqueuer
}

// NewManager 创建投递管理操作。
func NewManager(db *bun.DB, enqueuer servertask.TxEnqueuer) *Manager {
	return &Manager{db: db, enqueuer: enqueuer}
}

// Record 包含投递事实和当前渠道下允许的成员操作。
type Record struct {
	models.CustomerMessageDelivery `bun:",inherit"`
	CanRetry                       bool `bun:"can_retry"`
	Paused                         bool `bun:"paused"`
}

// List 读取当前企业客户会话中指定消息的投递状态。
func (m *Manager) List(ctx context.Context, organizationID, conversationID string, messageIDs []string) ([]Record, error) {
	exists, err := m.db.NewSelect().TableExpr("customer_conversations").Where("organization_id = ? AND conversation_id = ?", organizationID, conversationID).Exists(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUnavailable
	}
	rows := make([]Record, 0)
	if len(messageIDs) == 0 {
		return rows, nil
	}
	err = m.db.NewSelect().Model(&rows).ColumnExpr("cmd.*").
		ColumnExpr("cmd.status IN ('failed','needs_review') AND cmd.last_error <> 'bot_changed' AND ch.enabled AND tcs.bot_id = cmd.bot_id AS can_retry").
		ColumnExpr("cmd.status IN ('pending','retry_wait') AND NOT ch.enabled AS paused").
		Join("JOIN channels AS ch ON ch.id = cmd.channel_id AND ch.organization_id = cmd.organization_id").
		Join("JOIN telegram_channel_settings AS tcs ON tcs.channel_id = ch.id AND tcs.organization_id = ch.organization_id").Where("cmd.organization_id = ? AND cmd.conversation_id = ?", organizationID, conversationID).Where("cmd.message_id IN (?)", bun.In(messageIDs)).OrderExpr("cmd.position").Scan(ctx)
	return rows, err
}

// Resolve 锁定渠道身份后重新校验投递状态，人工重试排到队尾。
func (m *Manager) Resolve(ctx context.Context, identity *models.Identity, conversationID, deliveryID string, resolution domain.CustomerDeliveryResolution, confirmDuplicateRisk bool) error {
	return m.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		route, err := Prepare(ctx, tx, identity.Organization.ID, conversationID)
		if err != nil {
			return err
		}
		delivery := &models.CustomerMessageDelivery{}
		err = tx.NewSelect().Model(delivery).Where("cmd.id = ? AND cmd.organization_id = ? AND cmd.conversation_id = ?", deliveryID, identity.Organization.ID, conversationID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUnavailable
		}
		if err != nil {
			return err
		}
		review := delivery.Status == domain.CustomerDeliveryNeedsReview
		if !review && delivery.Status != domain.CustomerDeliveryFailed {
			return ErrConflict
		}
		now := time.Now().UTC()
		switch resolution {
		case domain.CustomerDeliveryRetry:
			if (review || delivery.LastError == "unknown_result") && !confirmDuplicateRisk {
				return ErrConflict
			}
			if delivery.LastError == "bot_changed" || !route.Enabled || route.BotID == nil || *route.BotID != delivery.BotID {
				return ErrConflict
			}
			if err := tx.NewSelect().TableExpr("customer_message_deliveries").ColumnExpr("MAX(position) + 1").
				Where("organization_id = ? AND channel_id = ? AND contact_channel_identity_id = ?", delivery.OrganizationID, delivery.ChannelID, delivery.ContactChannelIdentityID).Scan(ctx, &delivery.Position); err != nil {
				return err
			}
			delivery.Status, delivery.AvailableAt = domain.CustomerDeliveryPending, now
			delivery.UncertainUntil, delivery.LastError = nil, ""
		case domain.CustomerDeliveryConfirmSent:
			if !review {
				return ErrConflict
			}
			delivery.Status, delivery.SentAt = domain.CustomerDeliverySent, &now
			delivery.LastError = "manually_confirmed"
		case domain.CustomerDeliveryConfirmFailed:
			if !review {
				return ErrConflict
			}
			delivery.Status = domain.CustomerDeliveryFailed
		default:
			return ErrConflict
		}
		if err := saveDelivery(ctx, tx, delivery); err != nil {
			return err
		}
		if resolution == domain.CustomerDeliveryRetry && m.enqueuer != nil {
			if _, err := m.enqueuer.EnqueueIn(ctx, tx, SendActionName, Input{DeliveryID: delivery.ID}, servertask.EnqueueOptions{IdempotencyKey: "cdeliv-item:" + delivery.ID}); err != nil {
				return err
			}
		}
		slog.Info("客户消息投递已人工处理", "organization_id", identity.Organization.ID, "delivery_id", delivery.ID, "identity_id", identity.OrganizationIdentity.ID, "resolution", resolution)
		return nil
	})
}
