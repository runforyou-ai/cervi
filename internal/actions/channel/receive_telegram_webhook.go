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
	"strconv"
	"time"

	"github.com/runforyou-ai/cervi/internal/actions/channelmessage"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
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

const telegramAvatarRefreshTimeout = 15 * time.Second

// TelegramWebhookInput 定义公开回调完成认证和状态更新所需字段。
type TelegramWebhookInput struct {
	Secret       string
	UpdateID     int64
	MyChatMember bool
	Message      *TelegramWebhookMessage
}

// TelegramWebhookReply 保存 Telegram 引用消息的编号与一层快照。
type TelegramWebhookReply struct {
	MessageID   int64
	Body        string
	SenderName  string
	SenderIsBot bool
}

// TelegramWebhookMedia 定义随 Telegram 消息送达、内容待取回的单个媒体文件。
type TelegramWebhookMedia struct {
	FileID      string
	UniqueID    string
	FileName    string
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
}

// TelegramWebhookMessage 定义已归一化的 Telegram 私聊消息，Media 非空时 Body 为媒体说明。
type TelegramWebhookMessage struct {
	Reply        *TelegramWebhookReply
	Media        *TelegramWebhookMedia
	ChatID       int64
	MessageID    int64
	SenderID     int64
	DisplayName  string
	Body         string
	OriginatedAt time.Time
}

// telegramContactAvatarImporter 把 Telegram 头像写为可激活的企业文件。
type telegramContactAvatarImporter interface {
	Execute(context.Context, fileaction.ImportInput) (*servermodels.File, error)
}

// ReceiveTelegramWebhookAction 认证 Telegram 回调并更新连接状态。
type ReceiveTelegramWebhookAction struct {
	db             *bun.DB
	agentScheduler conversationaction.CustomerAgentMessageScheduler
	avatarAPI      telegram.ProfilePhotoAPI
	avatarFiles    telegramContactAvatarImporter
	mediaBackend   fileaction.StorageBackendResolver
	mediaTasks     servertask.TxEnqueuer
}

