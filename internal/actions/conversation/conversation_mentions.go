//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ConversationNavigationState 表示群聊可见尾端及个人提及进度。
type ConversationNavigationState struct {
	PendingMentionCount      int
	ReviewedThroughMessageID *string
	ReviewedThroughSequence  int64
	LatestMessageID          *string
	LatestSequence           int64
}

// PendingConversationMentions 表示一轮固定的轻量提及队列。
type PendingConversationMentions struct {
	MessageIDs         []string
	LastTargetSequence *int64
}

// ConversationMentionReview 表示连续确认结果。
type ConversationMentionReview struct {
	ReviewedThroughMessageID *string
	ReviewedThroughSequence  int64
	Outcome                  string
}

// GetConversationNavigationStateQuery 读取群聊导航状态。
type GetConversationNavigationStateQuery struct{ db *bun.DB }

// ListPendingConversationMentionsQuery 读取尚未连续确认的提及。
type ListPendingConversationMentionsQuery struct{ db *bun.DB }

// MarkConversationMentionReviewedAction 连续确认个人群聊提及。
type MarkConversationMentionReviewedAction struct{ db *bun.DB }

// NewGetConversationNavigationStateQuery 创建群聊导航状态查询。
func NewGetConversationNavigationStateQuery(db *bun.DB) *GetConversationNavigationStateQuery {
	return &GetConversationNavigationStateQuery{db: db}
}

// NewListPendingConversationMentionsQuery 创建提及队列查询。
func NewListPendingConversationMentionsQuery(db *bun.DB) *ListPendingConversationMentionsQuery {
	return &ListPendingConversationMentionsQuery{db: db}
}

// NewMarkConversationMentionReviewedAction 创建连续确认命令。
func NewMarkConversationMentionReviewedAction(db *bun.DB) *MarkConversationMentionReviewedAction {
	return &MarkConversationMentionReviewedAction{db: db}
}

// groupNavigationQuery 限定当前用户可阅读的群聊并关联提及水位。
func groupNavigationQuery(db bun.IDB, identity *servermodels.Identity, conversationID string) *bun.SelectQuery {
	return db.NewSelect().TableExpr("conversations AS cv").
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS subject ON subject.organization_id = mine.organization_id AND subject.id = mine.subject_id AND subject.kind = ? AND subject.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Join("LEFT JOIN conversation_user_states AS state ON state.organization_id = cv.organization_id AND state.conversation_id = cv.id AND state.user_id = ?", identity.User.ID).
		Join("LEFT JOIN messages AS reviewed ON reviewed.organization_id = cv.organization_id AND reviewed.conversation_id = cv.id AND reviewed.id = state.last_reviewed_mention_message_id").
		Where("cv.organization_id = ? AND cv.id = ? AND cv.type = ?", identity.Organization.ID, conversationID, domain.ConversationTypeGroup).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived)
}

// pendingMentionsQuery 共用提及资格，排除本人消息并按连续确认水位过滤。
func pendingMentionsQuery(db bun.IDB) *bun.SelectQuery {
	return db.NewSelect().TableExpr("messages AS pending").
		Join("JOIN conversation_participants AS sender ON sender.organization_id = pending.organization_id AND sender.conversation_id = pending.conversation_id AND sender.id = pending.sender_participant_id").
		Where("pending.organization_id = cv.organization_id AND pending.conversation_id = cv.id").
		Where("pending.deleted_at IS NULL AND sender.subject_id <> mine.subject_id").
		Where("pending.group_message_sequence > COALESCE(reviewed.group_message_sequence, 0)").
		Where(`pending.mention_all OR EXISTS (SELECT 1 FROM message_mentions AS mention WHERE mention.organization_id = pending.organization_id AND mention.message_id = pending.id AND mention.subject_id = mine.subject_id)`)
}

