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
	InitialRoutingTargetType  string    `bun:"initial_routing_target_type" json:"initialRoutingTargetType"`
	InitialRoutingTargetID    *string   `bun:"initial_routing_target_id" json:"initialRoutingTargetId"`
	FallbackRoutingTargetType string    `bun:"fallback_routing_target_type" json:"fallbackRoutingTargetType"`
	FallbackRoutingTargetID   *string   `bun:"fallback_routing_target_id" json:"fallbackRoutingTargetId"`
	Enabled                   bool      `bun:"enabled" json:"enabled"`
	CreatedAt                 time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt                 time.Time `bun:"updated_at" json:"updatedAt"`
}
