//go:build server

package conversation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

const conversationMessagePageSize = 50

// ListConversationMessagesQuery 分页读取成员可见的会话消息。
type ListConversationMessagesQuery struct {
	db *bun.DB
}

type conversationMessageRow struct {
	ID                             string                           `bun:"id"`
	Type                           string                           `bun:"type"`
	Body                           string                           `bun:"body"`
	SystemEventType                *string                          `bun:"system_event_type"`
	SystemEventPayload             json.RawMessage                  `bun:"system_event_payload"`
	OriginatedAt                   time.Time                        `bun:"originated_at"`
	GroupMessageSequence           *int64                           `bun:"group_message_sequence"`
	SourceOrder                    int64                            `bun:"source_order"`
	CreatedAt                      time.Time                        `bun:"created_at"`
	SenderSubjectID                *string                          `bun:"sender_subject_id"`
	SenderKind                     *string                          `bun:"sender_kind"`
	SenderSourceID                 *string                          `bun:"sender_source_id"`
	SenderDisplayName              *string                          `bun:"sender_display_name"`
	SenderAvatarFileID             *string                          `bun:"sender_avatar_file_id"`
	SenderIdentityType             *domain.OrganizationIdentityType `bun:"sender_identity_type"`
	ReplyToMessageID               *string                          `bun:"reply_to_message_id"`
	MentionAll                     bool                             `bun:"mention_all"`
	ReplyToDeleted                 bool                             `bun:"reply_to_deleted"`
	ReplyToBody                    *string                          `bun:"reply_to_body"`
	ReplyToSenderSubjectID         *string                          `bun:"reply_to_sender_subject_id"`
	ReplyToSenderKind              *string                          `bun:"reply_to_sender_kind"`
	ReplyToSenderSourceID          *string                          `bun:"reply_to_sender_source_id"`
	ReplyToSenderDisplayName       *string                          `bun:"reply_to_sender_display_name"`
	ReplyToSenderAvatarFileID      *string                          `bun:"reply_to_sender_avatar_file_id"`
	ReplyToSenderIdentityType      *domain.OrganizationIdentityType `bun:"reply_to_sender_identity_type"`
	ServiceSessionOpeningMessageID *string                          `bun:"service_session_opening_message_id"`
	ServiceSessionSequence         *int64                           `bun:"service_session_sequence"`
	ServiceSessionStartedAt        *time.Time                       `bun:"service_session_started_at"`
	ServiceSessionStatus           *string                          `bun:"service_session_status"`
}

// NewListConversationMessagesQuery 创建成员消息历史查询。
func NewListConversationMessagesQuery(db *bun.DB) *ListConversationMessagesQuery {
	return &ListConversationMessagesQuery{db: db}
}

// Execute 在同一只读快照中读取消息、关系和双向边界。
func (q *ListConversationMessagesQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ConversationMessageHistoryInput) (ConversationMessageHistory, error) {
	if fields := validateConversationMessageHistoryInput(input); len(fields) > 0 {
		return ConversationMessageHistory{}, &ValidationError{Fields: fields}
	}
	var history ConversationMessageHistory
	err := q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := authorizeConversationHistory(ctx, tx, identity, input.ConversationID); err != nil {
			return err
		}
		var conversationType domain.ConversationType
		if err := tx.NewSelect().TableExpr("conversations").Column("type").Where("organization_id = ? AND id = ?", identity.Organization.ID, input.ConversationID).Scan(ctx, &conversationType); err != nil {
			return err
		}
		group := conversationType == domain.ConversationTypeGroup
		for _, cursor := range []*MessageCursorPoint{input.Before, input.After} {
			if cursor != nil && (cursor.GroupMessageSequence != nil) != group {
				return &ValidationError{Fields: map[string]ValidationCode{"cursor": ValidationCursorInvalid}}
			}
		}
		rows, err := loadConversationWindowRows(ctx, tx, identity, input, group)
		if err != nil {
			return err
		}
		history, err = buildConversationMessageHistory(rows)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			history.HasEarlier, err = conversationMessagesQuery(tx, identity, input.ConversationID).Where("?", messageCursorCondition(*history.Before, "<")).Exists(ctx)
			if err != nil {
				return err
			}
			history.HasLater, err = conversationMessagesQuery(tx, identity, input.ConversationID).Where("?", messageCursorCondition(*history.After, ">")).Exists(ctx)
			if err != nil {
				return err
			}
		}
		return loadConversationMessageMentions(ctx, tx, identity.Organization.ID, history.Messages)
	})
	if err != nil {
		return ConversationMessageHistory{}, fmt.Errorf("read conversation message window: %w", err)
	}
	return history, nil
}

