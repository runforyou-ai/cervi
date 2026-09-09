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
