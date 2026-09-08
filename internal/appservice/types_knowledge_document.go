package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// KnowledgeDocumentStatus 定义文档处理状态。
type KnowledgeDocumentStatus string

const KnowledgeDocumentInitial KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentInitial)

// KnowledgeDocumentFormat 定义允许上传的文档扩展名。
type KnowledgeDocumentFormat string

const (
	KnowledgeDocumentTXT      KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentTXT)
	KnowledgeDocumentMD       KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentMD)
	KnowledgeDocumentMarkdown KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentMarkdown)
	KnowledgeDocumentHTML     KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentHTML)
	KnowledgeDocumentHTM      KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentHTM)
	KnowledgeDocumentPDF      KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentPDF)
	KnowledgeDocumentDOCX     KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentDOCX)
	KnowledgeDocumentPPTX     KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentPPTX)
	KnowledgeDocumentXLSX     KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentXLSX)
	KnowledgeDocumentCSV      KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentCSV)
	KnowledgeDocumentJSON     KnowledgeDocumentFormat = KnowledgeDocumentFormat(domain.KnowledgeDocumentJSON)
)

// KnowledgeDocument 定义文档列表与预览页使用的元数据。
type KnowledgeDocument struct {
	Format      KnowledgeDocumentFormat `json:"format"`
	ID          string                  `json:"id"`
	GroupID     string                  `json:"groupId"`
	Name        string                  `json:"name"`
	ContentType string                  `json:"contentType"`
	ByteSize    int64                   `json:"byteSize"`
	Status      KnowledgeDocumentStatus `json:"status"`
	CreatedAt   time.Time               `json:"createdAt"`
}

// KnowledgeDocumentListInput 定义分组文档的查询参数。
type KnowledgeDocumentListInput struct {
	GroupID  string `json:"groupId" query:"groupId"`
	Keyword  string `json:"keyword" query:"keyword"`
	Page     int    `json:"page" query:"page,default=1"`
	PageSize int    `json:"pageSize" query:"pageSize,default=20"`
}

// KnowledgeDocumentList 返回文档及分页信息。
type KnowledgeDocumentList struct {
	Documents []KnowledgeDocument `json:"documents"`
	Page      PageInfo            `json:"page"`
}

// KnowledgeDocumentBatchInput 将最多十个已上传原件保存到分组。
type KnowledgeDocumentBatchInput struct {
	GroupID string   `json:"groupId"`
	FileIDs []string `json:"fileIds"`
}

// KnowledgeDocumentBatch 返回已保存文档，供重试核对。
type KnowledgeDocumentBatch struct {
	Documents []KnowledgeDocument `json:"documents"`
}

// KnowledgeDocumentMoveInput 定义目标分组。
type KnowledgeDocumentMoveInput struct {
	GroupID string `json:"groupId"`
}

// KnowledgeDocumentPreviewRequest 定义读取原件用于本地预览的请求。
type KnowledgeDocumentPreviewRequest struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}
