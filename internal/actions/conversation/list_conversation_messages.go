//go:build server

package conversation

import (
	"context"
	"fmt"
	"slices"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const conversationMessagePageSize = 50

// ListConversationMessagesQuery 分页读取成员可见的客户会话消息。
type ListConversationMessagesQuery struct {
	db *bun.DB
}

type conversationMessageRow struct {
	ID                             string     `bun:"id"`
	Type                           string     `bun:"type"`
	Body                           string     `bun:"body"`
	OriginatedAt                   time.Time  `bun:"originated_at"`
	SourceOrder                    int64      `bun:"source_order"`
	CreatedAt                      time.Time  `bun:"created_at"`
	SenderSubjectID                *string    `bun:"sender_subject_id"`
	SenderKind                     *string    `bun:"sender_kind"`
	SenderDisplayName              *string    `bun:"sender_display_name"`
	ServiceSessionOpeningMessageID *string    `bun:"service_session_opening_message_id"`
	ServiceSessionSequence         *int64     `bun:"service_session_sequence"`
	ServiceSessionStartedAt        *time.Time `bun:"service_session_started_at"`
	ServiceSessionStatus           *string    `bun:"service_session_status"`
}

// NewListConversationMessagesQuery 创建成员消息历史查询。
func NewListConversationMessagesQuery(db *bun.DB) *ListConversationMessagesQuery {
	return &ListConversationMessagesQuery{db: db}
}

// Execute 返回当前企业客户 Conversation 的消息页。
func (q *ListConversationMessagesQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ConversationMessageHistoryInput) (ConversationMessageHistory, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return ConversationMessageHistory{}, err
	}
	if fields := validateConversationMessageHistoryInput(input); len(fields) > 0 {
		return ConversationMessageHistory{}, &ValidationError{Fields: fields}
	}

	available, err := q.db.NewSelect().
		TableExpr("conversations AS cv").
		Join("JOIN customer_conversations AS cc ON cc.conversation_id = cv.id AND cc.organization_id = cv.organization_id").
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", input.ConversationID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Exists(ctx)
	if err != nil {
		return ConversationMessageHistory{}, fmt.Errorf("check customer conversation access: %w", err)
	}
	if !available {
		return ConversationMessageHistory{}, ErrConversationNotFound
	}

	query := q.db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS id").
		ColumnExpr("msg.type AS type").
		ColumnExpr("msg.body AS body").
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.source_order AS source_order").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("cs.id AS sender_subject_id").
		ColumnExpr("cs.kind AS sender_kind").
		ColumnExpr("CASE WHEN cs.kind = ? THEN COALESCE(cci.display_name, c.display_name) WHEN cs.kind = ? THEN oi.display_name END AS sender_display_name", domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity).
		ColumnExpr("ss.opening_message_id AS service_session_opening_message_id").
		ColumnExpr("ss.sequence AS service_session_sequence").
		ColumnExpr("ss.created_at AS service_session_started_at").
		ColumnExpr("ss.status AS service_session_status").
		Join("LEFT JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN service_sessions AS ss ON ss.id = msg.service_session_id AND ss.organization_id = msg.organization_id AND ss.conversation_id = msg.conversation_id").
		Join("JOIN customer_conversations AS cc ON cc.conversation_id = msg.conversation_id AND cc.organization_id = msg.organization_id").
		Join("LEFT JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id AND cci.contact_id = cs.source_id AND cs.kind = ?", domain.ChatSubjectKindContact).
		Join("LEFT JOIN contacts AS c ON c.id = cs.source_id AND c.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindContact).
		Join("LEFT JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", identity.Organization.ID).
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

	var rows []conversationMessageRow
	if err := query.Limit(conversationMessagePageSize+1).Scan(ctx, &rows); err != nil {
		return ConversationMessageHistory{}, fmt.Errorf("list customer conversation messages: %w", err)
	}
	return buildConversationMessageHistory(rows, input), nil
}

// validateConversationMessageHistoryInput 校验成员消息历史输入。
func validateConversationMessageHistoryInput(input ConversationMessageHistoryInput) map[string]ValidationCode {
	fields := map[string]ValidationCode{}
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

// buildConversationMessageHistory 构造正序成员消息页。
func buildConversationMessageHistory(rows []conversationMessageRow, input ConversationMessageHistoryInput) ConversationMessageHistory {
	hasMore := len(rows) > conversationMessagePageSize
	if hasMore {
		rows = rows[:conversationMessagePageSize]
	}
	if input.After == nil {
		slices.Reverse(rows)
	}

	messages := make([]ConversationMessage, 0, len(rows))
	for _, row := range rows {
		message := ConversationMessage{
			ID: row.ID, Type: domain.MessageType(row.Type), Body: row.Body,
			OriginatedAt: row.OriginatedAt, CreatedAt: row.CreatedAt,
		}
		if row.SenderSubjectID != nil && row.SenderKind != nil {
			message.Sender = &ConversationMessageSender{
				ChatSubjectID: *row.SenderSubjectID,
				Kind:          domain.ChatSubjectKind(*row.SenderKind),
				DisplayName:   row.SenderDisplayName,
			}
		}
		if row.ServiceSessionOpeningMessageID != nil && *row.ServiceSessionOpeningMessageID == row.ID {
			message.SessionStart = &ConversationMessageSessionStart{
				Sequence:  *row.ServiceSessionSequence,
				StartedAt: *row.ServiceSessionStartedAt,
				Status:    domain.ServiceSessionStatus(*row.ServiceSessionStatus),
			}
		}
		messages = append(messages, message)
	}

	result := ConversationMessageHistory{Messages: messages}
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
