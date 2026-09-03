//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// CustomerConversation 表示客户会话渠道关系。
type CustomerConversation struct {
	bun.BaseModel `bun:"table:customer_conversations,alias:cc"`

	ConversationID           string    `bun:"conversation_id,pk"`
	CreatedAt                time.Time `bun:"created_at"`
	UpdatedAt                time.Time `bun:"updated_at"`
	OrganizationID           string    `bun:"organization_id"`
	ContactChannelIdentityID string    `bun:"contact_channel_identity_id"`
	CurrentServiceSessionID  *string   `bun:"current_service_session_id"`
}
