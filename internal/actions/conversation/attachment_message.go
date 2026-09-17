//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// SendAttachmentMessageAction 保存内部会话附件并激活文件。
type SendAttachmentMessageAction struct {
	db        *bun.DB
	scheduler AgentChatMessageScheduler
}

// NewSendAttachmentMessageAction 创建附件消息发送操作。
func NewSendAttachmentMessageAction(db *bun.DB, scheduler AgentChatMessageScheduler) *SendAttachmentMessageAction {
	return &SendAttachmentMessageAction{db: db, scheduler: scheduler}
}

// Execute 在成员和会话锁内幂等发送一个已上传的附件，首发时按需创建单聊、AI 聊天或 Copilot 线程，AI 聊天与 Copilot 线程首次保存时追加 Agent 输入。
func (a *SendAttachmentMessageAction) Execute(ctx context.Context, identity *servermodels.Identity, input AttachmentMessageInput) (AttachmentMessageResult, error) {
	clientMessageID, valid := common.NormalizeUUID(input.ClientMessageID)
	input.ClientMessageID = clientMessageID
	input.Body = strings.TrimSpace(input.Body)
	if !valid || !common.ValidUUID(input.FileID) ||
		(input.ConversationID == "") == (input.TargetIdentityID == "") ||
		(input.ConversationID != "" && !common.ValidUUID(input.ConversationID)) ||
		(input.TargetIdentityID != "" && !common.ValidUUID(input.TargetIdentityID)) ||
		(input.AgentIdentityID != "" && (input.TargetIdentityID != "" || !common.ValidUUID(input.AgentIdentityID))) ||
		(input.CustomerConversationID != "" && (input.AgentIdentityID == "" || !common.ValidUUID(input.CustomerConversationID))) ||
		input.ImageWidth < 0 || input.ImageHeight < 0 || utf8.RuneCountInString(input.Body) > 4000 {
		return AttachmentMessageResult{}, ErrConversationNotFound
	}
	var result AttachmentMessageResult
	var err error
	for attempt := 0; attempt < maxWriteAttempts; attempt++ {
		err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
			if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
				return err
			}
			member, agentContext, err := lockAttachmentConversation(ctx, tx, identity, input)
			if err != nil {
				return err
			}
			message, inserted, err := saveAttachmentMessage(ctx, tx, identity, member, input)
			if err != nil {
				return err
			}
			if agentContext != nil && inserted {
				if a.scheduler == nil || agentContext.AgentRevisionID == nil {
					return ErrDataInvariant
				}
				if err := a.scheduler.Schedule(ctx, tx, identity.Organization.ID, member.Conversation.ID, agentContext.AgentIdentityID, *agentContext.AgentRevisionID, message.ID, agentContext.SubjectID, agentContext.AgentInputKind); err != nil {
					return fmt.Errorf("schedule agent input attachment: %w", err)
				}
			}
			result = AttachmentMessageResult{ConversationID: member.Conversation.ID, Message: message}
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
			if input.AgentIdentityID != "" && input.CustomerConversationID == "" {
				summary, err := inboxaction.NewLoadInboxQuery(tx).LoadAgentConversation(ctx, identity, member.Conversation.ID)
				if err != nil {
					return err
				}
				result.AgentConversation = &summary
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

// lockAttachmentConversation 找到或创建附件所属的单聊、AI 聊天或 Copilot 线程并锁定发送资格，AI 聊天与 Copilot 线程同时返回 Agent 发送上下文。
func lockAttachmentConversation(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, input AttachmentMessageInput) (chatstate.Member, *internalMessageContext, error) {
	conversationID := input.ConversationID
	if input.AgentIdentityID != "" {
		// AI 聊天草稿按草稿编号创建会话，标题取附件说明，没有说明时取文件名。
		title := input.Body
		if title == "" {
			err := tx.NewSelect().Model((*servermodels.File)(nil)).Column("original_name").
				Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ? AND f.purpose = ?", input.FileID, identity.Organization.ID, identity.User.ID, domain.FilePurposeMessageAttachment).Scan(ctx, &title)
			if errors.Is(err, sql.ErrNoRows) {
				return chatstate.Member{}, nil, fileaction.ErrFileNotFound
			}
			if err != nil {
				return chatstate.Member{}, nil, err
			}
		}
		// 指定所属客户会话时首发 Copilot 线程，否则首发 AI 聊天。
		if input.CustomerConversationID != "" {
			if err := ensureCustomerCopilotThread(ctx, tx, identity, conversationID, input.CustomerConversationID, input.AgentIdentityID, title); err != nil {
				return chatstate.Member{}, nil, err
			}
		} else if err := ensureAgentConversation(ctx, tx, identity, conversationID, input.AgentIdentityID, title); err != nil {
			return chatstate.Member{}, nil, err
		}
	}
	if input.TargetIdentityID != "" {
		if input.TargetIdentityID == identity.OrganizationIdentity.ID {
			return chatstate.Member{}, nil, ErrDirectTargetNotFound
		}
		if _, err := loadDirectTarget(ctx, tx, identity.Organization.ID, input.TargetIdentityID); err != nil {
			return chatstate.Member{}, nil, err
		}
		conversation, err := findDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
		if err != nil {
			return chatstate.Member{}, nil, err
		}
		if conversation == nil {
			conversation, err = createDirectConversation(ctx, tx, identity.Organization.ID, identity.OrganizationIdentity.ID, input.TargetIdentityID)
			if err != nil {
				return chatstate.Member{}, nil, err
			}
		}
		conversationID = conversation.ID
	}
	conversation, err := chatstate.LockConversation(ctx, tx, identity.Organization.ID, conversationID)
	if err != nil {
		return chatstate.Member{}, nil, err
	}
	// Copilot 线程按所属客户会话授权，提问成员在发送时加入线程参与者。
	if conversation.Type == string(domain.ConversationTypeCopilot) {
		copilotContext, err := lockCustomerCopilotSendContext(ctx, tx, identity, conversationID)
		if err != nil {
			return chatstate.Member{}, nil, err
		}
		return chatstate.Member{Conversation: copilotContext.Conversation, ParticipantID: copilotContext.ParticipantID, SubjectID: copilotContext.SubjectID}, &copilotContext, nil
	}
	member, err := chatstate.LockMember(ctx, tx, identity, conversationID)
	if err != nil {
		return member, nil, err
	}
	switch domain.ConversationType(member.Conversation.Type) {
	case domain.ConversationTypeAgent:
		agentContext, err := lockAgentSendContext(ctx, tx, identity, conversationID)
		return member, &agentContext, err
	case domain.ConversationTypeDirect:
		if input.TargetIdentityID != "" && member.Conversation.Status == string(domain.ConversationStatusArchived) {
			if _, err := tx.NewUpdate().Model(member.Conversation).Set("status = ?", domain.ConversationStatusActive).Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
				return member, nil, err
			}
		}
		_, err = loadDirectSendContext(ctx, tx, identity, conversationID)
		return member, nil, err
	case domain.ConversationTypeGroup:
		if member.Conversation.Status != string(domain.ConversationStatusActive) {
			return member, nil, ErrConversationNotFound
		}
		return member, nil, nil
	default:
		return member, nil, ErrConversationNotFound
	}
}

// saveAttachmentMessage 校验完整发送意图并在同一事务内保存消息、附件、文件激活和阅读位置，返回是否新建消息。
func saveAttachmentMessage(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, member chatstate.Member, input AttachmentMessageInput) (ConversationMessage, bool, error) {
	key := "mmsg:" + identity.OrganizationIdentity.ID + ":" + input.ClientMessageID
	existing := &servermodels.Message{}
	err := tx.NewSelect().Model(existing).Where("msg.organization_id = ? AND msg.idempotency_key = ?", identity.Organization.ID, key).Scan(ctx)
	if err == nil {
		var stored struct {
			FileID      string `bun:"file_id"`
			ImageWidth  int    `bun:"image_width"`
			ImageHeight int    `bun:"image_height"`
		}
		if err := tx.NewSelect().Table("message_attachments").ColumnExpr("COALESCE(file_id::text, '') AS file_id, image_width, image_height").
			Where("organization_id = ? AND message_id = ?", identity.Organization.ID, existing.ID).Scan(ctx, &stored); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return ConversationMessage{}, false, err
		}
		if existing.Type != string(domain.MessageTypeAttachment) || existing.ConversationID != member.Conversation.ID ||
			existing.Body != input.Body || existing.SenderParticipantID == nil || *existing.SenderParticipantID != member.ParticipantID ||
			stored.FileID != input.FileID || stored.ImageWidth != input.ImageWidth || stored.ImageHeight != input.ImageHeight {
			return ConversationMessage{}, false, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
		}
		messages := []ConversationMessage{memberConversationMessage(existing, member.SubjectID, identity.OrganizationIdentity)}
		err := loadMessageAttachments(ctx, tx, identity.Organization.ID, messages)
		return messages[0], false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, false, err
	}
	file := &servermodels.File{}
	err = tx.NewSelect().Model(file).ColumnExpr("f.*").ColumnExpr("f.expires_at <= now() AS expired").
		Where("f.id = ? AND f.organization_id = ? AND f.created_by_user_id = ?", input.FileID, identity.Organization.ID, identity.User.ID).
		Where("f.purpose = ?", domain.FilePurposeMessageAttachment).For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ConversationMessage{}, false, fileaction.ErrFileNotFound
	}
	if err != nil {
		return ConversationMessage{}, false, err
	}
	if file.Status != string(domain.FileStatusUploaded) || file.Expired {
		return ConversationMessage{}, false, fileaction.ErrFileNotFound
	}
	message := &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID, ConversationID: member.Conversation.ID,
		SenderParticipantID: &member.ParticipantID, Type: string(domain.MessageTypeAttachment), Body: input.Body,
		SearchVector:    searchtext.Vector(input.Body, file.OriginalName),
		ClientMessageID: &input.ClientMessageID, IdempotencyKey: &key, OriginatedAt: time.Now().UTC(),
	}
	message, _, err = chatstate.AppendMessage(ctx, tx, member.Conversation, message)
	if err != nil {
		return ConversationMessage{}, false, err
	}
	if _, err := tx.NewRaw(`INSERT INTO message_attachments
 (message_id, organization_id, file_id, name, content_type, byte_size, image_width, image_height, transfer_status)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		message.ID, identity.Organization.ID, file.ID, file.OriginalName, file.ContentType, file.ByteSize, input.ImageWidth, input.ImageHeight, domain.MessageAttachmentTransferReady).Exec(ctx); err != nil {
		return ConversationMessage{}, false, err
	}
	if _, err := tx.NewUpdate().Model(file).Set("status = ?", domain.FileStatusActive).Set("expires_at = NULL").Set("updated_at = now()").WherePK().Exec(ctx); err != nil {
		return ConversationMessage{}, false, err
	}
	// Copilot 线程不维护个人会话状态，其余会话推进本人阅读水位。
	if member.Conversation.Type != string(domain.ConversationTypeCopilot) {
		if err := advanceConversationUserReadState(ctx, tx, &servermodels.ConversationUserState{
			OrganizationID: identity.Organization.ID, ConversationID: member.Conversation.ID, UserID: identity.User.ID, LastReadMessageID: &message.ID,
		}, message); err != nil {
			return ConversationMessage{}, false, err
		}
	}
	result := memberConversationMessage(message, member.SubjectID, identity.OrganizationIdentity)
	result.Attachment = &MessageAttachment{ID: file.ID, Name: file.OriginalName, ContentType: file.ContentType, ByteSize: file.ByteSize, ImageWidth: input.ImageWidth, ImageHeight: input.ImageHeight, TransferStatus: domain.MessageAttachmentTransferReady}
	return result, true, nil
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
		ColumnExpr("ma.message_id, COALESCE(ma.file_id::text, '') AS id, ma.name, ma.content_type, ma.byte_size, ma.image_width, ma.image_height, ma.transfer_status").
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
