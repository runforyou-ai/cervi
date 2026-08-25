//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// KnowledgeBase 表示 PostgreSQL 中的企业知识库。
type KnowledgeBase struct {
	bun.BaseModel `bun:"table:knowledge_bases,alias:kb"`

	ID                      string    `bun:"id,pk" json:"id"`
	OrganizationID          string    `bun:"organization_id" json:"organizationId"`
	CreatedByUserID         string    `bun:"created_by_user_id" json:"createdByUserId"`
	Name                    string    `bun:"name" json:"name"`
	Category                string    `bun:"category" json:"category"`
	Description             string    `bun:"description" json:"description"`
	IntegrationConnectionID *string   `bun:"integration_connection_id" json:"integrationConnectionId"`
	ExternalResourceID      *string   `bun:"external_resource_id" json:"externalResourceId"`
	CreatedAt               time.Time `bun:"created_at" json:"createdAt"`
	UpdatedAt               time.Time `bun:"updated_at" json:"updatedAt"`
}
