//go:build server

package knowledgebase

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// RetrieveKnowledgeBaseQuery 检索外部知识库。
type RetrieveKnowledgeBaseQuery struct {
	search *SearchService
}

// NewRetrieveKnowledgeBaseQuery 创建知识库检索查询。
func NewRetrieveKnowledgeBaseQuery(search *SearchService) *RetrieveKnowledgeBaseQuery {
	return &RetrieveKnowledgeBaseQuery{search: search}
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
	search, err := q.search.ForKnowledgeBase(ctx, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	result, err := search(ctx, knowledgeretrieval.Request{Queries: []string{input.Query}})
	if err != nil {
		return nil, fmt.Errorf("retrieve external knowledge base: %w", err)
	}
	output := make([]RetrievalRecord, 0, len(result.Records))
	for _, record := range result.Records {
		output = append(output, RetrievalRecord{
			DocumentID: record.DocumentID, DocumentName: record.DocumentName,
			SegmentID: record.SegmentID, Position: record.Position,
			Content: record.Content, Answer: record.Answer, Score: record.Score,
		})
	}
	return output, nil
}
