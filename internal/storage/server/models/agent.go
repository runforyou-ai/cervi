//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Agent 表示 PostgreSQL 中的 AI 员工。
type Agent struct {
	bun.BaseModel `bun:"table:agents,alias:a"`

	ID               string    `bun:"id,pk"`
	IdentityID       string    `bun:"identity_id"`
	OrganizationID   string    `bun:"organization_id"`
	ActiveRevisionID string    `bun:"active_revision_id"`
	Status           string    `bun:"status"`
	CreatedAt        time.Time `bun:"created_at"`
	UpdatedAt        time.Time `bun:"updated_at"`
}
