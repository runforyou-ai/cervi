//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type difyKnowledgeRetriever interface {
	Retrieve(
		context.Context,
		connector.DifyKnowledgeBaseConfig,
		string,
		string,
		domain.KnowledgeRetrievalOptions,
	) ([]connector.DifyKnowledgeRetrievalRecord, error)
}

// RetrieveKnowledgeBaseQuery 检索外部知识库。
type RetrieveKnowledgeBaseQuery struct {
	db        *bun.DB
	retriever difyKnowledgeRetriever
}

// NewRetrieveKnowledgeBaseQuery 创建知识库检索查询。
func NewRetrieveKnowledgeBaseQuery(db *bun.DB, retriever difyKnowledgeRetriever) *RetrieveKnowledgeBaseQuery {
	return &RetrieveKnowledgeBaseQuery{db: db, retriever: retriever}
}

// Execute 返回当前企业指定外部知识库的检索命中项。
func (q *RetrieveKnowledgeBaseQuery) Execute(
	ctx context.Context,
	identity *servermodels.Identity,
	knowledgeBaseID string,
	input RetrievalInput,
) ([]RetrievalRecord, error) {
	input, fields := normalizeRetrievalInput(input)
	if len(fields) > 0 {
		return nil, &common.FieldError{Fields: fields}
	}
	access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	records, err := q.retriever.Retrieve(ctx, access.Config, access.DatasetID, input.Query, domain.KnowledgeRetrievalOptions{
		Method: input.Method, RerankingEnabled: input.RerankingEnabled, TopK: input.TopK,
		ScoreThresholdEnabled: input.ScoreThresholdEnabled, ScoreThreshold: input.ScoreThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("retrieve external knowledge base: %w", err)
	}
	output := make([]RetrievalRecord, 0, len(records))
	for _, record := range records {
		output = append(output, RetrievalRecord{
			DocumentID: record.DocumentID, DocumentName: record.DocumentName,
			SegmentID: record.SegmentID, Position: record.Position,
			Content: record.Content, Answer: record.Answer, Score: record.Score,
		})
	}
	return output, nil
}
