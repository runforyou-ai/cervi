// Package knowledgeprocessing 定义知识文档处理服务的内部传输契约。
package knowledgeprocessing

import "github.com/runforyou-ai/cervi/internal/domain"

// ProcessInput 固定本次文档任务的来源、分段和向量参数。
type ProcessInput struct {
	OrganizationID           string `json:"organizationId"`
	KnowledgeBaseID          string `json:"knowledgeBaseId"`
	DocumentID               string `json:"documentId"`
	ProcessingID             string `json:"processingId"`
	ChunkLength              int    `json:"chunkLength"`
	ChunkOverlap             int    `json:"chunkOverlap"`
	EmbeddingProviderID      string `json:"embeddingProviderId"`
	EmbeddingModelIdentifier string `json:"embeddingModelIdentifier"`
	EmbeddingDimension       int    `json:"embeddingDimension"`
}

// EmbeddingCredential 提供访问向量模型所需的凭据，只在执行任务时传递。
type EmbeddingCredential struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
}

// ProcessResult 返回实际持久化的分段数量。
type ProcessResult struct {
	SegmentCount int  `json:"segmentCount"`
	Stale        bool `json:"stale"`
}

// Error 定义文档处理的失败原因码和执行阶段。
type Error struct {
	Code  string                         `json:"code"`
	Stage domain.KnowledgeDocumentStatus `json:"stage"`
}

// Error 返回语言无关的失败原因。
func (e *Error) Error() string { return "knowledge processing: " + e.Code }