// conversationMessagesQuery 共用消息正文、发送者、引用和系统事件查询。
func conversationMessagesQuery(db bun.IDB, identity *servermodels.Identity, conversationID string) *bun.SelectQuery {
	return db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS id").
		ColumnExpr("msg.type AS type").
		ColumnExpr("msg.body AS body").
		ColumnExpr("msg.system_event_type AS system_event_type").
		ColumnExpr("msg.system_event_payload AS system_event_payload").
		ColumnExpr("msg.originated_at AS originated_at").
		ColumnExpr("msg.source_order AS source_order").
		ColumnExpr("msg.group_message_sequence AS group_message_sequence").
		ColumnExpr("msg.created_at AS created_at").
		ColumnExpr("cs.id AS sender_subject_id").
		ColumnExpr("cs.kind AS sender_kind").
		ColumnExpr("cs.source_id AS sender_source_id").
		ColumnExpr("CASE WHEN cs.kind = ? THEN COALESCE(cci.display_name, c.display_name) WHEN cs.kind = ? THEN oi.display_name END AS sender_display_name", domain.ChatSubjectKindContact, domain.ChatSubjectKindOrganizationIdentity).
		ColumnExpr("CASE WHEN cs.kind = ? THEN cci.avatar_file_id ELSE oi.avatar_file_id END::text AS sender_avatar_file_id", domain.ChatSubjectKindContact).
		ColumnExpr("oi.type AS sender_identity_type").
		ColumnExpr("msg.reply_to_message_id AS reply_to_message_id").
		ColumnExpr("msg.mention_all AS mention_all").
		ColumnExpr("CASE WHEN reply_msg.deleted_at IS NULL THEN reply_msg.body END AS reply_to_body").
		ColumnExpr("reply_msg.deleted_at IS NOT NULL AS reply_to_deleted").
		ColumnExpr("reply_cs.id AS reply_to_sender_subject_id").
		ColumnExpr("reply_cs.kind AS reply_to_sender_kind").
		ColumnExpr("reply_cs.source_id AS reply_to_sender_source_id").
		ColumnExpr("reply_oi.display_name AS reply_to_sender_display_name").
		ColumnExpr("reply_oi.avatar_file_id::text AS reply_to_sender_avatar_file_id").
		ColumnExpr("reply_oi.type AS reply_to_sender_identity_type").
		ColumnExpr("ss.opening_message_id AS service_session_opening_message_id").
		ColumnExpr("ss.sequence AS service_session_sequence").
		ColumnExpr("ss.created_at AS service_session_started_at").
		ColumnExpr("ss.status AS service_session_status").
		Join("LEFT JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN service_sessions AS ss ON ss.id = msg.service_session_id AND ss.organization_id = msg.organization_id AND ss.conversation_id = msg.conversation_id").
		Join("LEFT JOIN customer_conversations AS cc ON cc.conversation_id = msg.conversation_id AND cc.organization_id = msg.organization_id").
		Join("LEFT JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id AND cci.contact_id = cs.source_id AND cs.kind = ?", domain.ChatSubjectKindContact).
		Join("LEFT JOIN contacts AS c ON c.id = cs.source_id AND c.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindContact).
		Join("LEFT JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("LEFT JOIN messages AS reply_msg ON reply_msg.organization_id = msg.organization_id AND reply_msg.conversation_id = msg.conversation_id AND reply_msg.id = msg.reply_to_message_id AND reply_msg.type = ?", domain.MessageTypeText).
		Join("LEFT JOIN conversation_participants AS reply_cp ON reply_cp.organization_id = reply_msg.organization_id AND reply_cp.conversation_id = reply_msg.conversation_id AND reply_cp.id = reply_msg.sender_participant_id").
		Join("LEFT JOIN chat_subjects AS reply_cs ON reply_cs.organization_id = reply_cp.organization_id AND reply_cs.id = reply_cp.subject_id").
		Join("LEFT JOIN organization_identities AS reply_oi ON reply_oi.organization_id = reply_cs.organization_id AND reply_oi.id = reply_cs.source_id AND reply_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", identity.Organization.ID).
		Where("msg.conversation_id = ?", conversationID).
		Where("msg.type IN (?)", bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeSystem})).
		Where("msg.deleted_at IS NULL")
}

