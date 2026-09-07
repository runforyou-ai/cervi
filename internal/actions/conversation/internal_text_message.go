//go:build server

package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// saveInternalTextMessage 在会话与成员已锁定的事务内幂等保存消息、个人状态和 Agent 输入。
func saveInternalTextMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, input InternalTextMessageInput, sendContext internalMessageContext, agentScheduler AgentChatMessageScheduler) (ConversationMessage, error) {
	idempotencyKey := "mmsg:" + identity.OrganizationIdentity.ID + ":" + input.ClientMessageID
	if saved, found, err := loadIdempotentMemberMessage(ctx, db, identity, input.ConversationID, input.Body, input.ReplyToMessageID, idempotencyKey, false); err != nil || found {
		return saved, err
	}
	replyTo, err := loadConversationReplyTarget(ctx, db, identity.Organization.ID, input.ConversationID, input.ReplyToMessageID)
	if err != nil {
		return ConversationMessage{}, err
	}
	message := &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: identity.Organization.ID,
		ConversationID: input.ConversationID, SenderParticipantID: &sendContext.ParticipantID,
		Type: string(domain.MessageTypeText), Body: input.Body,
		IdempotencyKey: &idempotencyKey, OriginatedAt: time.Now().UTC(),
	}
	if replyTo != nil {
		message.ReplyToMessageID = &replyTo.ID
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "sender_participant_id", "type", "body", "reply_to_message_id", "idempotency_key", "originated_at").
		Returning("*").
		Exec(ctx); err != nil {
		return ConversationMessage{}, fmt.Errorf("create individual text message: %w", err)
	}
	conversation := &servermodels.Conversation{ID: input.ConversationID, OrganizationID: identity.Organization.ID}
	if err := updateConversationSummary(ctx, db, conversation, message); err != nil {
		return ConversationMessage{}, err
	}
	if err := advanceConversationUserReadState(ctx, db, &servermodels.ConversationUserState{
		OrganizationID: identity.Organization.ID, ConversationID: input.ConversationID,
		UserID: identity.User.ID, LastReadMessageID: &message.ID,
	}, message); err != nil {
		return ConversationMessage{}, err
	}
	if sendContext.AgentIdentityID != "" {
		if agentScheduler == nil || sendContext.AgentRevisionID == nil {
			return ConversationMessage{}, ErrDataInvariant
		}
		if err := agentScheduler.Schedule(ctx, db, identity.Organization.ID, input.ConversationID, sendContext.AgentIdentityID, *sendContext.AgentRevisionID, message.ID); err != nil {
			return ConversationMessage{}, fmt.Errorf("schedule AI chat message: %w", err)
		}
	}
	result := memberConversationMessage(message, sendContext.SubjectID, identity.OrganizationIdentity)
	result.ReplyTo = replyTo
	return result, nil
}

// AgentChatMessageScheduler 把 AI 聊天成员消息加入持久化输入流。
type AgentChatMessageScheduler interface {
	Schedule(context.Context, bun.IDB, string, string, string, string, string) error
}

type internalMessageContext struct {
	ConversationID  string  `bun:"conversation_id"`
	ParticipantID   string  `bun:"participant_id"`
	SubjectID       string  `bun:"subject_id"`
	AgentIdentityID string  `bun:"agent_identity_id"`
	AgentRevisionID *string `bun:"agent_revision_id"`
}

// normalizeInternalMessageInput 规范化双方聊天正文和引用。
func normalizeInternalMessageInput(input InternalTextMessageInput) (InternalTextMessageInput, map[string]ValidationCode) {
	conversationID, clientMessageID, body, fields := normalizeInternalTextMessageInput(input.ConversationID, input.ClientMessageID, input.Body)
	input.ConversationID = conversationID
	input.ClientMessageID = clientMessageID
	input.Body = body
	if input.ReplyToMessageID != "" {
		var valid bool
		input.ReplyToMessageID, valid = common.NormalizeUUID(input.ReplyToMessageID)
		if !valid {
			fields["replyToMessageId"] = ValidationReplyToMessageIDInvalid
		}
	}
	return input, fields
}

// normalizeInternalTextMessageInput 规范化内部会话文本消息输入。
func normalizeInternalTextMessageInput(conversationID, clientMessageID, body string) (string, string, string, map[string]ValidationCode) {
	fields := map[string]ValidationCode{}
	body = strings.TrimSpace(body)
	var valid bool
	conversationID, valid = common.NormalizeUUID(conversationID)
	if !valid {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	clientMessageID, valid = common.NormalizeUUID(clientMessageID)
	if !valid {
		fields["clientMessageId"] = ValidationClientMessageIDInvalid
	}
	if body == "" {
		fields["body"] = ValidationBodyRequired
	} else if utf8.RuneCountInString(body) > 4000 {
		fields["body"] = ValidationBodyTooLong
	}
	return conversationID, clientMessageID, body, fields
}
