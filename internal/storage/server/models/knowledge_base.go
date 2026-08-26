//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// KnowledgeBase 表示 PostgreSQL 中的企业知识库。
type KnowledgeBase struct {
	bun.BaseModel `bun:"table:knowledge_bases,alias:kb"`

	ID                      string    `bun:"id,pk"`
	OrganizationID          string    `bun:"organization_id"`
	CreatedByUserID         string    `bun:"created_by_user_id"`
	Name                    string    `bun:"name"`
	Category                string    `bun:"category"`
	Description             string    `bun:"description"`
	IntegrationConnectionID *string   `bun:"integration_connection_id"`
	ExternalResourceID      *string   `bun:"external_resource_id"`
	CreatedAt               time.Time `bun:"created_at"`
	UpdatedAt               time.Time `bun:"updated_at"`
}
