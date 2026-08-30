//go:build server

package channel

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/telegram"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
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

// TelegramWebhookMessage 定义已归一化的 Telegram 私聊文本消息。
type TelegramWebhookMessage struct {
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
	db          *bun.DB
	avatarAPI   telegram.ProfilePhotoAPI
	avatarFiles telegramContactAvatarImporter
}

// NewReceiveTelegramWebhookAction 创建 Telegram Webhook 接收操作。
func NewReceiveTelegramWebhookAction(db *bun.DB, avatarAPI telegram.ProfilePhotoAPI, avatarFiles telegramContactAvatarImporter) *ReceiveTelegramWebhookAction {
	return &ReceiveTelegramWebhookAction{db: db, avatarAPI: avatarAPI, avatarFiles: avatarFiles}
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
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
			received, err := conversationaction.ReceiveInboundCustomerTextMessage(ctx, tx, channel, conversationaction.InboundCustomerTextMessageInput{
				ExternalID: strconv.FormatInt(input.Message.SenderID, 10), DisplayName: &displayName,
				SingleConversation: true, Body: input.Message.Body,
				IdempotencyKey: "chmsg:" + channelID + ":tg:" + strconv.FormatInt(input.Message.ChatID, 10) + ":" + strconv.FormatInt(input.Message.MessageID, 10),
				OriginatedAt:   input.Message.OriginatedAt, SourceOrder: input.Message.MessageID,
			})
			if err != nil {
				var conflict *conversationaction.ConflictError
				if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonIdempotencyMismatch {
					return err
				}
				ignoredConflict = true
			} else {
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
			avatarIdentityID, avatarBotToken, input.Message.SenderID, input.Message.MessageID,
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
	senderID, sourceOrder int64,
) error {
	current := &servermodels.ContactChannelIdentity{}
	if err := a.db.NewSelect().Model(current).
		Column("avatar_source_order").
		Where("cci.id = ?", identityID).
		Where("cci.organization_id = ?", organizationID).
		Where("cci.channel_id = ?", channelID).
		Scan(ctx); err != nil {
		return fmt.Errorf("load Telegram contact avatar cursor: %w", err)
	}
	if current.AvatarSourceOrder > sourceOrder {
		return nil
	}
	photo, err := a.avatarAPI.GetUserProfilePhoto(ctx, token, senderID)
	if err != nil {
		return err
	}
	if photo == nil {
		return a.applyTelegramContactAvatar(ctx, channelID, organizationID, identityID, sourceOrder, nil, nil)
	}
	digest := sha256.Sum256([]byte(photo.UniqueID))
	version := hex.EncodeToString(digest[:])
	unchanged, err := a.advanceTelegramContactAvatarCursor(ctx, channelID, organizationID, identityID, version, sourceOrder)
	if err != nil || unchanged {
		return err
	}
	if a.avatarFiles == nil {
		return errors.New("Telegram contact avatar importer is unavailable")
	}
	downloaded, err := a.avatarAPI.DownloadPhoto(ctx, token, photo.FileID)
	if err != nil {
		return err
	}
	imported, err := a.avatarFiles.Execute(ctx, fileaction.ImportInput{
		OrganizationID: organizationID, CreatedByUserID: createdByUserID,
		FileName:    telegramAvatarFileName(downloaded.ContentType),
		ContentType: downloaded.ContentType, Data: downloaded.Data,
	})
	if err != nil {
		return err
	}
	return a.applyTelegramContactAvatar(ctx, channelID, organizationID, identityID, sourceOrder, &version, imported)
}

// advanceTelegramContactAvatarCursor 在头像版本未变化时只推进来源顺序。
func (a *ReceiveTelegramWebhookAction) advanceTelegramContactAvatarCursor(ctx context.Context, channelID, organizationID, identityID, version string, sourceOrder int64) (bool, error) {
	result, err := a.db.NewUpdate().Model((*servermodels.ContactChannelIdentity)(nil)).
		Set("avatar_source_order = ?", sourceOrder).
		Set("updated_at = now()").
		Where("id = ?", identityID).
		Where("organization_id = ?", organizationID).
		Where("channel_id = ?", channelID).
		Where("avatar_file_id IS NOT NULL").
		Where("avatar_external_version = ?", version).
		Where("avatar_source_order <= ?", sourceOrder).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("advance Telegram contact avatar cursor: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read Telegram contact avatar cursor count: %w", err)
	}
	return rows > 0, nil
}

