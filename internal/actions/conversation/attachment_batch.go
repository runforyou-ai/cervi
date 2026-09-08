//go:build server

package conversation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"strings"
	"unicode/utf8"
)

// ExecuteBatch 在同一事务内按选择顺序保存附件，最后保存说明。
func (a *SendAttachmentMessageAction) ExecuteBatch(ctx context.Context, identity *servermodels.Identity, input AttachmentBatchInput) (AttachmentBatchResult, error) {
	input.Body = strings.TrimSpace(input.Body)
	if !common.ValidUUID(input.BatchID) || len(input.Attachments) == 0 || len(input.Attachments) > 100 || utf8.RuneCountInString(input.Body) > 4000 ||
		(input.ConversationID == "") == (input.TargetIdentityID == "") ||
		(input.ConversationID != "" && !common.ValidUUID(input.ConversationID)) || (input.TargetIdentityID != "" && !common.ValidUUID(input.TargetIdentityID)) {
		return AttachmentBatchResult{}, ErrConversationNotFound
	}
	seen := map[string]bool{input.BatchID: true}
	for _, item := range input.Attachments {
		if !common.ValidUUID(item.FileID) || !common.ValidUUID(item.ClientMessageID) || seen[item.ClientMessageID] || seen[item.FileID] || item.ImageWidth < 0 || item.ImageHeight < 0 {
			return AttachmentBatchResult{}, ErrConversationNotFound
		}
		seen[item.ClientMessageID] = true
		seen[item.FileID] = true
	}
	payload, _ := json.Marshal(input)
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	var result AttachmentBatchResult
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			batch := &servermodels.MessageBatch{}
			err := tx.NewSelect().Model(batch).Where("mb.id = ?", input.BatchID).Scan(ctx)
			if err == nil {
				if batch.OrganizationID != identity.Organization.ID || batch.SenderIdentityID != identity.OrganizationIdentity.ID || batch.RequestDigest != digest {
					return &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
				}
				result, err = loadAttachmentBatch(ctx, tx, identity, batch, input.TargetIdentityID)
				return err
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			member, err := lockAttachmentBatchConversation(ctx, tx, identity, input)
			if err != nil {
				return err
			}
			result = AttachmentBatchResult{ConversationID: member.Conversation.ID, Messages: []ConversationMessage{}}
			ids := []string{}
			for _, item := range input.Attachments {
				item.ConversationID = member.Conversation.ID
				item.Pending = true
				message, err := saveAttachmentMessage(ctx, tx, identity, member, item)
				if err != nil {
					return err
				}
				result.Messages = append(result.Messages, message)
				ids = append(ids, message.ID)
			}
			if input.Body != "" {
				message, err := saveInternalTextMessage(ctx, tx, identity, InternalTextMessageInput{ConversationID: member.Conversation.ID, ClientMessageID: input.BatchID, Body: input.Body}, internalMessageContext{ConversationID: member.Conversation.ID, ParticipantID: member.ParticipantID, SubjectID: member.SubjectID}, nil)
				if err != nil {
					return err
				}
				result.Messages = append(result.Messages, message)
				ids = append(ids, message.ID)
			}
			batch = &servermodels.MessageBatch{ID: input.BatchID, OrganizationID: identity.Organization.ID, SenderIdentityID: identity.OrganizationIdentity.ID, ConversationID: member.Conversation.ID, RequestDigest: digest, MessageIDs: ids}
			if _, err := tx.NewInsert().Model(batch).Exec(ctx); err != nil {
				return err
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
		if _, ok := retryableUniqueViolation(err, map[string]struct{}{"message_batches_pkey": {}, "messages_organization_idempotency_unique": {}, "direct_conversations_organization_identity_pair_unique": {}}); !ok {
			return AttachmentBatchResult{}, err
		}
	}
	return AttachmentBatchResult{}, err
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

// loadAttachmentBatch 按原批次顺序返回已保存的发送结果。
func loadAttachmentBatch(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, batch *servermodels.MessageBatch, targetID string) (AttachmentBatchResult, error) {
	if err := authorizeConversationHistory(ctx, tx, identity, batch.ConversationID); err != nil {
		return AttachmentBatchResult{}, err
	}
	rows := []conversationMessageRow{}
	if err := conversationMessagesQuery(tx, identity, batch.ConversationID).Where("msg.id IN (?)", bun.In(batch.MessageIDs)).OrderExpr("msg.originated_at ASC, msg.source_order ASC, msg.id ASC").Scan(ctx, &rows); err != nil {
		return AttachmentBatchResult{}, err
	}
	history, err := buildConversationMessageHistory(rows)
	if err != nil {
		return AttachmentBatchResult{}, err
	}
	if err := loadMessageAttachments(ctx, tx, identity.Organization.ID, history.Messages); err != nil {
		return AttachmentBatchResult{}, err
	}
	result := AttachmentBatchResult{ConversationID: batch.ConversationID, Messages: history.Messages}
	if targetID != "" {
		target, err := loadDirectTarget(ctx, tx, identity.Organization.ID, targetID)
		if err != nil {
			return result, err
		}
		summary, err := loadDirectConversationSummary(ctx, tx, identity.Organization.ID, batch.ConversationID, target)
		if err != nil {
			return result, err
		}
		result.Conversation = &summary
	}
	return result, nil
}
