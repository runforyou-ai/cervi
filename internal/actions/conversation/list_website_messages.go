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
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const websiteMessagePageSize = 50

// ListWebsiteMessagesQuery 分页读取网站访客客户线程消息。
type ListWebsiteMessagesQuery struct {
	db *bun.DB
}

type websiteMessageRow struct {
	ID           string    `bun:"id"`
	Body         string    `bun:"body"`
	OriginatedAt time.Time `bun:"originated_at"`
	CreatedAt    time.Time `bun:"created_at"`
	SubjectKind  string    `bun:"subject_kind"`
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
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("cs.kind AS subject_kind").
		Join("JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Where("msg.organization_id = ?", channel.OrganizationID).
		Where("msg.conversation_id = ?", input.ConversationID).
		Where("msg.type = ?", domain.MessageTypeText).
		Where("msg.deleted_at IS NULL")
	if input.Before != nil {
		query = query.Where("(msg.originated_at, msg.id) < (?, ?)", input.Before.OriginatedAt, input.Before.ID).
			OrderExpr("msg.originated_at DESC, msg.id DESC")
	} else if input.After != nil {
		query = query.Where("(msg.originated_at, msg.id) > (?, ?)", input.After.OriginatedAt, input.After.ID).
			OrderExpr("msg.originated_at ASC, msg.id ASC")
	} else {
		query = query.OrderExpr("msg.originated_at DESC, msg.id DESC")
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
		if cursor != nil && (cursor.OriginatedAt.IsZero() || !common.ValidUUID(cursor.ID)) {
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
		messages = append(messages, Message{
			ID: row.ID, Author: author, Body: row.Body,
			OriginatedAt: row.OriginatedAt, CreatedAt: row.CreatedAt,
		})
	}
	result := MessageHistory{Messages: messages}
	if len(rows) == 0 {
		return result
	}
	first := MessageCursorPoint{OriginatedAt: rows[0].OriginatedAt, ID: rows[0].ID}
	last := MessageCursorPoint{OriginatedAt: rows[len(rows)-1].OriginatedAt, ID: rows[len(rows)-1].ID}
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
