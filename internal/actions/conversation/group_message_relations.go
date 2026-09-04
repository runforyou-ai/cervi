//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type groupMessageRelationSenderRow struct {
	MessageID     string  `bun:"message_id"`
	Body          string  `bun:"body"`
	ChatSubjectID *string `bun:"chat_subject_id"`
	Kind          *string `bun:"kind"`
	SourceID      *string `bun:"source_id"`
	DisplayName   *string `bun:"display_name"`
}

type groupMentionTargetRow struct {
	MessageID     string  `bun:"message_id"`
	ChatSubjectID string  `bun:"chat_subject_id"`
	Kind          string  `bun:"kind"`
	SourceID      string  `bun:"source_id"`
	DisplayName   *string `bun:"display_name"`
}

// loadConversationMessageMentions 批量补充一页消息的提醒主体。
func loadConversationMessageMentions(ctx context.Context, db bun.IDB, organizationID string, messages []ConversationMessage) error {
	if len(messages) == 0 {
		return nil
	}
	messageIDs := make([]string, 0, len(messages))
	messageIndexes := make(map[string]int, len(messages))
	for index := range messages {
		messages[index].Mentions = []ConversationMessageMention{}
		messageIDs = append(messageIDs, messages[index].ID)
		messageIndexes[messages[index].ID] = index
	}
	rows := make([]groupMentionTargetRow, 0)
	if err := db.NewSelect().
		TableExpr("message_mentions AS mm").
		ColumnExpr("mm.message_id AS message_id").
		ColumnExpr("cs.id AS chat_subject_id").
		ColumnExpr("cs.kind AS kind").
		ColumnExpr("cs.source_id AS source_id").
		ColumnExpr("oi.display_name AS display_name").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = mm.organization_id AND cs.id = mm.subject_id").
		Join("LEFT JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("mm.organization_id = ?", organizationID).
		Where("mm.message_id IN (?)", bun.In(messageIDs)).
		OrderExpr("mm.message_id ASC, mm.subject_id ASC").
		Scan(ctx, &rows); err != nil {
		return fmt.Errorf("load conversation message mentions: %w", err)
	}
	for _, row := range rows {
		index, exists := messageIndexes[row.MessageID]
		if !exists {
			return ErrDataInvariant
		}
		messages[index].Mentions = append(messages[index].Mentions, ConversationMessageMention{
			ChatSubjectID: row.ChatSubjectID, Kind: domain.ChatSubjectKind(row.Kind),
			SourceID: row.SourceID, DisplayName: row.DisplayName,
		})
	}
	return nil
}

// loadGroupReplyTarget 校验并读取同一群聊中的文本引用目标。
func loadGroupReplyTarget(ctx context.Context, db bun.IDB, organizationID, conversationID, messageID string) (*ConversationMessageReference, error) {
	if messageID == "" {
		return nil, nil
	}
	reference, err := loadMessageReference(ctx, db, organizationID, conversationID, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &ConflictError{Reason: ConflictReasonGroupReplyTargetInvalid}
	}
	if err != nil {
		return nil, err
	}
	return reference, nil
}

// loadMessageReference 读取一条未删除文本消息的一层引用摘要。
func loadMessageReference(ctx context.Context, db bun.IDB, organizationID, conversationID, messageID string) (*ConversationMessageReference, error) {
	row := groupMessageRelationSenderRow{}
	err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.id AS message_id").
		ColumnExpr("msg.body AS body").
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
		Where("msg.deleted_at IS NULL").
		Scan(ctx, &row)
	if err != nil {
		return nil, err
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

// loadGroupMentionTargets 校验提醒目标是当前群聊中的有效参与者。
func loadGroupMentionTargets(ctx context.Context, db bun.IDB, organizationID, conversationID, senderSubjectID string, subjectIDs []string) ([]ConversationMessageMention, error) {
	if len(subjectIDs) == 0 {
		return []ConversationMessageMention{}, nil
	}
	rows := make([]groupMentionTargetRow, 0, len(subjectIDs))
	if err := db.NewSelect().
		TableExpr("conversation_participants AS cp").
		ColumnExpr("cs.id AS chat_subject_id").
		ColumnExpr("cs.kind AS kind").
		ColumnExpr("cs.source_id AS source_id").
		ColumnExpr("oi.display_name AS display_name").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN organization_identities AS oi ON oi.organization_id = cs.organization_id AND oi.id = cs.source_id").
		Where("cp.organization_id = ?", organizationID).
		Where("cp.conversation_id = ?", conversationID).
		Where("cp.left_at IS NULL").
		Where("cp.subject_id IN (?)", bun.In(subjectIDs)).
		OrderExpr("cp.subject_id ASC").
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load group mention targets: %w", err)
	}
	if len(rows) != len(subjectIDs) {
		return nil, &ConflictError{Reason: ConflictReasonGroupMentionTargetInvalid}
	}
	mentions := make([]ConversationMessageMention, 0, len(rows))
	for _, row := range rows {
		if row.ChatSubjectID == senderSubjectID {
			return nil, &ConflictError{Reason: ConflictReasonGroupMentionTargetInvalid}
		}
		mentions = append(mentions, ConversationMessageMention{
			ChatSubjectID: row.ChatSubjectID, Kind: domain.ChatSubjectKind(row.Kind),
			SourceID: row.SourceID, DisplayName: row.DisplayName,
		})
	}
	return mentions, nil
}

// createMessageMentions 持久化消息提醒关系。
func createMessageMentions(ctx context.Context, db bun.IDB, organizationID, messageID string, mentions []ConversationMessageMention) error {
	if len(mentions) == 0 {
		return nil
	}
	rows := make([]*servermodels.MessageMention, 0, len(mentions))
	for _, mention := range mentions {
		rows = append(rows, &servermodels.MessageMention{
			OrganizationID: organizationID, MessageID: messageID, SubjectID: mention.ChatSubjectID,
		})
	}
	if _, err := db.NewInsert().Model(&rows).
		Column("organization_id", "message_id", "subject_id").
		Exec(ctx); err != nil {
		return fmt.Errorf("create message mentions: %w", err)
	}
	return nil
}

// loadIdempotentGroupMessage 校验引用和提醒在内的完整群消息意图。
func loadIdempotentGroupMessage(ctx context.Context, db bun.IDB, identity *servermodels.Identity, input GroupTextMessageInput, idempotencyKey string) (ConversationMessage, bool, error) {
	saved, found, err := loadIdempotentMemberMessage(ctx, db, identity, input.ConversationID, input.Body, idempotencyKey, false)
	if err != nil || !found {
		return saved, found, err
	}
	var stored struct {
		ReplyToMessageID *string `bun:"reply_to_message_id"`
		MentionAll       bool    `bun:"mention_all"`
	}
	if err := db.NewSelect().
		TableExpr("messages AS msg").
		ColumnExpr("msg.reply_to_message_id AS reply_to_message_id, msg.mention_all AS mention_all").
		Where("msg.organization_id = ?", identity.Organization.ID).
		Where("msg.id = ?", saved.ID).
		Scan(ctx, &stored); err != nil {
		return ConversationMessage{}, true, fmt.Errorf("load idempotent group reply: %w", err)
	}
	storedMentionSubjectIDs := make([]string, 0)
	if err := db.NewSelect().
		TableExpr("message_mentions AS mm").
		ColumnExpr("mm.subject_id").
		Where("mm.organization_id = ?", identity.Organization.ID).
		Where("mm.message_id = ?", saved.ID).
		OrderExpr("mm.subject_id ASC").
		Scan(ctx, &storedMentionSubjectIDs); err != nil {
		return ConversationMessage{}, true, fmt.Errorf("load idempotent group mentions: %w", err)
	}
	storedReply := ""
	if stored.ReplyToMessageID != nil {
		storedReply = *stored.ReplyToMessageID
	}
	if storedReply != input.ReplyToMessageID || stored.MentionAll != input.MentionAll || !slices.Equal(storedMentionSubjectIDs, input.MentionSubjectIDs) {
		return ConversationMessage{}, true, &ConflictError{Reason: ConflictReasonIdempotencyMismatch}
	}
	if storedReply != "" {
		saved.ReplyTo, err = loadMessageReference(ctx, db, identity.Organization.ID, input.ConversationID, storedReply)
		if err != nil {
			return ConversationMessage{}, true, fmt.Errorf("load idempotent group reply reference: %w", err)
		}
	}
	saved.Mentions, err = loadPersistedMessageMentions(ctx, db, identity.Organization.ID, saved.ID)
	if err != nil {
		return ConversationMessage{}, true, err
	}
	saved.MentionAll = stored.MentionAll
	return saved, true, nil
}

// loadPersistedMessageMentions 读取一条消息已经保存的提醒主体。
func loadPersistedMessageMentions(ctx context.Context, db bun.IDB, organizationID, messageID string) ([]ConversationMessageMention, error) {
	messages := []ConversationMessage{{ID: messageID}}
	if err := loadConversationMessageMentions(ctx, db, organizationID, messages); err != nil {
		return nil, err
	}
	return messages[0].Mentions, nil
}
