//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// OrganizationIdentity 表示 PostgreSQL 中的企业身份。
type OrganizationIdentity struct {
	bun.BaseModel `bun:"table:organization_identities,alias:oi"`

	ID                  string    `bun:"id,pk" json:"id"`
	OrganizationID      string    `bun:"organization_id" json:"organizationId"`
	Type                string    `bun:"type" json:"type"`
	DisplayName         string    `bun:"display_name" json:"displayName"`
	AvatarFileID        *string   `bun:"avatar_file_id" json:"avatarFileId"`
	WorkStatus          string    `bun:"work_status" json:"workStatus"`
	WorkStatusUpdatedAt time.Time `bun:"work_status_updated_at" json:"workStatusUpdatedAt"`
	CreatedAt           time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt           time.Time `bun:"updated_at" json:"updatedAt"`
}
