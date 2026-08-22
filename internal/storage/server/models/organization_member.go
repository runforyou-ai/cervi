//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// OrganizationMember 表示 PostgreSQL 中统一的企业成员身份。
type OrganizationMember struct {
	bun.BaseModel `bun:"table:organization_members,alias:om"`

	ID             string    `bun:"id,pk" json:"id"`
	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	Type           string    `bun:"type" json:"type"`
	DisplayName    string    `bun:"display_name" json:"displayName"`
	AvatarFileID   *string   `bun:"avatar_file_id" json:"avatarFileId"`
	Status         string    `bun:"status" json:"status"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bun:"updated_at" json:"updatedAt"`
}
