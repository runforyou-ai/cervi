//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Message 表示会话消息。
type Message struct {
	bun.BaseModel `bun:"table:messages,alias:msg"`

	ID                  string     `bun:"id,pk"`
	CreatedAt           time.Time  `bun:"created_at"`
	UpdatedAt           time.Time  `bun:"updated_at"`
	OrganizationID      string     `bun:"organization_id"`
	ConversationID      string     `bun:"conversation_id"`
	ServiceSessionID    *string    `bun:"service_session_id"`
	SenderParticipantID *string    `bun:"sender_participant_id"`
	Type                string     `bun:"type"`
	Body                string     `bun:"body"`
	ReplyToMessageID    *string    `bun:"reply_to_message_id"`
	ThreadRootMessageID *string    `bun:"thread_root_message_id"`
	IdempotencyKey      *string    `bun:"idempotency_key"`
	OriginatedAt        time.Time  `bun:"originated_at"`
	EditedAt            *time.Time `bun:"edited_at"`
	DeletedAt           *time.Time `bun:"deleted_at"`
}
