//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Contact 表示企业的外部联系人。
type Contact struct {
	bun.BaseModel `bun:"table:contacts,alias:c"`

	ID              string     `bun:"id,pk" json:"id"`
	OrganizationID  string     `bun:"organization_id" json:"organizationId"`
	CreatedByUserID string     `bun:"created_by_user_id" json:"createdByUserId"`
	SourceChannelID string     `bun:"source_channel_id" json:"sourceChannelId"`
	DisplayName     *string    `bun:"display_name" json:"displayName"`
	Stage           string     `bun:"stage" json:"stage"`
	Notes           *string    `bun:"notes" json:"notes"`
	CreatedAt       time.Time  `bun:"created_at" json:"createdAt"`
	UpdatedAt       time.Time  `bun:"updated_at" json:"updatedAt"`
	DeletedAt       *time.Time `bun:"deleted_at" json:"deletedAt"`
}
