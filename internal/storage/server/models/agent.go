//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Agent 表示 PostgreSQL 中的 AI 员工。
type Agent struct {
	bun.BaseModel `bun:"table:agents,alias:a"`

	ID               string    `bun:"id,pk" json:"id"`
	IdentityID       string    `bun:"identity_id" json:"identityId"`
	OrganizationID   string    `bun:"organization_id" json:"organizationId"`
	ActiveRevisionID string    `bun:"active_revision_id" json:"activeRevisionId"`
	Status           string    `bun:"status" json:"status"`
	CreatedAt        time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt        time.Time `bun:"updated_at" json:"updatedAt"`
}
