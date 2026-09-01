//go:build server

package knowledgebase

import (
	"context"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type difyKnowledgeDocumentGetter interface {
	Get(context.Context, connector.DifyKnowledgeBaseConfig, string, string) (connector.DifyKnowledgeDocument, error)
}

// GetKnowledgeDocumentQuery 查询外部知识文档详情。
type GetKnowledgeDocumentQuery struct {
	db     *bun.DB
	getter difyKnowledgeDocumentGetter
}

// NewGetKnowledgeDocumentQuery 创建知识文档详情查询。
func NewGetKnowledgeDocumentQuery(db *bun.DB, getter difyKnowledgeDocumentGetter) *GetKnowledgeDocumentQuery {
	return &GetKnowledgeDocumentQuery{db: db, getter: getter}
}

// Execute 返回当前企业指定外部知识文档的详情。
func (q *GetKnowledgeDocumentQuery) Execute(
	ctx context.Context,
	identity *servermodels.Identity,
	knowledgeBaseID, documentID string,
) (DocumentDetailRecord, error) {
	documentID = strings.TrimSpace(documentID)
	access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return DocumentDetailRecord{}, err
	}
	if documentID == "" {
		return DocumentDetailRecord{}, ErrDocumentNotFound
	}
	document, err := q.getter.Get(ctx, access.Config, access.DatasetID, documentID)
	if err != nil {
		return DocumentDetailRecord{}, fmt.Errorf("get external knowledge document: %w", err)
	}
	status, err := knowledgeDocumentStatusFromDify(document.Status)
	if err != nil {
		return DocumentDetailRecord{}, err
	}
	return DocumentDetailRecord{
		ID: document.ID, Name: document.Name, Status: status,
		WordCount: document.WordCount, HitCount: document.HitCount, CreatedAt: document.CreatedAt,
	}, nil
}
