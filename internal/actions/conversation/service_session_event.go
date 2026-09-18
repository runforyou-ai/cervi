//go:build server

package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// appendServiceSessionEvent 在调用方持有会话锁的事务中写入成员操作客服处理周期的系统事件，仅成员可见，不改变周期摘要与首响；fromIdentityID 为原负责人。
func appendServiceSessionEvent(ctx context.Context, db bun.IDB, identity *servermodels.Identity, conversation *servermodels.Conversation, session *servermodels.ServiceSession,
	eventType domain.ConversationSystemEventType, fromIdentityID *string, target *domain.ServiceSessionTarget) error {
	event := domain.ServiceSessionOperatedEvent{
		ServiceSessionID: session.ID, ActorIdentityID: identity.OrganizationIdentity.ID, ActorDisplayName: identity.OrganizationIdentity.DisplayName,
		FromIdentityID: fromIdentityID, Target: target,
	}
	if fromIdentityID != nil {
		var fromName string
		if err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).Column("display_name").
			Where("oi.organization_id = ? AND oi.id = ?", session.OrganizationID, *fromIdentityID).
			Scan(ctx, &fromName); err != nil {
			return fmt.Errorf("load service session previous assignee: %w", err)
		}
		event.FromDisplayName = &fromName
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode service session event: %w", err)
	}
	typeName := string(eventType)
	if _, _, err := chatstate.AppendMessage(ctx, db, conversation, &servermodels.Message{
		ID: uuid.NewV7().String(), OrganizationID: session.OrganizationID, ConversationID: session.ConversationID,
		ServiceSessionID: &session.ID, Type: string(domain.MessageTypeSystem), Visibility: string(domain.MessageVisibilityInternalOnly),
		SystemEventType: &typeName, SystemEventPayload: payload, OriginatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("append service session event: %w", err)
	}
	return nil
}
