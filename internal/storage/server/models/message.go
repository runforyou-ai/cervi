//go:build server

package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// Message 表示会话消息。
type Message struct {
	bun.BaseModel `bun:"table:messages,alias:msg"`

	ID                  string          `bun:"id,pk"`
	CreatedAt           time.Time       `bun:"created_at"`
	UpdatedAt           time.Time       `bun:"updated_at"`
	OrganizationID      string          `bun:"organization_id"`
	ConversationID      string          `bun:"conversation_id"`
	ServiceSessionID    *string         `bun:"service_session_id"`
	SenderParticipantID *string         `bun:"sender_participant_id"`
	Type                string          `bun:"type"`
	Body                string          `bun:"body"`
	SystemEventType     *string         `bun:"system_event_type"`
	SystemEventPayload  json.RawMessage `bun:"system_event_payload,type:jsonb"`
	ReplyToMessageID    *string         `bun:"reply_to_message_id"`
	ThreadRootMessageID *string         `bun:"thread_root_message_id"`
	IdempotencyKey      *string         `bun:"idempotency_key"`
	OriginatedAt        time.Time       `bun:"originated_at"`
	SourceOrder         int64           `bun:"source_order"`
	EditedAt            *time.Time      `bun:"edited_at"`
	DeletedAt           *time.Time      `bun:"deleted_at"`
}
