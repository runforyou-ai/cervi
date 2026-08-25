//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// KnowledgeGroup 表示 PostgreSQL 中的知识库分组。
type KnowledgeGroup struct {
	bun.BaseModel `bun:"table:knowledge_groups,alias:kg"`

	ID              string    `bun:"id,pk"`
	KnowledgeBaseID string    `bun:"knowledge_base_id"`
	ParentID        *string   `bun:"parent_id"`
	Name            string    `bun:"name"`
	IsDefault       bool      `bun:"is_default"`
	SortOrder       int       `bun:"sort_order"`
	CreatedAt       time.Time `bun:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at"`
}
