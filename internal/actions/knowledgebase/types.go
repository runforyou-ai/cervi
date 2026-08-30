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

// ExternalOptionRecord 定义外部知识库选择项。
type ExternalOptionRecord struct {
	ID       string
	Name     string
	Category domain.KnowledgeBaseCategory
}

// DocumentListInput 定义知识文档列表查询条件。
type DocumentListInput struct {
	Keyword  string
	Status   domain.KnowledgeDocumentStatus
	Page     int
	PageSize int
}

// DocumentRecord 定义知识文档列表项。
type DocumentRecord struct {
	ID        string
	Name      string
	Status    domain.KnowledgeDocumentStatus
	CreatedAt *time.Time
}

// DocumentListOutput 定义知识文档分页结果。
type DocumentListOutput struct {
	Documents []DocumentRecord
	Page      int
	PageSize  int
	Total     int
}

// DocumentDetailRecord 定义知识文档详情。
type DocumentDetailRecord struct {
	ID        string
	Name      string
	Status    domain.KnowledgeDocumentStatus
	WordCount *int
	HitCount  int
	CreatedAt *time.Time
}

// DocumentSegmentListInput 定义知识文档分段列表查询条件。
type DocumentSegmentListInput struct {
	Keyword  string
	Status   domain.KnowledgeDocumentSegmentIndexStatus
	Page     int
	PageSize int
}

// DocumentSegmentRecord 定义知识文档分段列表项。
type DocumentSegmentRecord struct {
	ID          string
	Position    int
	Content     string
	Answer      *string
	WordCount   int
	HitCount    int
	IndexStatus domain.KnowledgeDocumentSegmentIndexStatus
	CreatedAt   *time.Time
}

// DocumentSegmentListOutput 定义知识文档分段分页结果。
type DocumentSegmentListOutput struct {
	Segments []DocumentSegmentRecord
	Page     int
	PageSize int
	Total    int
}

// RetrievalInput 定义知识库检索条件。
type RetrievalInput struct {
	Query                 string
	Method                domain.KnowledgeRetrievalMethod
	RerankingEnabled      bool
	TopK                  int
	ScoreThresholdEnabled bool
	ScoreThreshold        float64
}

// RetrievalRecord 定义知识库检索命中项。
type RetrievalRecord struct {
	DocumentID   string
	DocumentName string
	SegmentID    string
	Position     int
	Content      string
	Answer       *string
	Score        *float64
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
