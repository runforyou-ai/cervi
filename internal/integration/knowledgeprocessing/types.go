// Package knowledgeprocessing 定义知识文档处理服务的内部传输契约。
package knowledgeprocessing

import "github.com/runforyou-ai/cervi/internal/domain"

// ProcessInput 固定本次文档任务的来源和分段参数。
type ProcessInput struct {
	OrganizationID  string `json:"organizationId"`
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	DocumentID      string `json:"documentId"`
	ProcessingID    string `json:"processingId"`
	ChunkLength     int    `json:"chunkLength"`
	ChunkOverlap    int    `json:"chunkOverlap"`
}

// ProcessResult 返回实际持久化的分段数量。
type ProcessResult struct {
	SegmentCount int  `json:"segmentCount"`
	Stale        bool `json:"stale"`
}

// Segment 定义按来源顺序阅读的一段正文。
type Segment struct {
	ID             string `json:"id"`
	Position       int    `json:"position"`
	Content        string `json:"content"`
	CharacterCount int    `json:"characterCount"`
	PageNumber     *int   `json:"pageNumber"`
	SourceLabel    string `json:"sourceLabel"`
}

// ListInput 定义同一批次中的分页或锚点查询。
type ListInput struct {
	OrganizationID  string `json:"organizationId"`
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	DocumentID      string `json:"documentId"`
	SegmentBatchID  string `json:"segmentBatchId"`
	AnchorSegmentID string `json:"anchorSegmentId,omitempty"`
	Page            int    `json:"page"`
	PageSize        int    `json:"pageSize"`
}

// SegmentPage 返回目标页和经服务端核验的锚点。
type SegmentPage struct {
	SegmentBatchID  string    `json:"segmentBatchId"`
	Segments        []Segment `json:"segments"`
	Page            int       `json:"page"`
	PageSize        int       `json:"pageSize"`
	Total           int       `json:"total"`
	AnchorSegmentID string    `json:"anchorSegmentId"`
	AnchorPosition  int       `json:"anchorPosition"`
}

// Error 返回可本地化的处理失败原因，不包含原件或供应商凭据。
type Error struct {
	Code  string                         `json:"code"`
	Stage domain.KnowledgeDocumentStatus `json:"stage"`
}

// Error 返回语言无关的失败原因。
func (e *Error) Error() string { return "knowledge processing: " + e.Code }
