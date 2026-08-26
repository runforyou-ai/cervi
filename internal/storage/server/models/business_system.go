//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// BusinessSystem 表示 PostgreSQL 中的企业业务系统。
type BusinessSystem struct {
	bun.BaseModel `bun:"table:business_systems,alias:bs"`

	ID             string    `bun:"id,pk" json:"id"`
	OrganizationID string    `bun:"organization_id" json:"organizationId"`
	Name           string    `bun:"name" json:"name"`
	URL            string    `bun:"url" json:"url"`
	Enabled        bool      `bun:"enabled" json:"enabled"`
	CreatedAt      time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bun:"updated_at" json:"updatedAt"`
}
