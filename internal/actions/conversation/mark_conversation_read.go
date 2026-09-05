//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ConversationReadState 表示用户会话的已读水位。
type ConversationReadState struct {
	LastReadMessageID string
	LastReadAt        time.Time
}

// MarkConversationReadAction 单调推进用户会话已读水位。
type MarkConversationReadAction struct{ db *bun.DB }

// NewMarkConversationReadAction 创建用户会话标记已读操作。
func NewMarkConversationReadAction(db *bun.DB) *MarkConversationReadAction {
	return &MarkConversationReadAction{db: db}
}

// Execute 校验原生会话访问权并保存不回退的消息水位。
func (a *MarkConversationReadAction) Execute(ctx context.Context, identity *servermodels.Identity, conversationID, messageID string) (ConversationReadState, error) {
	fields := make(map[string]ValidationCode)
	if !common.ValidUUID(conversationID) {
		fields["conversationId"] = ValidationConversationIDInvalid
	}
	if !common.ValidUUID(messageID) {
		fields["lastReadMessageId"] = ValidationLastReadMessageIDInvalid
	}
	if len(fields) > 0 {
		return ConversationReadState{}, &ValidationError{Fields: fields}
	}
	var result ConversationReadState
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := lockConversationMember(ctx, tx, identity, conversationID); err != nil {
			return err
		}
		var target servermodels.Message
		err := tx.NewSelect().Model(&target).
			Join("JOIN conversations AS cv ON cv.organization_id = msg.organization_id AND cv.id = msg.conversation_id").
			Join("JOIN conversation_participants AS cp ON cp.organization_id = cv.organization_id AND cp.conversation_id = cv.id AND cp.left_at IS NULL").
			Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ? AND cs.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
			Where("msg.organization_id = ?", identity.Organization.ID).
			Where("msg.conversation_id = ?", conversationID).
			Where("msg.id = ?", messageID).
			Where("cv.type IN (?, ?)", domain.ConversationTypeDirect, domain.ConversationTypeGroup).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConversationNotFound
		}
		if err != nil {
			return fmt.Errorf("load conversation read target: %w", err)
		}

		state := &servermodels.ConversationUserState{
			OrganizationID: identity.Organization.ID, ConversationID: conversationID,
			UserID: identity.User.ID, LastReadMessageID: &messageID,
		}
		if err := advanceConversationUserReadState(ctx, tx, state, &target); err != nil {
			return err
		}
		if err := tx.NewSelect().
			TableExpr("conversation_user_states AS cus").
			ColumnExpr("cus.last_read_message_id, cus.last_read_at").
			Where("cus.organization_id = ?", identity.Organization.ID).
			Where("cus.conversation_id = ?", conversationID).
			Where("cus.user_id = ?", identity.User.ID).
			Scan(ctx, &result); err != nil {
			return fmt.Errorf("load current conversation read state: %w", err)
		}
		return nil
	})
	if err != nil {
		return ConversationReadState{}, fmt.Errorf("mark conversation read: %w", err)
	}
	return result, nil
}

// advanceConversationUserReadState 按消息稳定顺序单调推进用户已读水位。
func advanceConversationUserReadState(ctx context.Context, db bun.IDB, state *servermodels.ConversationUserState, message *servermodels.Message) error {
	readAt := time.Now().UTC()
	state.LastReadAt = &readAt
	orderCondition := bun.SafeQuery("(current_message.originated_at, current_message.source_order, current_message.id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID)
	if message.GroupMessageSequence != nil {
		orderCondition = bun.SafeQuery("current_message.group_message_sequence < ?", *message.GroupMessageSequence)
	}
	if _, err := db.NewInsert().Model(state).
		Column("organization_id", "conversation_id", "user_id", "last_read_message_id", "last_read_at").
		On("CONFLICT (organization_id, conversation_id, user_id) DO UPDATE").
		Set("last_read_message_id = EXCLUDED.last_read_message_id").
		Set("last_read_at = now()").
		Set("updated_at = now()").
		Where(`cus.last_read_message_id IS NULL OR EXISTS (
			SELECT 1 FROM messages AS current_message
			WHERE current_message.organization_id = cus.organization_id
				AND current_message.conversation_id = cus.conversation_id
				AND current_message.id = cus.last_read_message_id AND ?
		)`, orderCondition).
		Exec(ctx); err != nil {
		return fmt.Errorf("advance conversation user read state: %w", err)
	}
	return nil
}
