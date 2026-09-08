//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// SendAttachmentMessageAction 保存内部会话附件并激活文件。
type SendAttachmentMessageAction struct{ db *bun.DB }

// NewSendAttachmentMessageAction 创建附件消息发送操作。
func NewSendAttachmentMessageAction(db *bun.DB) *SendAttachmentMessageAction {
	return &SendAttachmentMessageAction{db: db}
}

// Execute 在成员和会话锁内幂等发送附件，首发时按需创建单聊。
func (a *SendAttachmentMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input AttachmentMessageInput) (AttachmentMessageResult, error) {
	if !common.ValidUUID(input.ClientMessageID) || !common.ValidUUID(input.FileID) ||
		(input.ConversationID == "") == (input.TargetIdentityID == "") ||
		(input.ConversationID != "" && !common.ValidUUID(input.ConversationID)) ||
		(input.TargetIdentityID != "" && !common.ValidUUID(input.TargetIdentityID)) {
		return AttachmentMessageResult{}, ErrConversationNotFound
	}
	var result AttachmentMessageResult
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			conversationID := input.ConversationID
			var target directTargetRow
			if input.TargetIdentityID != "" {
				if input.TargetIdentityID == identity.OrganizationIdentity.ID {
					return ErrDirectTargetNotFound
				}
				var err error
				target, err = loadDirectTarget(ctx, tx, identity.Organization.ID, input.TargetIdentityID)
				if err != nil {
					return err
				}
				conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
				if err != nil {
					return err
				}
				if conversation == nil {
					conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
					if err != nil {
						return err
					}
				}
				conversationID = conversation.ID
			}
			member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
			if err != nil {
				return err
			}
			switch domain.ConversationType(member.Conversation.Type) {
			case domain.ConversationTypeDirect:
				if input.TargetIdentityID != "" && member.Conversation.Status == string(domain.ConversationStatusArchived) {
					if _, err := tx.NewUpdate().Model(member.Conversation).Set("status = ?", domain.ConversationStatusActive).Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
						return err
					}
				}
				if _, err := loadDirectSendContext(ctx, tx, identity, conversationID); err != nil {
					return err
				}
			case domain.ConversationTypeGroup:
				if member.Conversation.Status != string(domain.ConversationStatusActive) {
					return ErrConversationNotFound
				}
			default:
				return ErrConversationNotFound
			}
			message, err := saveAttachmentMessage(ctx, tx, identity, member, input)
			if err != nil {
				return err
			}
			result = AttachmentMessageResult{ConversationID: conversationID, Message: message}
			if input.TargetIdentityID != "" {
				summary, err := loadDirectConversationSummary(ctx, tx, identity.Organization.ID, conversationID, target)
				if err != nil {
					return err
				}
				result.Conversation = &summary
			}
			return nil
		})
		if err == nil {
			return result, nil
		}
		if _, retryable := retryableUniqueViolation(err, map[string]struct{}{
			"direct_conversations_organization_identity_pair_unique": {},
			"messages_organization_idempotency_unique":               {},
		}); !retryable {
			return AttachmentMessageResult{}, err
		}
	}
	return AttachmentMessageResult{}, err
}