// applyTelegramContactAvatar 原子切换头像文件引用并回收旧文件。
func (a *ReceiveTelegramWebhookAction) applyTelegramContactAvatar(
	ctx context.Context,
	channelID, organizationID, identityID string,
	sourceOrder int64,
	version *string,
	imported *servermodels.File,
) error {
	return a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		current := &servermodels.ContactChannelIdentity{}
		if err := tx.NewSelect().Model(current).
			Column("id", "organization_id", "channel_id", "avatar_file_id", "avatar_external_version", "avatar_source_order").
			Where("cci.id = ?", identityID).
			Where("cci.organization_id = ?", organizationID).
			Where("cci.channel_id = ?", channelID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return fmt.Errorf("lock Telegram contact avatar: %w", err)
		}
		if current.AvatarSourceOrder > sourceOrder {
			return nil
		}
		if version != nil && current.AvatarFileID != nil && current.AvatarExternalVersion != nil && *current.AvatarExternalVersion == *version {
			_, err := tx.NewUpdate().Model((*servermodels.ContactChannelIdentity)(nil)).
				Set("avatar_source_order = ?", sourceOrder).
				Set("updated_at = now()").
				Where("id = ?", identityID).
				Where("organization_id = ?", organizationID).
				Where("channel_id = ?", channelID).
				Exec(ctx)
			return err
		}

		var nextFileID any
		var nextVersion any
		if imported != nil {
			if version == nil {
				return errors.New("Telegram contact avatar version is required")
			}
			result, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
				Set("status = ?", domain.FileStatusActive).
				Set("expires_at = NULL").
				Set("updated_at = now()").
				Where("id = ?", imported.ID).
				Where("organization_id = ?", organizationID).
				Where("purpose = ?", domain.FilePurposeContactAvatar).
				Where("status = ?", domain.FileStatusUploaded).
				Where("expires_at > now()").
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
			nextFileID = imported.ID
			nextVersion = *version
		}

		previousFileID := current.AvatarFileID
		result, err := tx.NewUpdate().Model((*servermodels.ContactChannelIdentity)(nil)).
			Set("avatar_file_id = ?", nextFileID).
			Set("avatar_external_version = ?", nextVersion).
			Set("avatar_source_order = ?", sourceOrder).
			Set("updated_at = now()").
			Where("id = ?", identityID).
			Where("organization_id = ?", organizationID).
			Where("channel_id = ?", channelID).
			Where("avatar_source_order <= ?", sourceOrder).
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
		if previousFileID != nil && (imported == nil || *previousFileID != imported.ID) {
			if _, err := tx.NewUpdate().Model((*servermodels.File)(nil)).
				Set("status = ?", domain.FileStatusDeleting).
				Set("expires_at = now()").
				Set("updated_at = now()").
				Where("id = ?", *previousFileID).
				Where("organization_id = ?", organizationID).
				Where("purpose = ?", domain.FilePurposeContactAvatar).
				Where("status = ?", domain.FileStatusActive).
				Exec(ctx); err != nil {
				return fmt.Errorf("retire previous Telegram contact avatar: %w", err)
			}
		}
		return nil
	})
}

// telegramAvatarFileName 返回已校验头像内容的固定文件名。
func telegramAvatarFileName(contentType string) string {
	switch contentType {
	case "image/png":
		return "telegram-avatar.png"
	case "image/webp":
		return "telegram-avatar.webp"
	default:
		return "telegram-avatar.jpg"
	}
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
