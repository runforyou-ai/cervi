//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Team 表示 PostgreSQL 中的企业团队。
type Team struct {
	bun.BaseModel `bun:"table:teams,alias:t"`

	ID              string    `bun:"id,pk" json:"id"`
	OrganizationID  string    `bun:"organization_id" json:"organizationId"`
	Name            string    `bun:"name" json:"name"`
	Description     string    `bun:"description" json:"description"`
	CreatedByUserID string    `bun:"created_by_user_id" json:"createdByUserId"`
	CreatedAt       time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `bun:"updated_at" json:"updatedAt"`
}
