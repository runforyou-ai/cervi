package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// KnowledgeBaseCategory 表示知识库内容类型。
type KnowledgeBaseCategory string

const (
	KnowledgeBaseCategoryStandard KnowledgeBaseCategory = KnowledgeBaseCategory(domain.KnowledgeBaseCategoryStandard)
	KnowledgeBaseCategoryQA       KnowledgeBaseCategory = KnowledgeBaseCategory(domain.KnowledgeBaseCategoryQA)
)

// KnowledgeBaseInput 定义知识库可编辑字段。
type KnowledgeBaseInput struct {
	EmbeddingProviderID      string                `json:"embeddingProviderId"`
	EmbeddingModelIdentifier string                `json:"embeddingModelIdentifier"`
	EmbeddingDimension       int                   `json:"embeddingDimension"`
	ChunkLength              *int                  `json:"chunkLength"`
	ChunkOverlap             *int                  `json:"chunkOverlap"`
	RetrievalCount           int                   `json:"retrievalCount"`
	RerankProviderID         string                `json:"rerankProviderId"`
	RerankModelIdentifier    string                `json:"rerankModelIdentifier"`
	Name                     string                `json:"name"`
	Category                 KnowledgeBaseCategory `json:"category"`
	Description              string                `json:"description"`
}

// KnowledgeGroupInput 定义知识库分组可编辑字段。
type KnowledgeGroupInput struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
}

// KnowledgeGroup 定义知识库分组树节点。
type KnowledgeGroup struct {
	ID        string           `json:"id"`
	ParentID  string           `json:"parentId"`
	Name      string           `json:"name"`
	IsDefault bool             `json:"isDefault"`
	Children  []KnowledgeGroup `json:"children"`
}

// KnowledgeBase 定义知识库详情。
type KnowledgeBase struct {
	EmbeddingProviderID      string                `json:"embeddingProviderId"`
	EmbeddingModelIdentifier string                `json:"embeddingModelIdentifier"`
	EmbeddingDimension       int                   `json:"embeddingDimension"`
	ChunkLength              *int                  `json:"chunkLength"`
	ChunkOverlap             *int                  `json:"chunkOverlap"`
	RetrievalCount           int                   `json:"retrievalCount"`
	RerankProviderID         string                `json:"rerankProviderId"`
	RerankModelIdentifier    string                `json:"rerankModelIdentifier"`
	ID                       string                `json:"id"`
	Name                     string                `json:"name"`
	Category                 KnowledgeBaseCategory `json:"category"`
	Description              string                `json:"description"`
	Groups                   []KnowledgeGroup      `json:"groups"`
	CreatedAt                time.Time             `json:"createdAt"`
	UpdatedAt                time.Time             `json:"updatedAt"`
}

// KnowledgeBaseList 定义知识库列表。
type KnowledgeBaseList struct {
	KnowledgeBases []KnowledgeBase `json:"knowledgeBases"`
}

// KnowledgeRetrievalInput 定义检索测试的查询内容。
type KnowledgeRetrievalInput struct {
	Query string `json:"query"`
}

// KnowledgeRetrievalRecord 定义检索测试命中的分段、两路名次、融合分数和重排得分；名次为 0 表示该路未命中，未重排时 RerankScore 为空。
type KnowledgeRetrievalRecord struct {
	DocumentID     string   `json:"documentId"`
	DocumentName   string   `json:"documentName"`
	SegmentID      string   `json:"segmentId"`
	SegmentBatchID string   `json:"segmentBatchId"`
	Position       int      `json:"position"`
	Content        string   `json:"content"`
	Score          float64  `json:"score"`
	RerankScore    *float64 `json:"rerankScore"`
	LexicalRank    int      `json:"lexicalRank"`
	VectorRank     int      `json:"vectorRank"`
}

// KnowledgeRetrievalResult 定义检索测试结果。
type KnowledgeRetrievalResult struct {
	Records []KnowledgeRetrievalRecord `json:"records"`
}
