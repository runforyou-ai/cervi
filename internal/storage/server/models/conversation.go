//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Conversation 表示聊天会话。
type Conversation struct {
	bun.BaseModel `bun:"table:conversations,alias:cv"`

	ID                     string     `bun:"id,pk"`
	CreatedAt              time.Time  `bun:"created_at"`
	UpdatedAt              time.Time  `bun:"updated_at"`
	OrganizationID         string     `bun:"organization_id"`
	Type                   string     `bun:"type"`
	Status                 string     `bun:"status"`
	Title                  *string    `bun:"title"`
	Description            *string    `bun:"description"`
	ImageFileID            *string    `bun:"image_file_id"`
	CreatedBySubjectID     *string    `bun:"created_by_subject_id"`
	LastMessageID          *string    `bun:"last_message_id"`
	LastMessageAt          *time.Time `bun:"last_message_at"`
	LastMessageSourceOrder int64      `bun:"last_message_source_order"`
}
