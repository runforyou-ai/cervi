//go:build server

package knowledgebase

import (
	"context"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ContextInput 定位指定知识库中的命中分段。
type ContextInput struct {
	DocumentID string
	SegmentID  string
	Position   int
}

// ContextRecord 定义命中文档的上下文分段。
type ContextRecord struct {
	DocumentName string
	SegmentID    string
	Position     int
	Content      string
	Answer       *string
	Matched      bool
}

// ReadKnowledgeContextQuery 读取当前企业知识库中的分段上下文。
type ReadKnowledgeContextQuery struct{ search *SearchService }

// NewReadKnowledgeContextQuery 创建分段上下文查询。
func NewReadKnowledgeContextQuery(search *SearchService) *ReadKnowledgeContextQuery {
	return &ReadKnowledgeContextQuery{search: search}
}

// Execute 在指定知识库范围内读取命中分段及前后最多各两段。
func (q *ReadKnowledgeContextQuery) Execute(ctx context.Context, identity *servermodels.Identity, knowledgeBaseID string, input ContextInput) ([]ContextRecord, error) {
	input.DocumentID = strings.TrimSpace(input.DocumentID)
	input.SegmentID = strings.TrimSpace(input.SegmentID)
	if input.DocumentID == "" || input.SegmentID == "" || input.Position <= 0 {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"context": ValidationContextInvalid}}
	}
	search, err := q.search.ForKnowledgeBase(ctx, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	result, err := search(ctx, knowledgeretrieval.Request{
		Cursor: &knowledgeretrieval.Cursor{KnowledgeBaseID: knowledgeBaseID, DocumentID: input.DocumentID, SegmentID: input.SegmentID, Position: input.Position},
		Before: 2, After: 2,
	})
	if err != nil {
		return nil, err
	}
	output := make([]ContextRecord, 0, len(result.Records))
	for _, record := range result.Records {
		output = append(output, ContextRecord{
			DocumentName: record.DocumentName, SegmentID: record.SegmentID, Position: record.Position,
			Content: record.Content, Answer: record.Answer, Matched: record.Matched,
		})
	}
	return output, nil
}
