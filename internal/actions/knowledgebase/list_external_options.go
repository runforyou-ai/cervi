//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type difyKnowledgeBaseLister interface {
	List(context.Context, connector.DifyKnowledgeBaseConfig) ([]connector.DifyKnowledgeBase, error)
}

// ListExternalOptionsQuery 查询 Dify 连接可访问的知识库选择项。
type ListExternalOptionsQuery struct {
	db     *bun.DB
	lister difyKnowledgeBaseLister
}

// NewListExternalOptionsQuery 创建外部知识库选择项查询。
func NewListExternalOptionsQuery(db *bun.DB, lister difyKnowledgeBaseLister) *ListExternalOptionsQuery {
	return &ListExternalOptionsQuery{db: db, lister: lister}
}

// Execute 返回当前企业指定 Dify 连接可访问的全部知识库。
func (q *ListExternalOptionsQuery) Execute(
	ctx context.Context,
	identity *servermodels.Identity,
	connectionID string,
) ([]ExternalOptionRecord, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	configuration, err := loadDifyConfiguration(ctx, q.db, identity.Organization.ID, connectionID)
	if err != nil {
		return nil, err
	}
	items, err := q.lister.List(ctx, connector.DifyKnowledgeBaseConfig{
		APIURL: configuration.APIURL,
		APIKey: configuration.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("list external knowledge bases: %w", err)
	}

	records := make([]ExternalOptionRecord, 0, len(items))
	for _, item := range items {
		category, err := categoryFromDifyDocForm(item.DocForm)
		if err != nil {
			return nil, err
		}
		records = append(records, ExternalOptionRecord{
			ID: item.ID, Name: item.Name, Category: category,
		})
	}
	return records, nil
}

// categoryFromDifyDocForm 把 Dify 文档模式映射为知识库类型，空模式按文档型处理。
func categoryFromDifyDocForm(docForm string) (domain.KnowledgeBaseCategory, error) {
	switch docForm {
	case "", "text_model", "hierarchical_model":
		return domain.KnowledgeBaseCategoryStandard, nil
	case "qa_model":
		return domain.KnowledgeBaseCategoryQA, nil
	default:
		return "", fmt.Errorf("unsupported dify doc_form %q", docForm)
	}
}
