//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const defaultKnowledgeDocumentPageSize = 20

type difyKnowledgeDocumentLister interface {
	List(context.Context, connector.DifyKnowledgeBaseConfig, string, int, int) (connector.DifyKnowledgeDocumentPage, error)
}

// ListKnowledgeDocumentsQuery 查询外部知识库中的文档。
type ListKnowledgeDocumentsQuery struct {
	db     *bun.DB
	lister difyKnowledgeDocumentLister
}

// NewListKnowledgeDocumentsQuery 创建知识文档列表查询。
func NewListKnowledgeDocumentsQuery(db *bun.DB, lister difyKnowledgeDocumentLister) *ListKnowledgeDocumentsQuery {
	return &ListKnowledgeDocumentsQuery{db: db, lister: lister}
}

// Execute 返回当前企业指定外部知识库的一页文档。
func (q *ListKnowledgeDocumentsQuery) Execute(
	ctx context.Context,
	identity *servermodels.Identity,
	knowledgeBaseID string,
	input DocumentListInput,
) (DocumentListOutput, error) {
	input, fields := normalizeDocumentListInput(input)
	if len(fields) > 0 {
		return DocumentListOutput{}, &common.FieldError{Fields: fields}
	}
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return DocumentListOutput{}, err
	}
	knowledgeBase, err := loadKnowledgeBase(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return DocumentListOutput{}, err
	}
	if knowledgeBase.IntegrationConnectionID == nil || knowledgeBase.ExternalResourceID == nil {
		return DocumentListOutput{}, ErrDocumentListUnsupported
	}
	configuration, err := loadDifyConfiguration(
		ctx,
		q.db,
		identity.Organization.ID,
		*knowledgeBase.IntegrationConnectionID,
	)
	if err != nil {
		return DocumentListOutput{}, err
	}
	page, err := q.lister.List(ctx, connector.DifyKnowledgeBaseConfig{
		APIURL: configuration.APIURL,
		APIKey: configuration.APIKey,
	}, *knowledgeBase.ExternalResourceID, input.Page, input.PageSize)
	if err != nil {
		return DocumentListOutput{}, fmt.Errorf("list external knowledge documents: %w", err)
	}

	documents := make([]DocumentRecord, 0, len(page.Documents))
	for _, document := range page.Documents {
		status, err := knowledgeDocumentStatusFromDify(document.Status)
		if err != nil {
			return DocumentListOutput{}, err
		}
		documents = append(documents, DocumentRecord{
			ID: document.ID, Name: document.Name, Status: status, CreatedAt: document.CreatedAt,
		})
	}
	return DocumentListOutput{
		Documents: documents, Page: input.Page, PageSize: input.PageSize, Total: page.Total,
	}, nil
}

// normalizeDocumentListInput 补齐并校验知识文档分页条件。
func normalizeDocumentListInput(input DocumentListInput) (DocumentListInput, map[string]common.FieldCode) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = defaultKnowledgeDocumentPageSize
	}
	fields := make(map[string]common.FieldCode)
	if input.PageSize > 100 {
		fields["pageSize"] = ValidationDocumentQueryInvalid
	}
	return input, fields
}

// knowledgeDocumentStatusFromDify 把 Dify 展示状态映射为统一知识文档状态。
func knowledgeDocumentStatusFromDify(status string) (domain.KnowledgeDocumentStatus, error) {
	switch status {
	case "queuing", "indexing":
		return domain.KnowledgeDocumentStatusProcessing, nil
	case "available":
		return domain.KnowledgeDocumentStatusReady, nil
	case "paused":
		return domain.KnowledgeDocumentStatusPaused, nil
	case "error":
		return domain.KnowledgeDocumentStatusError, nil
	case "disabled":
		return domain.KnowledgeDocumentStatusDisabled, nil
	case "archived":
		return domain.KnowledgeDocumentStatusArchived, nil
	default:
		return "", fmt.Errorf("unsupported dify knowledge document status %q", status)
	}
}
