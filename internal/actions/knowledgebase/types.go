//go:build server

package knowledgebase

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// Input 定义知识库可编辑字段。
type Input struct {
	Name                    string
	Category                domain.KnowledgeBaseCategory
	Description             string
	IntegrationConnectionID string
	ExternalResourceID      string
}

// GroupInput 定义知识库分组可编辑字段。
type GroupInput struct {
	Name     string
	ParentID string
}

// Record 定义知识库详情字段。
type Record struct {
	ID                      string                       `bun:"id"`
	Name                    string                       `bun:"name"`
	Category                domain.KnowledgeBaseCategory `bun:"category"`
	Description             string                       `bun:"description"`
	IntegrationConnectionID string
	ExternalResourceID      string
	Groups                  []GroupRecord
	CreatedAt               time.Time `bun:"created_at"`
	UpdatedAt               time.Time `bun:"updated_at"`
}

// GroupRecord 定义知识库分组树节点。
type GroupRecord struct {
	ID        string  `bun:"id"`
	ParentID  *string `bun:"parent_id"`
	Name      string  `bun:"name"`
	IsDefault bool    `bun:"is_default"`
	SortOrder int     `bun:"sort_order"`
	Children  []GroupRecord
}
