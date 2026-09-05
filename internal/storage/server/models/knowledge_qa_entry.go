//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// KnowledgeQAEntry 表示本地知识库中的一条完整问答。
type KnowledgeQAEntry struct {
	bun.BaseModel   `bun:"table:knowledge_qa_entries,alias:kqe"`
	ID              string    `bun:"id,pk"`
	KnowledgeBaseID string    `bun:"knowledge_base_id"`
	GroupID         string    `bun:"group_id"`
	CreatedByUserID string    `bun:"created_by_user_id"`
	CreatedAt       time.Time `bun:"created_at"`
	UpdatedAt       time.Time `bun:"updated_at"`
}
