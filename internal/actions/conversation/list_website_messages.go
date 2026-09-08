//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/storage/server/messagequery"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const websiteMessagePageSize = 50

// ListWebsiteMessagesQuery 分页读取网站访客客户线程消息。
type ListWebsiteMessagesQuery struct {
	db *bun.DB
}

type websiteMessageRow struct {
	ID                      string                           `bun:"id"`
	Body                    string                           `bun:"body"`
	SenderIdentityType      *domain.OrganizationIdentityType `bun:"sender_identity_type"`
	OriginatedAt            time.Time                        `bun:"originated_at"`
	SourceOrder             int64                            `bun:"source_order"`
	CreatedAt               time.Time                        `bun:"created_at"`
	SubjectKind             string                           `bun:"subject_kind"`
	ReplyToMessageID        *string                          `bun:"reply_to_message_id"`
	ReplyDeleted            bool                             `bun:"reply_deleted"`
	ReplyBody               string                           `bun:"reply_body"`
	ReplySubjectKind        string                           `bun:"reply_subject_kind"`
	ReplySenderIdentityType *domain.OrganizationIdentityType `bun:"reply_sender_identity_type"`
}

// NewListWebsiteMessagesQuery 创建网站访客消息历史查询。
func NewListWebsiteMessagesQuery(db *bun.DB) *ListWebsiteMessagesQuery {
	return &ListWebsiteMessagesQuery{db: db}
}

