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

// DocumentRecord 汇总文档归属与原件元数据。
type DocumentRecord struct {
	ID          string                         `bun:"id"`
	GroupID     string                         `bun:"group_id"`
	Name        string                         `bun:"name"`
	ContentType string                         `bun:"content_type"`
	ByteSize    int64                          `bun:"byte_size"`
	Status      domain.KnowledgeDocumentStatus `bun:"status"`
	CreatedAt   time.Time                      `bun:"created_at"`
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
