//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Channel 表示企业接收客户消息的渠道。
type Channel struct {
	bun.BaseModel `bun:"table:channels,alias:c"`

	ID                        string    `bun:"id,pk" json:"id"`
	OrganizationID            string    `bun:"organization_id" json:"organizationId"`
	CreatedByUserID           string    `bun:"created_by_user_id" json:"createdByUserId"`
	Type                      string    `bun:"type" json:"type"`
	Name                      string    `bun:"name" json:"name"`
	Description               *string   `bun:"description" json:"description"`
	DefaultLocale             string    `bun:"default_locale" json:"defaultLocale"`
	NewConversationTargetType string    `bun:"new_conversation_target_type" json:"newConversationTargetType"`
	NewConversationTargetID   *string   `bun:"new_conversation_target_id" json:"newConversationTargetId"`
	FallbackTargetType        string    `bun:"fallback_target_type" json:"fallbackTargetType"`
	FallbackTargetID          *string   `bun:"fallback_target_id" json:"fallbackTargetId"`
	Enabled                   bool      `bun:"enabled" json:"enabled"`
	CreatedAt                 time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt                 time.Time `bun:"updated_at" json:"updatedAt"`
}
