//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ExecuteBatch 在同一事务中按顺序创建文件和消息，重试复用每条消息的发送编号。
func (a *SendAttachmentMessageAction) ExecuteBatch(ctx context.Context, identity *servermodels.Identity, input AttachmentBatchInput, backend domain.FileStorageBackend) (AttachmentBatchResult, error) {
	if len(input.Attachments) == 0 || len(input.Attachments) > 100 ||
		(input.ConversationID == "") == (input.TargetIdentityID == "") ||
		(input.ConversationID != "" && !common.ValidUUID(input.ConversationID)) || (input.TargetIdentityID != "" && !common.ValidUUID(input.TargetIdentityID)) {
		return AttachmentBatchResult{}, ErrConversationNotFound
	}
	seen := map[string]bool{}
	for index, item := range input.Attachments {
		item.Body = strings.TrimSpace(item.Body)
		if !common.ValidUUID(item.ClientMessageID) || seen[item.ClientMessageID] || item.ImageWidth < 0 || item.ImageHeight < 0 || utf8.RuneCountInString(item.Body) > 4000 {
			return AttachmentBatchResult{}, ErrConversationNotFound
		}
		seen[item.ClientMessageID] = true
		item.File.Purpose = domain.FilePurposeMessageAttachment
		normalized, fields := fileaction.NormalizeUploadInput(item.File)
		if len(fields) > 0 {
			return AttachmentBatchResult{}, &fileaction.ValidationError{Fields: fields}
		}
		item.File = normalized
		input.Attachments[index] = item
	}
	var result AttachmentBatchResult
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			member, err := lockAttachmentBatchConversation(ctx, tx, identity, input)
			if err != nil {
				return err
			}
			result = AttachmentBatchResult{ConversationID: member.Conversation.ID, Messages: []ConversationMessage{}}
			for _, item := range input.Attachments {
				message, err := savePendingAttachment(ctx, tx, identity, member, item, backend)
				if err != nil {
					return err
				}
				result.Messages = append(result.Messages, message)
			}
			if input.TargetIdentityID != "" {
				target, err := loadDirectTarget(ctx, tx, identity.Organization.ID, input.TargetIdentityID)
				if err != nil {
					return err
				}
				summary, err := loadDirectConversationSummary(ctx, tx, identity.Organization.ID, member.Conversation.ID, target)
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
		if _, ok := retryableUniqueViolation(err, map[string]struct{}{"messages_organization_idempotency_unique": {}, "direct_conversations_organization_identity_pair_unique": {}}); !ok {
			return AttachmentBatchResult{}, err
		}
	}
	return AttachmentBatchResult{}, err
}

// savePendingAttachment 先校验消息重放，再为首次发送创建临时文件，取消后的重放保持原状态。
func savePendingAttachment(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, member chatstate.Member, item AttachmentBatchItem, backend domain.FileStorageBackend) (ConversationMessage, error) {
	existing := &servermodels.Message{}
	err := tx.NewSelect().Model(existing).Where("msg.organization_id = ? AND msg.idempotency_key = ?", identity.Organization.ID, "mmsg:"+identity.OrganizationIdentity.ID+":"+item.ClientMessageID).Scan(ctx)
	if err == nil {
		if existing.Type != string(domain.MessageTypeAttachment) || existing.Body != item.Body || existing.ConversationID != member.Conversation.ID || existing.SenderParticipantID == nil || *existing.SenderParticipantID != member.ParticipantID {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		messages := []ConversationMessage{memberConversationMessage(existing, member.SubjectID, identity.OrganizationIdentity)}
		if err := loadMessageAttachments(ctx, tx, identity.Organization.ID, messages); err != nil {
			return ConversationMessage{}, err
		}
		attachment := messages[0].Attachment
		if attachment.Name != item.File.FileName || attachment.ContentType != item.File.ContentType || attachment.ByteSize != item.File.ByteSize || attachment.ImageWidth != item.ImageWidth || attachment.ImageHeight != item.ImageHeight {
			return ConversationMessage{}, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		return messages[0], nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, err
	}
	file, err := fileaction.CreatePending(ctx, tx, identity, backend, item.File)
	if err != nil {
		return ConversationMessage{}, err
	}
	return saveAttachmentMessage(ctx, tx, identity, member, attachmentMessageContent{Pending: true, Body: item.Body, ClientMessageID: item.ClientMessageID, FileID: file.ID, ImageWidth: item.ImageWidth, ImageHeight: item.ImageHeight})
}

// lockAttachmentBatchConversation 找到或创建成员单聊并锁定发送资格。
func lockAttachmentBatchConversation(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, input AttachmentBatchInput) (chatstate.Member, error) {
	conversationID := input.ConversationID
	if input.TargetIdentityID != "" {
		if input.TargetIdentityID == identity.OrganizationIdentity.ID {
			return chatstate.Member{}, ErrDirectTargetNotFound
		}
		if _, err := loadDirectTarget(ctx, tx, identity.Organization.ID, input.TargetIdentityID); err != nil {
			return chatstate.Member{}, err
		}
		conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
		if err != nil {
			return chatstate.Member{}, err
		}
		if conversation == nil {
			conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
			if err != nil {
				return chatstate.Member{}, err
			}
		}
		conversationID = conversation.ID
	}
	member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
	if err != nil {
		return member, err
	}
	if member.Conversation.Type != string(domain.ConversationTypeDirect) {
		return member, ErrConversationNotFound
	}
	if input.TargetIdentityID != "" && member.Conversation.Status == string(domain.ConversationStatusArchived) {
		if _, err := tx.NewUpdate().Model(member.Conversation).Set("status = ?", domain.ConversationStatusActive).Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
			return member, err
		}
	}
	_, err = loadDirectSendContext(ctx, tx, identity, conversationID)
	return member, err
}