// saveAttachmentMessage 校验完整发送意图并在同一事务内保存消息、附件和阅读位置。
func saveAttachmentMessage(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, member chatstate.Member, input AttachmentMessageInput) (ConversationMessage, error) {
	key := "mmsg:" + identity.OrganizationIdentity.ID + ":" + input.ClientMessageID
	existing := &servermodels.Message{}
	err := tx.NewSelect().Model(existing).Where("msg.organization_id = ? AND msg.idempotency_key = ?", identity.Organization.ID, key).Scan(ctx)
	if err == nil {
		var fileID string
		if err := tx.NewSelect().Table("message_attachments").Column("file_id").Where("organization_id = ? AND message_id = ?", identity.Organization.ID, existing.ID).Scan(ctx, &fileID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ConversationMessage{}, err
		}
		if existing.Type != string(domain.MessageTypeAttachment) || existing.ConversationID != member.Conversation.ID ||
			existing.DeletedAt != nil || existing.SenderParticipantID == nil || *existing.SenderParticipantID != member.ParticipantID || fileID != input.FileID {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		messages := []ConversationMessage{memberConversationMessage(existing, member.SubjectID, identity.OrganizationIdentity)}
		err := loadMessageAttachments(ctx, tx, identity.Organization.ID, messages)
		return messages[0], err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, err
	}
	file := &servermodels.File{}
	err = tx.NewSelect().Model(file).ColumnExpr("f.*").ColumnExpr("f.expires_at <= now() AS expired").
		Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ?", input.FileID, identity.Organization.ID, identity.User.ID).
		Where("f.purpose = ?", domain.FilePurposeMessageAttachment).For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, fileaction.ErrFileNotFound
	}
	if err != nil {
		return ConversationMessage{}, err
	}
	if (file.Status != string(domain.FileStatusUploaded) && !(input.Pending && file.Status == string(domain.FileStatusPending))) || file.Expired {
		return ConversationMessage{}, fileaction.ErrFileNotFound
	}
	message := &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: member.Conversation.ID,
		SenderParticipantID: &member.ParticipantID, Type: string(domain.MessageTypeAttachment),
		IdempotencyKey: &key, OriginatedAt: time.Now().UTC(),
	}
	if member.Conversation.Type == string(domain.ConversationTypeGroup) {
		sequence, err := nextGroupMessageSequence(ctx, tx, identity.Organization.ID, member.Conversation.ID)
		if err != nil {
			return ConversationMessage{}, err
		}
		message.GroupMessageSequence = &sequence
	}
	if _, err := tx.NewInsert().Model(message).Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at", "group_message_sequence").Returning("*").Exec(ctx); err != nil {
		return ConversationMessage{}, err
	}
	status := domain.AttachmentReady
	if input.Pending {
		status = domain.AttachmentUploading
	}
	if _, err := tx.NewRaw(`INSERT INTO message_attachments
 (message_id, organization_id, file_id, name, content_type, byte_size, image_width, image_height, upload_status, upload_expires_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? THEN now() + interval '2 minutes' END)`,
		message.ID, identity.Organization.ID, file.ID, file.OriginalName, file.ContentType, file.ByteSize, input.ImageWidth, input.ImageHeight, status, input.Pending).Exec(ctx); err != nil {
		return ConversationMessage{}, err
	}
	if !input.Pending {
		if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusActive).Set("expires_at = NULL").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return ConversationMessage{}, err
		}
	}
	if err := updateConversationSummary(ctx, tx, member.Conversation, message); err != nil {
		return ConversationMessage{}, err
	}
	if err := advanceConversationUserReadState(ctx, tx, &servermodels.ConversationUserState{
		OrganizationID: identity.Organization.ID, ConversationID: member.Conversation.ID, UserID: identity.User.ID, LastReadMessageID: &message.ID,
	}, message); err != nil {
		return ConversationMessage{}, err
	}
	result := memberConversationMessage(message, member.SubjectID, identity.OrganizationIdentity)
	result.Attachment = &MessageAttachment{ID: file.ID, Name: file.OriginalName, ContentType: file.ContentType, ByteSize: file.ByteSize, UploadStatus: status, ImageWidth: input.ImageWidth, ImageHeight: input.ImageHeight}
	return result, nil
}

