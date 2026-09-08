//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// AppendMessage 在调用方事务和会话锁内追加消息并维护摘要；调用方负责授权及完整发送意图校验。
func AppendMessage(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, message *servermodels.Message) (*servermodels.Message, bool, error) {
	if message.IdempotencyKey != nil {
		existing := &servermodels.Message{}
		err := db.NewSelect().Model(existing).
			Where("msg.organization_id = ? AND msg.conversation_id = ? AND msg.idempotency_key = ?", conversation.OrganizationID, conversation.ID, *message.IdempotencyKey).
			Scan(ctx)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, fmt.Errorf("load appended message: %w", err)
		}
	}
	// 群聊继续使用现有分配器，幂等重放不消耗序号。
	if conversation.Type == string(domain.ConversationTypeGroup) {
		var sequence int64
		if err := db.NewUpdate().Model(conversation).
			Set("last_group_message_sequence = last_group_message_sequence + 1").
			WherePK().Where("organization_id = ?", conversation.OrganizationID).
			Returning("last_group_message_sequence").Scan(ctx, &sequence); err != nil {
			return nil, false, fmt.Errorf("allocate group message sequence: %w", err)
		}
		message.GroupMessageSequence = &sequence
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "system_event_type", "system_event_payload", "reply_to_message_id", "mention_all", "thread_root_message_id", "idempotency_key", "originated_at", "source_order", "group_message_sequence").
		Returning("*").Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("append conversation message: %w", err)
	}
	if message.ServiceSessionID != nil {
		if _, err := db.NewUpdate().Model((*servermodels.ServiceSession)(nil)).
			Set("last_message_id = ?", message.ID).
			Set("last_message_at = ?", message.OriginatedAt).
			Set("last_message_source_order = ?", message.SourceOrder).
			Set("updated_at = now()").
			Where("organization_id = ? AND conversation_id = ? AND id = ?", conversation.OrganizationID, conversation.ID, *message.ServiceSessionID).
			Where("status = ?", domain.ServiceSessionStatusOpen).
			Where("(last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID).
			Exec(ctx); err != nil {
			return nil, false, fmt.Errorf("update service session summary: %w", err)
		}
	}
	query := db.NewUpdate().Model(conversation).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("last_message_source_order = ?", message.SourceOrder).
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", conversation.OrganizationID)
	if message.GroupMessageSequence != nil {
		query = query.Where("last_group_message_sequence = ?", *message.GroupMessageSequence)
	} else {
		query = query.Where("last_message_at IS NULL OR (last_message_at, last_message_source_order, last_message_id) < (?, ?, ?)", message.OriginatedAt, message.SourceOrder, message.ID)
	}
	if _, err := query.Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("update conversation summary: %w", err)
	}
	return message, true, nil
}

// RecomputeConversationSummary 在调用方事务和会话锁内撤去末条消息摘要，保留当前最后一条可见消息。
func RecomputeConversationSummary(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, removedMessageID string) error {
	_, err := db.NewRaw(`UPDATE conversations SET (last_message_id, last_message_at, last_message_source_order) =
 (SELECT latest.id, latest.originated_at, COALESCE(latest.source_order, 0) FROM (SELECT 1) AS anchor LEFT JOIN LATERAL
 (SELECT id, originated_at, source_order FROM messages WHERE organization_id = ? AND conversation_id = ? AND deleted_at IS NULL ORDER BY originated_at DESC, source_order DESC, id DESC LIMIT 1) latest ON true), updated_at = now()
 WHERE organization_id = ? AND id = ? AND last_message_id = ?`, conversation.OrganizationID, conversation.ID, conversation.OrganizationID, conversation.ID, removedMessageID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("recompute conversation summary: %w", err)
	}
	return nil
}
