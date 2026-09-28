//go:build server

package customerchat

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/actions/channelmessage"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// ErrTelegramWebhookUnauthorized 表示 Telegram Webhook Secret 不匹配。
var ErrTelegramWebhookUnauthorized = errors.New("Telegram webhook unauthorized")

// TelegramWebhookInput 定义公开回调完成认证和状态更新所需字段。
type TelegramWebhookInput struct {
	Secret       string
	UpdateID     int64
	MyChatMember bool
	Message      *telegram.InboundMessage
}

// ReceiveTelegramWebhookAction 认证 Telegram 回调并更新连接状态。
type ReceiveTelegramWebhookAction struct {
	db             *bun.DB
	agentScheduler conversationaction.CustomerAgentMessageScheduler
	mediaBackend   fileaction.StorageBackendResolver
	tasks          servertask.TxEnqueuer
}

// NewReceiveTelegramWebhookAction 创建 Telegram Webhook 接收操作，mediaBackend 决定入站媒体的存储类型，tasks 在入站事务内投递媒体取回与头像同步任务。
func NewReceiveTelegramWebhookAction(db *bun.DB, agentScheduler conversationaction.CustomerAgentMessageScheduler, mediaBackend fileaction.StorageBackendResolver, tasks servertask.TxEnqueuer) *ReceiveTelegramWebhookAction {
	return &ReceiveTelegramWebhookAction{db: db, agentScheduler: agentScheduler, mediaBackend: mediaBackend, tasks: tasks}
}

