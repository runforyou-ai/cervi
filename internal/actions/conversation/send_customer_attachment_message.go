//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"
	"uuid"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// SendCustomerAttachmentMessageAction 持久化企业成员发往客户会话的附件消息。
type SendCustomerAttachmentMessageAction struct {
	enqueuer servertask.TxEnqueuer
	db       *bun.DB
}

// NewSendCustomerAttachmentMessageAction 创建成员客户会话附件发送操作。
func NewSendCustomerAttachmentMessageAction(db *bun.DB, enqueuer servertask.TxEnqueuer) *SendCustomerAttachmentMessageAction {
	return &SendCustomerAttachmentMessageAction{db: db, enqueuer: enqueuer}
}

// Execute 在一个可重试事务中写入成员客户会话附件回复并激活文件。
func (a *SendCustomerAttachmentMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input CustomerAttachmentMessageInput) (ConversationMessage, error) {
	normalized, fields := normalizeCustomerAttachmentMessageInput(input)
	if len(fields) > 0 {
		return ConversationMessage{}, &ValidationError{Fields: fields}
	}
	// 预生成一次事务重试期间稳定使用的 UUIDv7。
	values := make([]string, 3)
	for index := range values {
		values[index] = uuid.NewV7().String()
	}
	ids := memberMessageIDs{subject: values[0], participant: values[1], message: values[2]}
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + normalized.ClientMessageID
	payload := customerMessagePayload{
		ConversationID: normalized.ConversationID, ClientMessageID: normalized.ClientMessageID,
		Body: normalized.Body, ReplyToMessageID: normalized.ReplyToMessageID, Type: domain.MessageTypeAttachment,
		// 附件只用于对客回复，内部备注附件不在本次范围内。
		Visibility: domain.MessageVisibilityCustomerVisible,
		Attachment: &customerAttachmentPayload{FileID: normalized.FileID, ImageWidth: normalized.ImageWidth, ImageHeight: normalized.ImageHeight},
	}
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		var result ConversationMessage
		err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
			var executeErr error
			result, executeErr = sendCustomerMessage(ctx, tx, identity, a.enqueuer, payload, ids, idempotencyKey)
			return executeErr
		})
		if err == nil {
			// 发送结果与历史查询使用同一引用能力判定，未取得平台回执时不可被引用。
			var row conversationMessageRow
			if err := conversationMessagesQuery(a.db, identity, normalized.ConversationID).Where("msg.id = ?", result.ID).Scan(ctx, &row); err != nil {
				return ConversationMessage{}, fmt.Errorf("load sent attachment reference state: %w", err)
			}
			result.ReplyUnavailable = row.ReplyUnavailable
			return result, nil
		}
		constraint, retryable := retryableUniqueViolation(err, memberMessageRetryableConstraintNames)
		if !retryable {
			return ConversationMessage{}, err
		}
		if attempt < maxWriteAttempts-1 {
			slog.Info("成员客户附件写入重试", "conversation_id", normalized.ConversationID, "attempt", attempt+2, "constraint", constraint)
		}
	}
	slog.Warn("成员客户附件写入重试耗尽", "conversation_id", normalized.ConversationID, "error", err)
	return ConversationMessage{}, fmt.Errorf("send customer attachment retries exhausted: %w", err)
}

// normalizeCustomerAttachmentMessageInput 规范化并校验成员客户会话附件输入。
func normalizeCustomerAttachmentMessageInput(input CustomerAttachmentMessageInput) (CustomerAttachmentMessageInput, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	input.Body = strings.TrimSpace(input.Body)
	var valid bool
	input.ConversationID, valid = common.NormalizeUUID(input.ConversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	input.ClientMessageID, valid = common.NormalizeUUID(input.ClientMessageID)
	if !valid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	input.FileID, valid = common.NormalizeUUID(input.FileID)
	if !valid {
		fields["fileId"] = ValidationFileIDInvalid
	}
	if input.ReplyToMessageID != "" {
		input.ReplyToMessageID, valid = common.NormalizeUUID(input.ReplyToMessageID)
		if !valid {
			fields["replyToMessageId"] = ValidationReplyToMessageIDInvalid
		}
	}
	if utf8.RuneCountInString(input.Body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	if input.ImageWidth < 0 || input.ImageHeight < 0 {
		fields["fileId"] = ValidationFileIDInvalid
	}
	return input, fields
}

// lockCustomerAttachmentFile 锁定本人上传的有效文件并按渠道上限校验字节数。
func lockCustomerAttachmentFile(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, channelType domain.ChannelType, payload customerAttachmentPayload) (*MessageAttachment, error) {
	file := &servermodels.File{}
	err := tx.NewSelect().Model(file).ColumnExpr("f.*").ColumnExpr("f.expires_at <= now() AS expired").
		Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ?", payload.FileID, identity.Organization.ID, identity.User.ID).
		Where("f.purpose = ? AND f.uploader_channel_identity_id IS NULL", domain.FilePurposeMessageAttachment).
		For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fileaction.ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	if file.Status != string(domain.FileStatusUploaded) || file.Expired {
		return nil, fileaction.ErrFileNotFound
	}
	if limit := domain.ChannelAttachmentLimit(channelType, file.ContentType); limit > 0 && file.ByteSize > limit {
		return nil, &ConflictError{Reason: ConflictReasonAttachmentTooLarge}
	}
	if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusActive).Set("expires_at = NULL").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
		return nil, fmt.Errorf("activate customer attachment file: %w", err)
	}
	return &MessageAttachment{
		ID: file.ID, Name: file.OriginalName, ContentType: file.ContentType, ByteSize: file.ByteSize,
		ImageWidth: payload.ImageWidth, ImageHeight: payload.ImageHeight, TransferStatus: domain.MessageAttachmentTransferReady,
	}, nil
}

// saveCustomerAttachment 写入客户会话消息的附件关联。
func saveCustomerAttachment(ctx context.Context, tx bun.IDB, organizationID, messageID string, attachment MessageAttachment) error {
	if _, err := tx.NewRaw(`INSERT INTO message_attachments
 (message_id, organization_id, file_id, name, content_type, byte_size, image_width, image_height, transfer_status)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		messageID, organizationID, common.OptionalString(attachment.ID), attachment.Name, attachment.ContentType, attachment.ByteSize,
		attachment.ImageWidth, attachment.ImageHeight, attachment.TransferStatus).Exec(ctx); err != nil {
		return fmt.Errorf("save customer message attachment: %w", err)
	}
	return nil
}
