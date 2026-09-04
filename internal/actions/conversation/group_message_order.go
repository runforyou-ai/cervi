//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// lockConversationMember 锁定成员所属的内部会话，并在等待锁后重新校验成员关系。
func lockConversationMember(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversationID string) (*servermodels.Conversation, error) {
	conversation := &servermodels.Conversation{}
	err := db.NewSelect().Model(conversation).
		Join("JOIN conversation_participants AS mine ON mine.organization_id = cv.organization_id AND mine.conversation_id = cv.id AND mine.left_at IS NULL").
		Join("JOIN chat_subjects AS subject ON subject.organization_id = mine.organization_id AND subject.id = mine.subject_id AND subject.kind = ? AND subject.source_id = ?", domain.ChatSubjectKindOrganizationIdentity, identity.OrganizationIdentity.ID).
		Where("cv.organization_id = ? AND cv.id = ?", identity.Organization.ID, conversationID).
		Where("cv.type IN (?, ?)", domain.ConversationTypeDirect, domain.ConversationTypeGroup).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
		For("UPDATE OF cv").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock member conversation: %w", err)
	}
	if err := authorizeConversationHistory(ctx, db, identity, conversationID); err != nil {
		return nil, err
	}
	return conversation, nil
}

// nextGroupMessageSequence 在已锁定群聊的事务中分配下一条消息序号。
func nextGroupMessageSequence(ctx context.Context, db bun.IDB, organizationID, conversationID string) (int64, error) {
	var sequence int64
	err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
		Set("message_sequence = message_sequence + 1").
		Where("organization_id = ? AND id = ? AND type = ?", organizationID, conversationID, domain.ConversationTypeGroup).
		Returning("message_sequence").Scan(ctx, &sequence)
	if err != nil {
		return 0, fmt.Errorf("allocate group message sequence: %w", err)
	}
	return sequence, nil
}