// NewReceiveTelegramWebhookAction 创建 Telegram Webhook 接收操作，mediaBackend 决定入站媒体的存储类型，mediaTasks 在入站事务内投递取回任务。
func NewReceiveTelegramWebhookAction(db *bun.DB, agentScheduler conversationaction.CustomerAgentMessageScheduler, avatarAPI telegram.ProfilePhotoAPI, avatarFiles telegramContactAvatarImporter, mediaBackend fileaction.StorageBackendResolver, mediaTasks servertask.TxEnqueuer) *ReceiveTelegramWebhookAction {
	return &ReceiveTelegramWebhookAction{db: db, agentScheduler: agentScheduler, avatarAPI: avatarAPI, avatarFiles: avatarFiles, mediaBackend: mediaBackend, mediaTasks: mediaTasks}
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
	var ignoredConflict bool
	var avatarIdentityID string
	var avatarBotToken string
	var avatarOrganizationID string
	var avatarCreatedByUserID string
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
			inbound := conversationaction.InboundCustomerMessageInput{
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
				inbound.ExternalMedia = &conversationaction.InboundExternalMedia{
					ExternalID: media.UniqueID, FileName: media.FileName, ContentType: media.ContentType, ByteSize: media.ByteSize,
					ImageWidth: media.Width, ImageHeight: media.Height, StorageBackend: backend,
				}
			}
			received, err := conversationaction.ReceiveInboundCustomerMessage(ctx, tx, a.mediaTasks, channel, inbound)
			if err != nil {
				var conflict *conversationaction.ConflictError
				if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
					return err
				}
				ignoredConflict = true
			} else {
				// 仅新入站消息触发 AI 客服，回调重放不追加运行输入；取回中的媒体由取回任务在终态时调度。
				if received.Inserted && received.Attachment != nil && received.Attachment.TransferStatus == domain.MessageAttachmentTransferPending {
					if _, err := a.mediaTasks.EnqueueIn(ctx, tx, RetrieveTelegramMediaActionName, RetrieveTelegramMediaInput{
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
				avatarIdentityID = received.ChannelIdentityID
				avatarOrganizationID = channel.OrganizationID
				avatarCreatedByUserID = channel.CreatedByUserID
				if setting.BotToken != nil {
					avatarBotToken = *setting.BotToken
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
	if input.Message != nil && a.avatarAPI != nil && avatarIdentityID != "" && avatarBotToken != "" {
		refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telegramAvatarRefreshTimeout)
		refreshErr := a.refreshTelegramContactAvatar(
			refreshCtx, channelID, avatarOrganizationID, avatarCreatedByUserID,
			avatarIdentityID, avatarBotToken, input.Message.SenderID,
		)
		cancel()
		if refreshErr != nil {
			logTelegramRemoteFailure("同步 Telegram 用户头像失败", channelID, refreshErr)
		}
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

// refreshTelegramContactAvatar 在消息事务提交后持久化渠道身份头像。
func (a *ReceiveTelegramWebhookAction) refreshTelegramContactAvatar(
	ctx context.Context,
	channelID, organizationID, createdByUserID, identityID, token string,
	senderID int64,
) error {
	photo, err := a.avatarAPI.GetUserProfilePhoto(ctx, token, senderID)
	if err != nil {
		return err
	}
	if photo == nil {
		return a.applyTelegramContactAvatar(ctx, channelID, organizationID, identityID, nil)
	}
	existing, err := a.findTelegramContactAvatarFile(ctx, organizationID, photo.UniqueID)
	if err != nil {
		return err
	}
	if existing != nil {
		return a.applyTelegramContactAvatar(ctx, channelID, organizationID, identityID, existing)
	}
	if a.avatarFiles == nil {
		return errors.New("Telegram contact avatar importer is unavailable")
	}
	downloaded, err := a.avatarAPI.DownloadPhoto(ctx, token, photo.FileID)
	if err != nil {
		return err
	}
	// 返回已校验头像内容的固定文件名。
	fileName := "telegram-avatar.jpg"
	switch downloaded.ContentType {
	case "image/png":
		fileName = "telegram-avatar.png"
	case "image/webp":
		fileName = "telegram-avatar.webp"
	}
	imported, err := a.avatarFiles.Execute(ctx, fileaction.ImportInput{
		OrganizationID: organizationID, CreatedByUserID: createdByUserID,
		ExternalID:  photo.UniqueID,
		FileName:    fileName,
		ContentType: downloaded.ContentType, Data: downloaded.Data,
	})
	if err != nil {
		return err
	}
	return a.applyTelegramContactAvatar(ctx, channelID, organizationID, identityID, imported)
}

// findTelegramContactAvatarFile 按 Telegram 文件唯一标识复用已写入的企业文件。
func (a *ReceiveTelegramWebhookAction) findTelegramContactAvatarFile(ctx context.Context, organizationID, externalID string) (*servermodels.File, error) {
	record := &servermodels.File{}
	err := a.db.NewSelect().Model(record).
		Where("f.organization_id = ?", organizationID).
		Where("f.purpose = ?", domain.FilePurposeContactAvatar).
		Where("f.external_id = ?", externalID).
		Where("(f.status = ? OR (f.status = ? AND f.expires_at > now()))", domain.FileStatusActive, domain.FileStatusUploaded).
		OrderExpr("CASE WHEN f.status = ? THEN 0 ELSE 1 END", domain.FileStatusActive).
		OrderExpr("f.created_at DESC, f.id DESC").
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find Telegram contact avatar file: %w", err)
	}
	return record, nil
}

// applyTelegramContactAvatar 原子切换头像文件引用、回收旧文件，并推进该渠道身份所在客户会话的版本。
func (a *ReceiveTelegramWebhookAction) applyTelegramContactAvatar(
	ctx context.Context,
	channelID, organizationID, identityID string,
	next *servermodels.File,
) error {
	return realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		current := &servermodels.ContactChannelIdentity{}
		if err := tx.NewSelect().Model(current).
			Column("id", "organization_id", "channel_id", "avatar_file_id").
			Where("cci.id = ?", identityID).
			Where("cci.organization_id = ?", organizationID).
			Where("cci.channel_id = ?", channelID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return fmt.Errorf("lock Telegram contact avatar: %w", err)
		}
		if next == nil && current.AvatarFileID == nil {
			return nil
		}
		if next != nil && current.AvatarFileID != nil && *current.AvatarFileID == next.ID {
			return nil
		}

		// 按编号顺序锁定新旧头像文件。
		fileIDs := make([]string, 0, 2)
		if next != nil {
			fileIDs = append(fileIDs, next.ID)
		}
		if current.AvatarFileID != nil {
			fileIDs = append(fileIDs, *current.AvatarFileID)
		}
		var files []servermodels.File
		if err := tx.NewSelect().Model(&files).Column("id").
			Where("f.organization_id = ? AND f.id IN (?)", organizationID, bun.In(fileIDs)).
			OrderExpr("f.id").For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock Telegram contact avatar files: %w", err)
		}

		var nextFileID any
		if next != nil {
			result, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
				Set("status = ?", domain.FileStatusActive).
				Set("expires_at = NULL").
				Set("updated_at = now()").
				Where("id = ?", next.ID).
				Where("organization_id = ?", organizationID).
				Where("purpose = ?", domain.FilePurposeContactAvatar).
				Where("(status = ? OR (status = ? AND expires_at > now()))", domain.FileStatusActive, domain.FileStatusUploaded).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("activate Telegram contact avatar file: %w", err)
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("read Telegram contact avatar activation count: %w", err)
			}
			if rows == 0 {
				return fileaction.ErrFileNotFound
			}
			nextFileID = next.ID
		}

		previousFileID := current.AvatarFileID
		result, err := tx.NewUpdate().Model((*servermodels.ContactChannelIdentity)(nil)).
			Set("avatar_file_id = ?", nextFileID).
			Set("updated_at = now()").
			Where("id = ?", identityID).
			Where("organization_id = ?", organizationID).
			Where("channel_id = ?", channelID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update Telegram contact avatar: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read Telegram contact avatar update count: %w", err)
		}
		if rows == 0 {
			return errors.New("Telegram contact avatar was not updated")
		}
		if previousFileID != nil && (next == nil || *previousFileID != next.ID) {
			if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
				Set("status = ?", domain.FileStatusDeleting).
				Set("expires_at = now()").
				Set("updated_at = now()").
				Where("id = ?", *previousFileID).
				Where("organization_id = ?", organizationID).
				Where("purpose = ?", domain.FilePurposeContactAvatar).
				Where("status = ?", domain.FileStatusActive).
				Where("NOT EXISTS (SELECT 1 FROM contact_channel_identities AS other WHERE other.organization_id = f.organization_id AND other.avatar_file_id = f.id)").
				Exec(ctx); err != nil {
				return fmt.Errorf("retire previous Telegram contact avatar: %w", err)
			}
		}
		return chatstate.TouchChannelIdentityConversations(ctx, tx, organizationID, identityID)
	})
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