// Preflight 在读取请求体前校验渠道和当前 Secret。
func (a *ReceiveTelegramWebhookAction) Preflight(ctx context.Context, channelID, secret string) error {
	if !common.ValidUUID(channelID) {
		return channelaction.ErrNotFound
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
		return channelaction.ErrNotFound
	}
	var ignoredConflict bool
	err := realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		setting, err := loadActiveTelegramWebhookSetting(ctx, tx, channelID, true)
		if err != nil {
			return err
		}
		if err := authorizeTelegramWebhook(setting, input.Secret); err != nil {
			return err
		}
		if input.Message != nil {
			channel := &servermodels.Channel{}
			if err := tx.NewSelect().Model(channel).
				Where("c.id = ?", channelID).
				Where("c.organization_id = ?", setting.OrganizationID).
				Where("c.type = ?", domain.ChannelTypeTelegram).
				Where("c.enabled = TRUE").
				Scan(ctx); err != nil {
				return fmt.Errorf("load Telegram webhook channel: %w", err)
			}
			displayName := input.Message.DisplayName
			platformMessage := &channelmessage.Inbound{
				AccountID: strconv.FormatInt(*setting.BotID, 10), ConversationID: strconv.FormatInt(input.Message.ChatID, 10), MessageID: strconv.FormatInt(input.Message.MessageID, 10),
			}
			if reply := input.Message.Reply; reply != nil {
				platformMessage.Reply = &channelmessage.Reply{MessageID: strconv.FormatInt(reply.MessageID, 10), Body: reply.Body, SenderName: reply.SenderName, SenderIsBot: reply.SenderIsBot}
			}
			inbound := InboundCustomerMessageInput{
				ExternalID: strconv.FormatInt(input.Message.SenderID, 10), DisplayName: &displayName,
				ChannelMessage:     platformMessage,
				SingleConversation: true, Body: input.Message.Body,
				IdempotencyKey: "chmsg:" + channelID + ":tg:" + strconv.FormatInt(*setting.BotID, 10) + ":" + strconv.FormatInt(input.Message.ChatID, 10) + ":" + strconv.FormatInt(input.Message.MessageID, 10),
				OriginatedAt:   input.Message.OriginatedAt, SourceOrder: input.Message.MessageID,
			}
			// 媒体按企业当前存储配置建立取回中的文件记录，内容由取回任务写入。
			if media := input.Message.Media; media != nil {
				backend, err := a.mediaBackend(ctx, channel.OrganizationID)
				if err != nil {
					return fmt.Errorf("resolve Telegram media storage: %w", err)
				}
				inbound.ExternalMedia = &InboundExternalMedia{
					ExternalID: media.UniqueID, FileName: media.FileName, ContentType: media.ContentType, ByteSize: media.ByteSize,
					ImageWidth: media.Width, ImageHeight: media.Height, StorageBackend: backend,
				}
			}
			received, err := ReceiveInboundCustomerMessage(ctx, tx, a.tasks, channel, inbound)
			if err != nil {
				var conflict *conversationaction.ConflictError
				if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
					return err
				}
				ignoredConflict = true
			} else {
				// 仅新入站消息触发 AI 客服，回调重放不追加运行输入；取回中的媒体由取回任务在终态时调度。
				if received.Inserted && received.Attachment != nil && received.Attachment.TransferStatus == domain.MessageAttachmentTransferPending {
					if _, err := a.tasks.EnqueueIn(ctx, tx, RetrieveTelegramMediaActionName, RetrieveTelegramMediaInput{
						OrganizationID: channel.OrganizationID, ChannelID: channelID, ConversationID: received.Message.ConversationID,
						MessageID: received.Message.ID, FileID: received.Attachment.ID, BotID: *setting.BotID, TelegramFileID: input.Message.Media.FileID,
					}, servertask.EnqueueOptions{
						Queue: "files", MaxAttempts: telegramMediaRetrieveMaxAttempts,
						IdempotencyKey: "tgmedia:" + received.Attachment.ID, TriggerType: servertask.TriggerBusiness,
					}); err != nil {
						return fmt.Errorf("enqueue Telegram media retrieval: %w", err)
					}
				} else if received.Inserted {
					if _, err := a.agentScheduler.ScheduleCustomerAuto(ctx, tx, channel.OrganizationID, received.Message.ConversationID, received.Session.ID, received.Message.ID); err != nil {
						return fmt.Errorf("schedule Telegram customer agent: %w", err)
					}
				}
				// 新入站消息在事务内投递按渠道身份去重的头像同步任务，连续消息只同步一次。
				if received.Inserted {
					if _, err := a.tasks.EnqueueIn(ctx, tx, channelaction.RefreshTelegramContactAvatarActionName, channelaction.RefreshTelegramContactAvatarInput{
						OrganizationID: channel.OrganizationID, ChannelID: channelID, ChannelIdentityID: received.ChannelIdentityID, SenderID: input.Message.SenderID,
					}, servertask.EnqueueOptions{
						MaxAttempts: 1, IdempotencyKey: "tgavatar:" + received.ChannelIdentityID, TriggerType: servertask.TriggerBusiness,
					}); err != nil {
						return fmt.Errorf("enqueue Telegram contact avatar refresh: %w", err)
					}
				}
			}
		}
		result, err := tx.NewUpdate().
			Model(setting).
			Set("webhook_status = ?", domain.TelegramWebhookStatusNormal).
			Set("webhook_connected_at = now()").
			Set("updated_at = now()").
			Where("organization_id = ?", setting.OrganizationID).
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
	attributes := []any{"channel_id", channelID, "update_id", input.UpdateID}
	if input.Message != nil {
		attributes = append(attributes, "chat_id", input.Message.ChatID, "message_id", input.Message.MessageID)
	}
	if ignoredConflict {
		slog.Warn("Telegram 消息幂等冲突已忽略", attributes...)
		return nil
	}
	slog.Info("Telegram Webhook 回调已接收", attributes...)
	return nil
}

// loadActiveTelegramWebhookSetting 读取启用渠道当前可接收回调的设置。
//
// 所属企业由本次查询结果确定，调用方随后按 setting.OrganizationID 限定企业。
func loadActiveTelegramWebhookSetting(ctx context.Context, db bun.IDB, channelID string, lock bool) (*servermodels.TelegramChannelSetting, error) {
	setting := &servermodels.TelegramChannelSetting{}
	query := db.NewSelect().
		Model(setting).
		Join("JOIN channels AS c ON c.id = tcs.channel_id AND c.organization_id = tcs.organization_id").
		Where("tcs.channel_id = ?", channelID).
		Where("c.type = ?", domain.ChannelTypeTelegram).
		Where("c.enabled = TRUE").
		Where("tcs.webhook_secret IS NOT NULL").
		Where("tcs.bot_id IS NOT NULL")
	if lock {
		query = query.For("UPDATE OF tcs")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, channelaction.ErrNotFound
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
