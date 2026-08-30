//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ServiceSession 表示客户会话客服处理周期。
type ServiceSession struct {
	bun.BaseModel `bun:"table:service_sessions,alias:ss"`

	ID                       string     `bun:"id,pk"`
	CreatedAt                time.Time  `bun:"created_at"`
	UpdatedAt                time.Time  `bun:"updated_at"`
	OrganizationID           string     `bun:"organization_id"`
	ConversationID           string     `bun:"conversation_id"`
	ContactChannelIdentityID string     `bun:"contact_channel_identity_id"`
	Sequence                 int64      `bun:"sequence"`
	Status                   string     `bun:"status"`
	TeamID                   *string    `bun:"team_id"`
	AssigneeIdentityID       *string    `bun:"assignee_identity_id"`
	OpeningMessageID         string     `bun:"opening_message_id"`
	LastMessageID            string     `bun:"last_message_id"`
	LastMessageAt            time.Time  `bun:"last_message_at"`
	LastMessageSourceOrder   int64      `bun:"last_message_source_order"`
	AssignedAt               *time.Time `bun:"assigned_at"`
	FirstResponseAt          *time.Time `bun:"first_response_at"`
	StatusChangedAt          time.Time  `bun:"status_changed_at"`
	ClosedAt                 *time.Time `bun:"closed_at"`
	ClosedByIdentityID       *string    `bun:"closed_by_identity_id"`
}
