//go:build server

package knowledgebase

import (
	"errors"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

var (
	ErrDocumentNotFound     = errors.New("knowledge document not found")
	ErrDocumentUnsupported  = errors.New("knowledge document unsupported")
	ErrDocumentBatchInvalid = errors.New("knowledge document batch must contain 1 to 10 distinct files")
)

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

// ProcessError 定义文档处理的失败原因码和执行阶段。
type ProcessError struct {
	Code  string
	Stage domain.KnowledgeDocumentStatus
}

// Error 返回语言无关的失败原因。
func (e *ProcessError) Error() string { return "knowledge processing: " + e.Code }

// DocumentRecord 汇总文档归属与原件元数据。
type DocumentRecord struct {
	ID             string                         `bun:"id"`
	GroupID        string                         `bun:"group_id"`
	Name           string                         `bun:"name"`
	ContentType    string                         `bun:"content_type"`
	ByteSize       int64                          `bun:"byte_size"`
	Status         domain.KnowledgeDocumentStatus `bun:"status"`
	SegmentBatchID string                         `bun:"segment_batch_id"`
	SegmentCount   int                            `bun:"segment_count"`
	FailureCode    string                         `bun:"failure_code"`
	CreatedAt      time.Time                      `bun:"created_at"`
}

// DocumentListInput 定义分组文档的分页查询条件。
type DocumentListInput struct {
	GroupID, Keyword string
	Page, PageSize   int
}

// DocumentListOutput 返回文档分页结果。
type DocumentListOutput struct {
	Documents             []DocumentRecord
	Page, PageSize, Total int
}
