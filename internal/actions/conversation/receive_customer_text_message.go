//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// InboundCustomerTextMessageInput 定义渠道文本入站事务的稳定事实。
type InboundCustomerTextMessageInput struct {
	ExternalID              string
	DisplayName             *string
	RequestedConversationID *string
	SingleConversation      bool
	Body                    string
	IdempotencyKey          string
	OriginatedAt            time.Time
	SourceOrder             int64
}

// InboundCustomerTextMessageResult 返回渠道文本入站事务创建或取得的事实。
type InboundCustomerTextMessageResult struct {
	Summary              ConversationSummary
	Session              *servermodels.ServiceSession
	Message              *servermodels.Message
	ChannelIdentityID    string
	CreatedConversation  bool
	OpenedServiceSession bool
}

// ReceiveInboundCustomerTextMessage 在调用方事务中幂等写入客户文本消息。
func ReceiveInboundCustomerTextMessage(ctx context.Context, db bun.IDB, channel *servermodels.Channel, input InboundCustomerTextMessageInput) (InboundCustomerTextMessageResult, error) {
	ids, err := generateIDs()
	if err != nil {
		return InboundCustomerTextMessageResult{}, fmt.Errorf("generate inbound customer message ids: %w", err)
	}
	ensured, err := contactaction.EnsureChannelIdentity(ctx, db, contactaction.EnsureChannelIdentityInput{
		OrganizationID: channel.OrganizationID,
		ChannelID:      channel.ID,
		ExternalID:     input.ExternalID,
		ContactID:      ids.contact,
		IdentityID:     ids.channelIdentity,
	})
	if err != nil {
		return InboundCustomerTextMessageResult{}, err
	}
	identity := ensured.Identity
	if input.DisplayName != nil && (identity.DisplayName == nil || *identity.DisplayName != *input.DisplayName) {
		if _, err := db.NewUpdate().Model(identity).
			Set("display_name = ?", *input.DisplayName).
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", channel.OrganizationID).
			Exec(ctx); err != nil {
			return InboundCustomerTextMessageResult{}, fmt.Errorf("update channel identity display name: %w", err)
		}
		identity.DisplayName = input.DisplayName
	}
	if saved, found, err := loadInboundCustomerTextMessage(ctx, db, channel, identity, input); err != nil || found {
		return saved, err
	}

	subject, err := ensureContactSubject(ctx, db, channel.OrganizationID, ensured.Contact.ID, ids.subject)
	if err != nil {
		return InboundCustomerTextMessageResult{}, err
	}
	var conversation *servermodels.Conversation
	var insertedConversation bool
	if input.SingleConversation {
		conversation, insertedConversation, err = selectSingleCustomerConversation(ctx, db, channel.OrganizationID, identity.ID, input.Body, ids.conversation)
	} else {
		conversation, insertedConversation, err = selectTargetConversation(ctx, db, channel.OrganizationID, identity.ID, input.RequestedConversationID, input.Body, ids.conversation)
	}
	if err != nil {
		return InboundCustomerTextMessageResult{}, err
	}
	participant, err := ensureContactParticipant(ctx, db, channel.OrganizationID, conversation.ID, subject.ID, ids.participant)
	if err != nil {
		return InboundCustomerTextMessageResult{}, err
	}

	var session *servermodels.ServiceSession
	var createSession bool
	if insertedConversation {
		createSession = true
		session = &servermodels.ServiceSession{Sequence: 1}
	} else {
		session, createSession, err = selectServiceSession(ctx, db, channel.OrganizationID, conversation.ID, identity.ID)
		if err != nil {
			return InboundCustomerTextMessageResult{}, err
		}
	}
	if conversation.Status == string(domain.ConversationStatusArchived) {
		if _, err := db.NewUpdate().Model(conversation).
			Set("status = ?", domain.ConversationStatusActive).
			Set("updated_at = now()").
			WherePK().
			Where("organization_id = ?", channel.OrganizationID).
			Exec(ctx); err != nil {
			return InboundCustomerTextMessageResult{}, fmt.Errorf("reactivate customer conversation: %w", err)
		}
		conversation.Status = string(domain.ConversationStatusActive)
	}

	if createSession {
		route, err := resolveRouteSnapshot(ctx, db, channel, input.OriginatedAt)
		if err != nil {
			return InboundCustomerTextMessageResult{}, err
		}
		session = &servermodels.ServiceSession{
			ID: ids.serviceSession, OrganizationID: channel.OrganizationID,
			ConversationID: conversation.ID, ContactChannelIdentityID: identity.ID,
			Sequence: session.Sequence, Status: string(domain.ServiceSessionStatusOpen),
			TeamID: route.teamID, AssigneeIdentityID: route.assigneeIdentityID,
			OpeningMessageID: ids.message, LastMessageID: ids.message,
			LastMessageAt: input.OriginatedAt, LastMessageSourceOrder: input.SourceOrder,
			AssignedAt: route.assignedAt, StatusChangedAt: input.OriginatedAt,
		}
		if _, err := db.NewInsert().Model(session).
			Column("id", "organization_id", "conversation_id", "contact_channel_identity_id", "sequence", "status", "team_id", "assignee_identity_id", "opening_message_id", "last_message_id", "last_message_at", "last_message_source_order", "assigned_at", "status_changed_at").
			Returning("*").
			Exec(ctx); err != nil {
			return InboundCustomerTextMessageResult{}, fmt.Errorf("create service session: %w", err)
		}
	}

	message := &servermodels.Message{
		ID: ids.message, OrganizationID: channel.OrganizationID,
		ConversationID: conversation.ID, ServiceSessionID: &session.ID,
		SenderParticipantID: &participant.ID, Type: string(domain.MessageTypeText),
		Body: input.Body, IdempotencyKey: &input.IdempotencyKey,
		OriginatedAt: input.OriginatedAt, SourceOrder: input.SourceOrder,
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "idempotency_key", "originated_at", "source_order").
		Returning("*").
		Exec(ctx); err != nil {
		return InboundCustomerTextMessageResult{}, fmt.Errorf("create inbound customer message: %w", err)
	}
	if !createSession {
		if err := updateSessionSummary(ctx, db, session, message); err != nil {
			return InboundCustomerTextMessageResult{}, err
		}
	}
	if err := updateConversationSummary(ctx, db, conversation, message); err != nil {
		return InboundCustomerTextMessageResult{}, err
	}
	if _, err := db.NewUpdate().Model(identity).
		Set("last_seen_at = CASE WHEN last_seen_at IS NULL OR last_seen_at < ? THEN ? ELSE last_seen_at END", input.OriginatedAt, input.OriginatedAt).
		Set("updated_at = now()").
		WherePK().
		Where("organization_id = ?", channel.OrganizationID).
		Exec(ctx); err != nil {
		return InboundCustomerTextMessageResult{}, fmt.Errorf("update channel identity last seen: %w", err)
	}

	summary, err := loadConversationSummary(ctx, db, channel.OrganizationID, conversation.ID, identity.ID)
	if err != nil {
		return InboundCustomerTextMessageResult{}, err
	}
	return inboundCustomerTextMessageResult(summary, session, message), nil
}

