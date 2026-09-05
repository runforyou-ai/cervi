//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

type messageReferenceRow struct {
	Deleted       bool    `bun:"deleted"`
	MessageID     string  `bun:"message_id"`
	Body          string  `bun:"body"`
	ChatSubjectID *string `bun:"chat_subject_id"`
	Kind          *string `bun:"kind"`
	SourceID      *string `bun:"source_id"`
	DisplayName   *string `bun:"display_name"`
}

// loadConversationReplyTarget 校验并读取同一会话中的文本引用目标。
func loadConversationReplyTarget(ctx context.Context, db bun.IDB, organizationID, conversationID, messageID string) (*ConversationMessageReference, error) {
	if messageID == "" {
		return nil, nil
	}
	reference, err := loadMessageReference(ctx, db, organizationID, conversationID, messageID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && reference.Deleted) {
		return nil, &ConflictError{Reason: ConflictReasonReplyTargetInvalid}
	}
	if err != nil {
		return nil, err
	}
	return reference, nil
}

// loadMessageReference 读取文本消息引用，并将软删除表示为不可用摘要。
func loadMessageReference(ctx context.Context, db bun.IDB, organizationID, conversationID, messageID string) (*ConversationMessageReference, error) {
	row := messageReferenceRow{}
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS message_id").
		ColumnExpr("CASE WHEN msg.deleted_at IS NULL THEN msg.body ELSE '' END AS body").
		ColumnExpr("msg.deleted_at IS NOT NULL AS deleted").
		ColumnExpr("cs.id AS chat_subject_id").
		ColumnExpr("cs.kind AS kind").
		ColumnExpr("cs.source_id AS source_id").
		ColumnExpr("oi.display_name AS display_name").
		Join("LEFT JOIN conversation_participants AS cp ON cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id AND cp.id = msg.sender_participant_id").
		Join("LEFT JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id").
		Join("LEFT JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ?", organizationID).
		Where("msg.conversation_id = ?", conversationID).
		Where("msg.id = ?", messageID).
		Where("msg.type = ?", domain.MessageTypeText).
		Scan(ctx, &row)
	if err != nil {
		return nil, err
	}
	if row.Deleted {
		return &ConversationMessageReference{ID: row.MessageID, Deleted: true}, nil
	}
	if row.ChatSubjectID == nil || row.Kind == nil || row.SourceID == nil {
		return nil, ErrDataInvariant
	}
	return &ConversationMessageReference{
		ID: row.MessageID, Body: row.Body,
		Sender: &ConversationMessageSender{
			ChatSubjectID: *row.ChatSubjectID, Kind: domain.ChatSubjectKind(*row.Kind),
			SourceID: *row.SourceID, DisplayName: row.DisplayName,
		},
	}, nil
}