// messageCursorCondition 根据游标类型构造时间线边界。
func messageCursorCondition(point MessageCursorPoint, operator string) schema.QueryWithArgs {
	if point.GroupMessageSequence != nil {
		return bun.SafeQuery("msg.group_message_sequence "+operator+" ?", *point.GroupMessageSequence)
	}
	return bun.SafeQuery("(msg.originated_at, msg.source_order, msg.id) "+operator+" (?, ?, ?)", point.OriginatedAt, point.SourceOrder, point.ID)
}

// loadConversationWindowRows 按当前方向或目标读取连续消息行。
func loadConversationWindowRows(ctx context.Context, db bun.IDB, identity *servermodels.Identity, input ConversationMessageHistoryInput, group bool) ([]conversationMessageRow, error) {
	order := "msg.originated_at, msg.source_order, msg.id"
	if group {
		order = "msg.group_message_sequence"
	}
	var rows []conversationMessageRow
	if input.AroundMessageID != "" {
		var target conversationMessageRow
		err := conversationMessagesQuery(db, identity, input.ConversationID).Where("msg.id = ?", input.AroundMessageID).Scan(ctx, &target)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMessageUnavailable
		}
		if err != nil {
			return nil, err
		}
		point := MessageCursorPoint{ID: target.ID, OriginatedAt: target.OriginatedAt, SourceOrder: target.SourceOrder, GroupMessageSequence: target.GroupMessageSequence}
		if group && target.GroupMessageSequence == nil {
			return nil, ErrDataInvariant
		}
		if err := conversationMessagesQuery(db, identity, input.ConversationID).Where("?", messageCursorCondition(point, "<")).OrderExpr(orderDescending(order)).Limit(25).Scan(ctx, &rows); err != nil {
			return nil, err
		}
		slices.Reverse(rows)
		rows = append(rows, target)
		var later []conversationMessageRow
		if err := conversationMessagesQuery(db, identity, input.ConversationID).Where("?", messageCursorCondition(point, ">")).OrderExpr(order).Limit(25).Scan(ctx, &later); err != nil {
			return nil, err
		}
		return append(rows, later...), nil
	}
	query := conversationMessagesQuery(db, identity, input.ConversationID)
	if input.Before != nil {
		query = query.Where("?", messageCursorCondition(*input.Before, "<"))
	}
	if input.After != nil {
		query = query.Where("?", messageCursorCondition(*input.After, ">"))
	} else {
		order = orderDescending(order)
	}
	if err := query.OrderExpr(order).Limit(conversationMessagePageSize).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	if input.After == nil {
		slices.Reverse(rows)
	}
	if group {
		for _, row := range rows {
			if row.GroupMessageSequence == nil {
				return nil, ErrDataInvariant
			}
		}
	}
	return rows, nil
}

// orderDescending 将固定时间线排序列转换为倒序。
func orderDescending(columns string) string {
	return strings.ReplaceAll(columns, ", ", " DESC, ") + " DESC"
}

// authorizeConversationHistory 对不同会话类型应用各自的成员可见性规则。
func authorizeConversationHistory(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) error {
	var conversationType string
	err := db.NewSelect().
		TableExpr("conversations AS cv").
		ColumnExpr("cv.type").
		Where("cv.organization_id = ?", identity.Organization.ID).
		Where("cv.id = ?", conversationID).
		Scan(ctx, &conversationType)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationNotFound
	}
	if err != nil {
		return fmt.Errorf("load conversation type for history: %w", err)
	}

	switch domain.ConversationType(conversationType) {
	case domain.ConversationTypeCustomer:
		available, err := db.NewSelect().
			TableExpr("customer_conversations AS cc").
			Where("cc.organization_id = ?", identity.Organization.ID).
			Where("cc.conversation_id = ?", conversationID).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("check customer conversation access: %w", err)
		}
		if !available {
			return ErrConversationNotFound
		}
		return nil
	case domain.ConversationTypeDirect, domain.ConversationTypeGroup:
		available, err := db.NewSelect().
			TableExpr("conversation_participants AS cp").
			Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id").
			Where("cp.organization_id = ?", identity.Organization.ID).
			Where("cp.conversation_id = ?", conversationID).
			Where("cp.left_at IS NULL").
			Where("cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
			Where("cs.source_id = ?", identity.OrganizationIdentity.ID).
			Exists(ctx)
		if err != nil {
			return fmt.Errorf("check internal conversation access: %w", err)
		}
		if !available {
			return ErrConversationNotFound
		}
		return nil
	default:
		return ErrConversationNotFound
	}
}

