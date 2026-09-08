//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DocumentQuery 读取当前企业中的文档和可预览原件。
type DocumentQuery struct{ db *bun.DB }

// NewDocumentQuery 创建知识文档查询。
func NewDocumentQuery(db *bun.DB) *DocumentQuery { return &DocumentQuery{db: db} }

// List 按创建时间倒序返回分组文档。
func (q *DocumentQuery) List(ctx context.Context, identity *servermodels.Identity, baseID string, input DocumentListInput) (DocumentListOutput, error) {
	base, err := loadKnowledgeBase(ctx, q.db, identity.Organization.ID, baseID)
	if err != nil {
		return DocumentListOutput{}, err
	}
	if base.Category != string(domain.KnowledgeBaseCategoryStandard) {
		return DocumentListOutput{}, ErrDocumentUnsupported
	}
	if _, err := loadKnowledgeGroup(ctx, q.db, identity.Organization.ID, baseID, input.GroupID); err != nil {
		return DocumentListOutput{}, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 20
	}
	records := make([]DocumentRecord, 0)
	query := documentSelect(q.db).Where("kd.knowledge_base_id = ? AND kd.group_id = ?", baseID, input.GroupID)
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		// 搜索文件名时按字面处理通配符。
		pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword) + "%"
		query = query.Where("f.original_name ILIKE ?", pattern)
	}
	total, err := query.OrderExpr("kd.created_at DESC, kd.id DESC").Limit(input.PageSize).Offset((input.Page-1)*input.PageSize).ScanAndCount(ctx, &records)
	return DocumentListOutput{Documents: records, Page: input.Page, PageSize: input.PageSize, Total: total}, err
}

// Get 读取当前企业知识库中的文档详情。
func (q *DocumentQuery) Get(ctx context.Context, identity *servermodels.Identity, baseID, documentID string) (*DocumentRecord, error) {
	if _, err := loadKnowledgeBase(ctx, q.db, identity.Organization.ID, baseID); err != nil {
		return nil, err
	}
	return loadDocumentRecord(ctx, q.db, baseID, documentID)
}

// File 读取仍关联当前企业文档的已激活原件。
func (q *DocumentQuery) File(ctx context.Context, identity *servermodels.Identity, baseID, documentID string) (*servermodels.File, error) {
	if !common.ValidUUID(baseID) || !common.ValidUUID(documentID) {
		return nil, ErrDocumentNotFound
	}
	record := &servermodels.File{}
	err := q.db.NewSelect().Model(record).Join("JOIN knowledge_documents kd ON kd.file_id = f.id").Join("JOIN knowledge_bases kb ON kb.id = kd.knowledge_base_id").Where("kd.id = ? AND kb.id = ? AND kb.organization_id = ? AND f.organization_id = ?", documentID, baseID, identity.Organization.ID, identity.Organization.ID).Where("f.status = ?", domain.FileStatusActive).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDocumentNotFound
	}
	return record, err
}

// documentSelect 联结原件元数据，不在文档中重复保存文件属性。
func documentSelect(db bun.IDB) *bun.SelectQuery {
	return db.NewSelect().TableExpr("knowledge_documents AS kd").ColumnExpr("kd.id, kd.group_id, kd.status, kd.created_at, f.original_name AS name, f.content_type, f.byte_size").Join("JOIN files f ON f.id = kd.file_id")
}

// loadDocumentRecord 读取指定知识库下的一条文档。
func loadDocumentRecord(ctx context.Context, db bun.IDB, baseID, documentID string) (*DocumentRecord, error) {
	if !common.ValidUUID(documentID) {
		return nil, ErrDocumentNotFound
	}
	record := &DocumentRecord{}
	err := documentSelect(db).Where("kd.knowledge_base_id = ? AND kd.id = ?", baseID, documentID).Scan(ctx, record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDocumentNotFound
	}
	return record, err
}
