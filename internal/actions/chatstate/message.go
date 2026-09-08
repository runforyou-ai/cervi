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
	// 幂等重放返回既有消息，不再次分配序号或更新摘要。
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
	// 事务回滚同时撤销未提交序号。
	if err := db.NewUpdate().Model(conversation).
		Set("last_message_seq = last_message_seq + 1").
		WherePK().Where("organization_id = ?", conversation.OrganizationID).
		Returning("last_message_seq").Scan(ctx, &message.MessageSeq); err != nil {
		return nil, false, fmt.Errorf("allocate message sequence: %w", err)
	}
	conversation.LastMessageSeq = message.MessageSeq
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "system_event_type", "system_event_payload", "reply_to_message_id", "mention_all", "thread_root_message_id", "idempotency_key", "originated_at", "source_order", "message_seq").
		Returning("*").Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("append conversation message: %w", err)
	}
	if message.ServiceSessionID != nil {
		if _, err := db.NewUpdate().Model((*servermodels.ServiceSession)(nil)).
			Set("last_message_id = ?", message.ID).
			Set("last_message_at = ?", message.OriginatedAt).
			Set("updated_at = now()").
			Where("organization_id = ? AND conversation_id = ? AND id = ?", conversation.OrganizationID, conversation.ID, *message.ServiceSessionID).
			Where("status = ?", domain.ServiceSessionStatusOpen).
			Exec(ctx); err != nil {
			return nil, false, fmt.Errorf("update service session summary: %w", err)
		}
	}
	query := db.NewUpdate().Model(conversation).
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", conversation.OrganizationID)
	if _, err := query.Exec(ctx); err != nil {
		return nil, false, fmt.Errorf("update conversation summary: %w", err)
	}
	return message, true, nil
}

// RecomputeConversationSummary 在调用方事务和会话锁内撤去末条消息摘要，保留当前最后一条可见消息。
func RecomputeConversationSummary(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, removedMessageID string) error {
	_, err := db.NewRaw(`UPDATE conversations SET (last_message_id, last_message_at) =
 (SELECT latest.id, latest.originated_at FROM (SELECT 1) AS anchor LEFT JOIN LATERAL
 (SELECT id, originated_at FROM messages WHERE organization_id = ? AND conversation_id = ? AND deleted_at IS NULL ORDER BY message_seq DESC LIMIT 1) latest ON true), updated_at = now()
 WHERE organization_id = ? AND id = ? AND last_message_id = ?`, conversation.OrganizationID, conversation.ID, conversation.OrganizationID, conversation.ID, removedMessageID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("recompute conversation summary: %w", err)
	}
	return nil
}
