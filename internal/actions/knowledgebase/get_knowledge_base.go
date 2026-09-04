//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type knowledgeBaseDetailGetter interface {
	Get(context.Context, connector.DifyKnowledgeBaseConfig, string) (connector.DifyKnowledgeBaseDetail, error)
}

// GetKnowledgeBaseQuery 查询当前企业的指定知识库。
type GetKnowledgeBaseQuery struct {
	db     *bun.DB
	getter knowledgeBaseDetailGetter
}

// NewGetKnowledgeBaseQuery 创建知识库详情查询。
func NewGetKnowledgeBaseQuery(db *bun.DB, getter knowledgeBaseDetailGetter) *GetKnowledgeBaseQuery {
	return &GetKnowledgeBaseQuery{db: db, getter: getter}
}

// Execute 返回指定知识库详情。
func (q *GetKnowledgeBaseQuery) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string) (*Record, error) {
	record, err := loadKnowledgeBaseRecord(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return nil, fmt.Errorf("get knowledge base: %w", err)
	}
	if record.IntegrationConnectionID != "" {
		access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
		if err != nil {
			return nil, fmt.Errorf("load dify knowledge access: %w", err)
		}
		detail, err := q.getter.Get(ctx, access.Config, access.DatasetID)
		if err != nil {
			return nil, fmt.Errorf("get dify knowledge base configuration: %w", err)
		}
		record.ExternalConfiguration = &ExternalConfigurationRecord{
			IndexingTechnique: detail.IndexingTechnique, DocumentCount: detail.DocumentCount,
			WordCount: detail.WordCount, EmbeddingModel: detail.EmbeddingModel,
			EmbeddingModelProvider: detail.EmbeddingModelProvider,
			RetrievalMethod:        detail.RetrievalModel.SearchMethod, TopK: detail.RetrievalModel.TopK,
			ScoreThresholdEnabled: detail.RetrievalModel.ScoreThresholdEnabled,
			ScoreThreshold:        detail.RetrievalModel.ScoreThreshold,
			RerankingEnabled:      detail.RetrievalModel.RerankingEnable,
			RerankingModel:        detail.RetrievalModel.RerankingModel.Name,
			RerankingProvider:     detail.RetrievalModel.RerankingModel.Provider,
		}
	}
	return record, nil
}
