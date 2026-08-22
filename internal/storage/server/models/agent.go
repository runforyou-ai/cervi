//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Agent 表示 PostgreSQL 中的 AI 员工子实体。
type Agent struct {
	bun.BaseModel `bun:"table:agents,alias:a"`

	ID             string    `bun:"id,pk" json:"id"`
	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bun:"updated_at" json:"updatedAt"`
}
