package domain

import (
	"path/filepath"
	"strings"
)

// KnowledgeDocumentStatus 表示文档处理流程的执行状态。
type KnowledgeDocumentStatus string

const (
	KnowledgeDocumentInitial     KnowledgeDocumentStatus = "initial"
	KnowledgeDocumentQueued      KnowledgeDocumentStatus = "queued"
	KnowledgeDocumentFetching    KnowledgeDocumentStatus = "fetching"
	KnowledgeDocumentConverting  KnowledgeDocumentStatus = "converting"
	KnowledgeDocumentExtracting  KnowledgeDocumentStatus = "extracting"
	KnowledgeDocumentRecognizing KnowledgeDocumentStatus = "recognizing"
	KnowledgeDocumentSplitting   KnowledgeDocumentStatus = "splitting"
	KnowledgeDocumentEmbedding   KnowledgeDocumentStatus = "embedding"
	KnowledgeDocumentIndexing    KnowledgeDocumentStatus = "indexing"
	KnowledgeDocumentPublishing  KnowledgeDocumentStatus = "publishing"
	KnowledgeDocumentSucceeded   KnowledgeDocumentStatus = "succeeded"
	KnowledgeDocumentFailed      KnowledgeDocumentStatus = "failed"
	KnowledgeDocumentCancelled   KnowledgeDocumentStatus = "cancelled"
)

// IsProcessing 判断文档是否处于待处理或执行中的状态。
func (status KnowledgeDocumentStatus) IsProcessing() bool {
	switch status {
	case KnowledgeDocumentQueued, KnowledgeDocumentFetching, KnowledgeDocumentConverting, KnowledgeDocumentExtracting, KnowledgeDocumentRecognizing, KnowledgeDocumentSplitting, KnowledgeDocumentEmbedding, KnowledgeDocumentIndexing, KnowledgeDocumentPublishing:
		return true
	default:
		return false
	}
}

// KnowledgeDocumentFormat 表示 Haystack 内置转换器支持的文件扩展名。
type KnowledgeDocumentFormat string

const (
	KnowledgeDocumentTXT      KnowledgeDocumentFormat = ".txt"
	KnowledgeDocumentMD       KnowledgeDocumentFormat = ".md"
	KnowledgeDocumentMarkdown KnowledgeDocumentFormat = ".markdown"
	KnowledgeDocumentHTML     KnowledgeDocumentFormat = ".html"
	KnowledgeDocumentHTM      KnowledgeDocumentFormat = ".htm"
	KnowledgeDocumentPDF      KnowledgeDocumentFormat = ".pdf"
	KnowledgeDocumentDOCX     KnowledgeDocumentFormat = ".docx"
	KnowledgeDocumentPPTX     KnowledgeDocumentFormat = ".pptx"
	KnowledgeDocumentXLSX     KnowledgeDocumentFormat = ".xlsx"
	KnowledgeDocumentCSV      KnowledgeDocumentFormat = ".csv"
	KnowledgeDocumentJSON     KnowledgeDocumentFormat = ".json"
)

// KnowledgeDocumentContentType 按扩展名返回知识文档的规范内容类型。
func KnowledgeDocumentContentType(name string) string {
	switch KnowledgeDocumentFormat(strings.ToLower(filepath.Ext(name))) {
	case KnowledgeDocumentTXT:
		return "text/plain"
	case KnowledgeDocumentMD, KnowledgeDocumentMarkdown:
		return "text/markdown"
	case KnowledgeDocumentHTML, KnowledgeDocumentHTM:
		return "text/html"
	case KnowledgeDocumentPDF:
		return "application/pdf"
	case KnowledgeDocumentDOCX:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case KnowledgeDocumentPPTX:
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case KnowledgeDocumentXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case KnowledgeDocumentCSV:
		return "text/csv"
	case KnowledgeDocumentJSON:
		return "application/json"
	default:
		return ""
	}
}
