//go:build server

package conversation

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// nextGroupMessageSequence 在已锁定群聊的事务中分配下一条消息序号。
func nextGroupMessageSequence(ctx context.Context, db bun.IDB, organizationID, conversationID string) (int64, error) {
	var sequence int64
	err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
		Set("last_group_message_sequence = last_group_message_sequence + 1").
		Where("organization_id = ? AND id = ? AND type = ?", organizationID, conversationID, domain.ConversationTypeGroup).
		Returning("last_group_message_sequence").Scan(ctx, &sequence)
	if err != nil {
		return 0, fmt.Errorf("allocate group message sequence: %w", err)
	}
	return sequence, nil
}