// Execute 在单个查询快照内统计待查看提及并读取最新可见消息。
func (q *GetConversationNavigationStateQuery) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (ConversationNavigationState, error) {
	if !common.ValidUUID(conversationID) {
		return ConversationNavigationState{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var result ConversationNavigationState
	err := groupNavigationQuery(q.db, identity, conversationID).
		ColumnExpr("(?) AS pending_mention_count", pendingMentionsQuery(q.db).ColumnExpr("count(*)")).
		ColumnExpr("state.last_reviewed_mention_message_id AS reviewed_through_message_id").
		ColumnExpr("COALESCE(reviewed.group_message_sequence, 0) AS reviewed_through_sequence").
		ColumnExpr("latest.id AS latest_message_id, COALESCE(latest.group_message_sequence, 0) AS latest_sequence").
		Join("LEFT JOIN LATERAL (SELECT id, group_message_sequence FROM messages WHERE organization_id = cv.organization_id AND conversation_id = cv.id AND deleted_at IS NULL ORDER BY group_message_sequence DESC LIMIT 1) AS latest ON TRUE").
		Scan(ctx, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return ConversationNavigationState{}, ErrConversationNotFound
	}
	if err != nil {
		return ConversationNavigationState{}, fmt.Errorf("get conversation navigation state: %w", err)
	}
	return result, nil
}

// Execute 一次返回同一快照中的有序目标 ID 和本轮上界。
func (q *ListPendingConversationMentionsQuery) Execute(ctx context.Context, identity *servermodels.Identity, conversationID string) (PendingConversationMentions, error) {
	if !common.ValidUUID(conversationID) {
		return PendingConversationMentions{}, &ValidationError{Fields: map[string]ValidationCode{"conversationId": ValidationConversationIDInvalid}}
	}
	var rows []struct {
		ID       *string
		Sequence *int64
	}
	err := groupNavigationQuery(q.db, identity, conversationID).
		ColumnExpr("targets.id, targets.group_message_sequence AS sequence").
		Join("LEFT JOIN LATERAL (?) AS targets ON TRUE", pendingMentionsQuery(q.db).ColumnExpr("pending.id, pending.group_message_sequence")).
		OrderExpr("targets.group_message_sequence ASC").Scan(ctx, &rows)
	if err != nil {
		return PendingConversationMentions{}, fmt.Errorf("list pending conversation mentions: %w", err)
	}
	if len(rows) == 0 {
		return PendingConversationMentions{}, ErrConversationNotFound
	}
	result := PendingConversationMentions{MessageIDs: []string{}}
	for _, row := range rows {
		if row.ID != nil {
			result.MessageIDs = append(result.MessageIDs, *row.ID)
			result.LastTargetSequence = row.Sequence
		}
	}
	return result, nil
}

// Execute 在会话锁内仅确认最早有效提及，重复请求返回当前进度。
func (a *MarkConversationMentionReviewedAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID, messageID string) (ConversationMentionReview, error) {
	if !common.ValidUUID(conversationID) || !common.ValidUUID(messageID) {
		return ConversationMentionReview{}, &ValidationError{Fields: map[string]ValidationCode{"messageId": ValidationLastReadMessageIDInvalid}}
	}
	var result ConversationMentionReview
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		conversation, err := lockConversationMember(ctx, tx, identity, conversationID)
		if err != nil {
			return err
		}
		if conversation.Type != string(domain.ConversationTypeGroup) {
			return ErrConversationNotFound
		}
		var target struct {
			Sequence int64
			Deleted  bool
			Eligible bool
		}
		err = groupNavigationQuery(tx, identity, conversationID).
			Join("JOIN messages AS target ON target.organization_id = cv.organization_id AND target.conversation_id = cv.id AND target.id = ?", messageID).
			Join("JOIN conversation_participants AS sender ON sender.organization_id = target.organization_id AND sender.conversation_id = target.conversation_id AND sender.id = target.sender_participant_id").
			ColumnExpr("target.group_message_sequence AS sequence, target.deleted_at IS NOT NULL AS deleted").
			ColumnExpr("sender.subject_id <> mine.subject_id AND (target.mention_all OR EXISTS (SELECT 1 FROM message_mentions AS mention WHERE mention.organization_id = target.organization_id AND mention.message_id = target.id AND mention.subject_id = mine.subject_id)) AS eligible").Scan(ctx, &target)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMentionTargetInvalid
		}
		if err != nil {
			return err
		}
		if !target.Eligible {
			return ErrMentionTargetInvalid
		}
		if err := groupNavigationQuery(tx, identity, conversationID).
			ColumnExpr("state.last_reviewed_mention_message_id AS reviewed_through_message_id, COALESCE(reviewed.group_message_sequence, 0) AS reviewed_through_sequence").Scan(ctx, &result); err != nil {
			return err
		}
		if target.Sequence <= result.ReviewedThroughSequence {
			result.Outcome = "alreadyReviewed"
			return nil
		}
		if target.Deleted {
			result.Outcome = "unavailable"
			return nil
		}
		var firstID string
		if err := groupNavigationQuery(tx, identity, conversationID).
			ColumnExpr("(?) AS first_id", pendingMentionsQuery(tx).ColumnExpr("pending.id").OrderExpr("pending.group_message_sequence ASC").Limit(1)).Scan(ctx, &firstID); err != nil {
			return err
		}
		if firstID != messageID {
			return ErrMentionProgressChanged
		}
		state := &servermodels.ConversationUserState{OrganizationID: identity.Organization.ID, ConversationID: conversationID, UserID: identity.User.ID, LastReviewedMentionMessageID: &messageID}
		if _, err := tx.NewInsert().Model(state).Column("organization_id", "conversation_id", "user_id", "last_reviewed_mention_message_id").
			On("CONFLICT (organization_id, conversation_id, user_id) DO UPDATE").Set("last_reviewed_mention_message_id = EXCLUDED.last_reviewed_mention_message_id").Set("updated_at = now()").Exec(ctx); err != nil {
			return err
		}
		result = ConversationMentionReview{ReviewedThroughMessageID: &messageID, ReviewedThroughSequence: target.Sequence, Outcome: "reviewed"}
		return nil
	})
	if err != nil {
		return ConversationMentionReview{}, fmt.Errorf("mark conversation mention reviewed: %w", err)
	}
	return result, nil
}