// validateConversationMessageHistoryInput 校验成员消息历史输入。
func validateConversationMessageHistoryInput(input ConversationMessageHistoryInput) map[string]ValidationCode {
	fields := map[string]ValidationCode{}
	if !common.ValidUUID(input.ConversationID) {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	if (input.Before != nil && input.After != nil) || (input.AroundMessageID != "" && (input.Before != nil || input.After != nil || !common.ValidUUID(input.AroundMessageID))) {
		fields["cursor"] = ValidationCursorInvalid
	}
	for _, cursor := range []*MessageCursorPoint{input.Before, input.After} {
		if cursor != nil && (!common.ValidUUID(cursor.ID) || (cursor.GroupMessageSequence == nil && (cursor.OriginatedAt.IsZero() || cursor.SourceOrder < 0)) || (cursor.GroupMessageSequence != nil && *cursor.GroupMessageSequence <= 0)) {
			fields["cursor"] = ValidationCursorInvalid
		}
	}
	return fields
}

// buildConversationMessageHistory 构造正序成员消息页。
func buildConversationMessageHistory(rows []conversationMessageRow) (ConversationMessageHistory, error) {
	messages := make([]ConversationMessage, 0, len(rows))
	for _, row := range rows {
		message := ConversationMessage{
			ID: row.ID, Type: domain.MessageType(row.Type), Body: row.Body,
			OriginatedAt: row.OriginatedAt, SourceOrder: row.SourceOrder, CreatedAt: row.CreatedAt, MentionAll: row.MentionAll, GroupMessageSequence: row.GroupMessageSequence,
		}
		if message.Type == domain.MessageTypeSystem {
			if row.SystemEventType == nil || len(row.SystemEventPayload) == 0 {
				return ConversationMessageHistory{}, fmt.Errorf("load conversation system event: %w", ErrDataInvariant)
			}
			event := &ConversationSystemEvent{Type: domain.ConversationSystemEventType(*row.SystemEventType)}
			if err := json.Unmarshal(row.SystemEventPayload, event); err != nil {
				return ConversationMessageHistory{}, fmt.Errorf("decode conversation system event: %w", err)
			}
			message.SystemEvent = event
		}
		if row.SenderSubjectID != nil && row.SenderKind != nil && row.SenderSourceID != nil {
			message.Sender = &ConversationMessageSender{
				ChatSubjectID: *row.SenderSubjectID,
				Kind:          domain.ChatSubjectKind(*row.SenderKind),
				SourceID:      *row.SenderSourceID,
				DisplayName:   row.SenderDisplayName, AvatarFileID: row.SenderAvatarFileID, IdentityType: row.SenderIdentityType,
			}
		}
		if row.ReplyToMessageID != nil && row.ReplyToDeleted {
			message.ReplyTo = &ConversationMessageReference{ID: *row.ReplyToMessageID, Deleted: true}
		} else if row.ReplyToMessageID != nil {
			if row.ReplyToBody == nil || row.ReplyToSenderSubjectID == nil || row.ReplyToSenderKind == nil || row.ReplyToSenderSourceID == nil {
				return ConversationMessageHistory{}, fmt.Errorf("load conversation reply reference: %w", ErrDataInvariant)
			}
			message.ReplyTo = &ConversationMessageReference{
				ID: *row.ReplyToMessageID, Body: *row.ReplyToBody,
				Sender: &ConversationMessageSender{
					ChatSubjectID: *row.ReplyToSenderSubjectID,
					Kind:          domain.ChatSubjectKind(*row.ReplyToSenderKind),
					SourceID:      *row.ReplyToSenderSourceID,
					DisplayName:   row.ReplyToSenderDisplayName, AvatarFileID: row.ReplyToSenderAvatarFileID, IdentityType: row.ReplyToSenderIdentityType,
				},
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
		return result, nil
	}
	first := rows[0]
	last := rows[len(rows)-1]
	result.Before = &MessageCursorPoint{OriginatedAt: first.OriginatedAt, SourceOrder: first.SourceOrder, ID: first.ID, GroupMessageSequence: first.GroupMessageSequence}
	result.After = &MessageCursorPoint{OriginatedAt: last.OriginatedAt, SourceOrder: last.SourceOrder, ID: last.ID, GroupMessageSequence: last.GroupMessageSequence}
	return result, nil
}