// Execute 返回指定客户线程的消息页。
func (q *ListWebsiteMessagesQuery) Execute(ctx context.Context, input MessageHistoryInput) (MessageHistory, error) {
	fields := validateMessageHistoryInput(input)
	if len(fields) > 0 {
		return MessageHistory{}, &ValidationError{Fields: fields}
	}
	channel, err := loadWebsiteChannel(ctx, q.db, input.ChannelID)
	if err != nil {
		return MessageHistory{}, err
	}
	identity := &servermodels.ContactChannelIdentity{}
	err = q.db.NewSelect().Model(identity).
		Where("cci.organization_id = ?", channel.OrganizationID).
		Where("cci.channel_id = ?", channel.ID).
		Where("cci.external_id = ?", input.ExternalID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return MessageHistory{}, ErrConversationNotFound
	}
	if err != nil {
		return MessageHistory{}, fmt.Errorf("load website message identity: %w", err)
	}
	owned, err := q.db.NewSelect().
		TableExpr("customer_conversations AS cc").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id AND cv.type = ?", domain.ConversationTypeCustomer).
		Where("cc.organization_id = ?", channel.OrganizationID).
		Where("cc.conversation_id = ?", input.ConversationID).
		Where("cc.contact_channel_identity_id = ?", identity.ID).
		Exists(ctx)
	if err != nil {
		return MessageHistory{}, fmt.Errorf("check website conversation ownership: %w", err)
	}
	if !owned {
		return MessageHistory{}, ErrConversationNotFound
	}

	query := q.db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS id").
		ColumnExpr("msg.body AS body").
		ColumnExpr("oi.type AS sender_identity_type").
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.source_order AS source_order").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("cs.kind AS subject_kind").
		ColumnExpr("msg.reply_to_message_id").
		ColumnExpr("reply.deleted_at IS NOT NULL AS reply_deleted").
		ColumnExpr("? AS reply_body", messagequery.Summary("reply")).
		ColumnExpr("reply_cs.kind AS reply_subject_kind").
		ColumnExpr("reply_oi.type AS reply_sender_identity_type").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN messages AS reply ON reply.id = msg.reply_to_message_id AND reply.organization_id = msg.organization_id AND reply.conversation_id = msg.conversation_id AND reply.type IN (?, ?)", domain.MessageTypeText, domain.MessageTypeAttachment).
		Join("LEFT JOIN conversation_participants AS reply_cp ON reply_cp.id = reply.sender_participant_id AND reply_cp.organization_id = reply.organization_id AND reply_cp.conversation_id = reply.conversation_id").
		Join("LEFT JOIN chat_subjects AS reply_cs ON reply_cs.id = reply_cp.subject_id AND reply_cs.organization_id = reply_cp.organization_id").
		Join("LEFT JOIN organization_identities AS reply_oi ON reply_oi.id = reply_cs.source_id AND reply_oi.organization_id = reply_cs.organization_id AND reply_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", channel.OrganizationID).
		Where("msg.conversation_id = ?", input.ConversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL")
	if input.Before != nil {
		query = query.Where("(msg.originated_at, msg.source_order, msg.id) < (?, ?, ?)", input.Before.OriginatedAt, input.Before.SourceOrder, input.Before.ID).
			OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC")
	} else if input.After != nil {
		query = query.Where("(msg.originated_at, msg.source_order, msg.id) > (?, ?, ?)", input.After.OriginatedAt, input.After.SourceOrder, input.After.ID).
			OrderExpr("msg.originated_at ASC, msg.source_order ASC, msg.id ASC")
	} else {
		query = query.OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC")
	}
	var rows []websiteMessageRow
	if err := query.Limit(websiteMessagePageSize+1).Scan(ctx, &rows); err != nil {
		return MessageHistory{}, fmt.Errorf("list website conversation messages: %w", err)
	}
	return buildMessageHistory(rows, input), nil
}

// validateMessageHistoryInput 校验消息分页输入。
func validateMessageHistoryInput(input MessageHistoryInput) map[string]ValidationCode {
	fields := map[string]ValidationCode{}
	if !common.ValidUUID(input.ChannelID) {
		fields["channelId"] = ValidationChannelIDInvalid
	}
	if !validWebsiteExternalID(input.ExternalID) {
		fields["visitorToken"] = ValidationExternalIDInvalid
	}
	if !common.ValidUUID(input.ConversationID) {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	if input.Before != nil && input.After != nil {
		fields["cursor"] = ValidationCursorInvalid
	}
	for _, cursor := range []*MessageCursorPoint{input.Before, input.After} {
		if cursor != nil && (cursor.OriginatedAt.IsZero() || cursor.SourceOrder < 0 || !common.ValidUUID(cursor.ID)) {
			fields["cursor"] = ValidationCursorInvalid
		}
	}
	return fields
}

// buildMessageHistory 构造正序消息页。
func buildMessageHistory(rows []websiteMessageRow, input MessageHistoryInput) MessageHistory {
	hasMore := len(rows) > websiteMessagePageSize
	if hasMore {
		rows = rows[:websiteMessagePageSize]
	}
	if input.After == nil {
		slices.Reverse(rows)
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		author := domain.MessageAuthorAgent
		if row.SubjectKind == string(domain.ChatSubjectKindContact) {
			author = domain.MessageAuthorVisitor
		}
		message := Message{
			ID: row.ID, Author: author, Body: row.Body, SenderIdentityType: row.SenderIdentityType,
			OriginatedAt: row.OriginatedAt, SourceOrder: row.SourceOrder, CreatedAt: row.CreatedAt,
		}
		if row.ReplyToMessageID != nil {
			message.ReplyTo = &MessageReference{ID: *row.ReplyToMessageID, Deleted: row.ReplyDeleted}
			if !row.ReplyDeleted {
				message.ReplyTo.Author = domain.MessageAuthorAgent
				if row.ReplySubjectKind == string(domain.ChatSubjectKindContact) {
					message.ReplyTo.Author = domain.MessageAuthorVisitor
				}
				message.ReplyTo.Body = row.ReplyBody
				message.ReplyTo.SenderIdentityType = row.ReplySenderIdentityType
			}
		}
		messages = append(messages, message)
	}
	result := MessageHistory{Messages: messages}
	if len(rows) == 0 {
		return result
	}
	first := MessageCursorPoint{OriginatedAt: rows[0].OriginatedAt, SourceOrder: rows[0].SourceOrder, ID: rows[0].ID}
	last := MessageCursorPoint{OriginatedAt: rows[len(rows)-1].OriginatedAt, SourceOrder: rows[len(rows)-1].SourceOrder, ID: rows[len(rows)-1].ID}
	switch {
	case input.Before != nil:
		if hasMore {
			result.Before = &first
		}
	case input.After != nil:
		result.After = &last
	default:
		if hasMore {
			result.Before = &first
		}
		result.After = &last
	}
	return result
}