// loadMessageAttachments 批量读取当前消息窗口中的附件元数据。
func loadMessageAttachments(ctx context.Context, db bun.IDB, organizationID string, messages []ConversationMessage) error {
	ids := make([]string, 0)
	for _, message := range messages {
		if message.Type == domain.MessageTypeAttachment {
			ids = append(ids, message.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows := []struct {
		MessageID string `bun:"message_id"`
		MessageAttachment
	}{}
	if err := db.NewSelect().TableExpr("message_attachments AS ma").
		ColumnExpr("ma.message_id, COALESCE(ma.file_id::text, '') AS id, ma.name, ma.content_type, ma.byte_size, ma.image_width, ma.image_height").
		ColumnExpr("CASE WHEN ma.upload_status = ? AND ma.upload_expires_at <= now() THEN ? ELSE ma.upload_status END AS upload_status", domain.AttachmentUploading, domain.AttachmentFailed).
		Where("ma.organization_id = ? AND ma.message_id IN (?)", organizationID, bun.In(ids)).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("load message attachments: %w", err)
	}
	byMessage := make(map[string]MessageAttachment, len(rows))
	for _, row := range rows {
		byMessage[row.MessageID] = row.MessageAttachment
	}
	for index := range messages {
		if messages[index].Type != domain.MessageTypeAttachment {
			continue
		}
		attachment, found := byMessage[messages[index].ID]
		if !found {
			return ErrDataInvariant
		}
		messages[index].Attachment = &attachment
	}
	return nil
}

// GetAttachmentFile 读取当前成员可见消息所关联的有效文件。
func (q *ListConversationMessagesQuery) GetAttachmentFile(ctx context.Context, identity *servermodels.Identity, conversationID, messageID string) (*servermodels.File, error) {
	if !common.ValidUUID(conversationID) || !common.ValidUUID(messageID) {
		return nil, ErrConversationNotFound
	}
	if err := authorizeConversationHistory(ctx, q.db, identity, conversationID); err != nil {
		return nil, err
	}
	record := &servermodels.File{}
	err := q.db.NewSelect().Model(record).
		Join("JOIN message_attachments AS ma ON ma.organization_id = f.organization_id AND ma.file_id = f.id").
		Join("JOIN messages AS msg ON msg.organization_id = ma.organization_id AND msg.id = ma.message_id").
		Where("msg.organization_id = ? AND msg.conversation_id = ? AND msg.id = ? AND msg.deleted_at IS NULL", identity.Organization.ID, conversationID, messageID).
		Where("f.status = ?", domain.FileStatusActive).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fileaction.ErrFileNotFound
	}
	return record, err
}

// AttachmentStates 读取已有附件消息的状态，不依赖新增消息游标。
func (q *ListConversationMessagesQuery) AttachmentStates(ctx context.Context, identity *servermodels.Identity, conversationID string, ids []string) ([]AttachmentMessageState, error) {
	if !common.ValidUUID(conversationID) {
		return nil, ErrConversationNotFound
	}
	for _, id := range ids {
		if !common.ValidUUID(id) {
			return nil, ErrConversationNotFound
		}
	}
	if err := authorizeConversationHistory(ctx, q.db, identity, conversationID); err != nil {
		return nil, err
	}
	rows := []struct {
		ID      string
		Deleted bool
	}{}
	if err := q.db.NewSelect().TableExpr("messages msg").ColumnExpr("msg.id, msg.deleted_at IS NOT NULL AS deleted").
		Where("msg.organization_id = ? AND msg.conversation_id = ? AND msg.id IN (?) AND msg.type = ?", identity.Organization.ID, conversationID, bun.In(ids), domain.MessageTypeAttachment).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	messages := make([]ConversationMessage, len(rows))
	for index, row := range rows {
		messages[index] = ConversationMessage{ID: row.ID, Type: domain.MessageTypeAttachment}
	}
	if err := loadMessageAttachments(ctx, q.db, identity.Organization.ID, messages); err != nil {
		return nil, err
	}
	states := make([]AttachmentMessageState, 0, len(rows))
	for index, row := range rows {
		states = append(states, AttachmentMessageState{MessageID: row.ID, Attachment: *messages[index].Attachment, Deleted: row.Deleted})
	}
	return states, nil
}
