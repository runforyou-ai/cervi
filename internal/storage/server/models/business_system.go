//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// BusinessSystem 表示 PostgreSQL 中的企业业务系统。
type BusinessSystem struct {
	bun.BaseModel `bun:"table:business_systems,alias:bs"`

	ID             string    `bun:"id,pk"`
	OrganizationID string    `bun:"organization_id"`
	Name           string    `bun:"name"`
	URL            string    `bun:"url"`
	Enabled        bool      `bun:"enabled"`
	CreatedAt      time.Time `bun:"created_at"`
	UpdatedAt      time.Time `bun:"updated_at"`
}