// loadInboundCustomerTextMessage 校验并返回已经写入的渠道文本消息。
func loadInboundCustomerTextMessage(ctx context.Context, db bun.IDB, channel *servermodels.Channel, identity *servermodels.ContactChannelIdentity, input InboundCustomerTextMessageInput) (InboundCustomerTextMessageResult, bool, error) {
	message := &servermodels.Message{}
	err := db.NewSelect().Model(message).
		Where("msg.organization_id = ?", channel.OrganizationID).
		Where("msg.idempotency_key = ?", input.IdempotencyKey).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return InboundCustomerTextMessageResult{}, false, nil
	}
	if err != nil {
		return InboundCustomerTextMessageResult{}, false, fmt.Errorf("find idempotent inbound customer message: %w", err)
	}
	if message.ServiceSessionID == nil || message.SenderParticipantID == nil || message.Type != string(domain.MessageTypeText) || message.DeletedAt != nil {
		return InboundCustomerTextMessageResult{}, true, ErrDataInvariant
	}
	if message.Body != input.Body || (input.RequestedConversationID != nil && *input.RequestedConversationID != message.ConversationID) {
		return InboundCustomerTextMessageResult{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	session := &servermodels.ServiceSession{}
	err = db.NewSelect().Model(session).
		Join("JOIN customer_conversations AS cc ON cc.organization_id = ss.organization_id AND cc.conversation_id = ss.conversation_id").
		Join("JOIN conversation_participants AS cp ON cp.organization_id = cc.organization_id AND cp.conversation_id = cc.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id").
		Where("cc.organization_id = ?", channel.OrganizationID).
		Where("cc.conversation_id = ?", message.ConversationID).
		Where("cc.contact_channel_identity_id = ?", identity.ID).
		Where("ss.id = ?", *message.ServiceSessionID).
		Where("ss.contact_channel_identity_id = ?", identity.ID).
		Where("cp.id = ?", *message.SenderParticipantID).
		Where("cs.kind = ?", domain.ChatSubjectKindContact).
		Where("cs.source_id = ?", identity.ContactID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return InboundCustomerTextMessageResult{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	if err != nil {
		return InboundCustomerTextMessageResult{}, true, fmt.Errorf("check idempotent inbound customer message ownership: %w", err)
	}
	summary, err := loadConversationSummary(ctx, db, channel.OrganizationID, message.ConversationID, identity.ID)
	if err != nil {
		return InboundCustomerTextMessageResult{}, true, err
	}
	return inboundCustomerTextMessageResult(summary, session, message), true, nil
}

// inboundCustomerTextMessageResult 构造渠道文本入站结果。
func inboundCustomerTextMessageResult(summary ConversationSummary, session *servermodels.ServiceSession, message *servermodels.Message) InboundCustomerTextMessageResult {
	openedSession := session.OpeningMessageID == message.ID
	return InboundCustomerTextMessageResult{
		Summary: summary, Session: session, Message: message,
		ChannelIdentityID:    session.ContactChannelIdentityID,
		CreatedConversation:  openedSession && session.Sequence == 1,
		OpenedServiceSession: openedSession,
	}
}

// selectSingleCustomerConversation 取得渠道身份固定映射的最早客户会话。
func selectSingleCustomerConversation(ctx context.Context, db bun.IDB, organizationID, channelIdentityID, body, conversationID string) (*servermodels.Conversation, bool, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Join("JOIN customer_conversations AS cc ON cc.organization_id = cv.organization_id AND cc.conversation_id = cv.id").
		Where("cv.organization_id = ?", organizationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Where("cc.contact_channel_identity_id = ?", channelIdentityID).
		OrderExpr("cc.created_at ASC, cc.conversation_id ASC").
		Limit(1).
		Scan(ctx)
	if err == nil {
		return conversation, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("load channel identity customer conversation: %w", err)
	}
	return createCustomerConversation(ctx, db, organizationID, channelIdentityID, body, conversationID)
}
