//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// AppendMessage 在调用方事务和会话锁内追加消息、推进会话版本、维护摘要并登记会话受众通知；调用方负责授权及完整发送意图校验。
func AppendMessage(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation, message *servermodels.Message) (*servermodels.Message, bool, error) {
	// 幂等重放返回既有消息并保留序号和摘要。
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
	// 序号与会话版本同句推进。
	if err := db.NewUpdate().Model(conversation).
		Set("last_message_seq = last_message_seq + 1").
		Set("version = version + 1").
		WherePK().Where("organization_id = ?", conversation.OrganizationID).
		Returning("last_message_seq, version").Scan(ctx); err != nil {
		return nil, false, fmt.Errorf("allocate message sequence: %w", err)
	}
	message.MessageSeq = conversation.LastMessageSeq
	// 文本消息按正文生成检索词元；附件消息由调用方按说明和文件名生成。
	if message.Type == string(domain.MessageTypeText) {
		message.SearchVector = searchtext.Vector(message.Body)
	}
	if _, err := db.NewInsert().Model(message).
		Column("id", "organization_id", "conversation_id", "service_session_id", "sender_participant_id", "type", "body", "search_vector", "system_event_type", "system_event_payload", "reply_to_message_id", "mention_all", "thread_root_message_id", "idempotency_key", "client_message_id", "originated_at", "source_order", "message_seq").
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
	// 活动时间取锁内数据库时钟，保留同会话已提交的较大值。
	query := db.NewUpdate().Model(conversation).
		Set("last_activity_at = GREATEST(last_activity_at, clock_timestamp())").
		Set("last_message_id = ?", message.ID).
		Set("last_message_at = ?", message.OriginatedAt).
		Set("updated_at = now()").
		WherePK().Where("organization_id = ?", conversation.OrganizationID)
	if err := query.Returning("last_activity_at").Scan(ctx); err != nil {
		return nil, false, fmt.Errorf("update conversation summary: %w", err)
	}
	if err := NotifyConversationChanged(ctx, db, conversation); err != nil {
		return nil, false, err
	}
	return message, true, nil
}

// TouchConversation 在调用方持有会话锁的事务内推进会话版本，并登记会话受众的变更通知。
func TouchConversation(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation) error {
	if err := db.NewUpdate().Model(conversation).
		Set("version = version + 1").
		WherePK().Where("organization_id = ?", conversation.OrganizationID).
		Returning("version").Scan(ctx); err != nil {
		return fmt.Errorf("advance conversation version: %w", err)
	}
	return NotifyConversationChanged(ctx, db, conversation)
}

// NotifyConversationChanged 按会话当前版本登记变更通知：客户会话通知企业客服共享受众，内部会话通知当前真人成员。
func NotifyConversationChanged(ctx context.Context, db bun.IDB, conversation *servermodels.Conversation) error {
	if conversation.Type == string(domain.ConversationTypeCustomer) {
		realtime.Notify(ctx, realtime.CustomerInboxConversationChanged(conversation.OrganizationID, conversation.ID, conversation.Version))
		return nil
	}
	var userIDs []string
	if err := db.NewSelect().TableExpr("conversation_participants AS cp").
		Join("JOIN chat_subjects AS cs ON cs.organization_id = cp.organization_id AND cs.id = cp.subject_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN users AS u ON u.organization_id = cs.organization_id AND u.identity_id = cs.source_id").
		Column("u.id").
		Where("cp.organization_id = ? AND cp.conversation_id = ? AND cp.left_at IS NULL", conversation.OrganizationID, conversation.ID).
		Scan(ctx, &userIDs); err != nil {
		return fmt.Errorf("load conversation notification audience: %w", err)
	}
	for _, userID := range userIDs {
		realtime.Notify(ctx, realtime.UserConversationChanged(conversation.OrganizationID, userID, conversation.ID, conversation.Version))
	}
	return nil
}
