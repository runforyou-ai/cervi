//go:build server

package knowledgebase

import (
	"context"
	"fmt"
	"strings"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

type difyKnowledgeDocumentSegmentLister interface {
	ListSegments(context.Context, connector.DifyKnowledgeBaseConfig, string, string, connector.DifyKnowledgeDocumentSegmentListInput) (connector.DifyKnowledgeDocumentSegmentPage, error)
}

// ListKnowledgeDocumentSegmentsQuery 查询外部知识文档中的分段。
type ListKnowledgeDocumentSegmentsQuery struct {
	db     *bun.DB
	lister difyKnowledgeDocumentSegmentLister
}

// NewListKnowledgeDocumentSegmentsQuery 创建知识文档分段列表查询。
func NewListKnowledgeDocumentSegmentsQuery(
	db *bun.DB,
	lister difyKnowledgeDocumentSegmentLister,
) *ListKnowledgeDocumentSegmentsQuery {
	return &ListKnowledgeDocumentSegmentsQuery{db: db, lister: lister}
}

// Execute 返回当前企业指定外部知识文档的一页分段。
func (q *ListKnowledgeDocumentSegmentsQuery) Execute(
	ctx context.Context,
	identity *servermodels.Identity,
	knowledgeBaseID, documentID string,
	input DocumentSegmentListInput,
) (DocumentSegmentListOutput, error) {
	input, fields := normalizeDocumentSegmentListInput(input)
	if len(fields) > 0 {
		return DocumentSegmentListOutput{}, &common.FieldError{Fields: fields}
	}
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return DocumentSegmentListOutput{}, err
	}
	documentID = strings.TrimSpace(documentID)
	access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return DocumentSegmentListOutput{}, err
	}
	if documentID == "" {
		return DocumentSegmentListOutput{}, ErrDocumentNotFound
	}
	page, err := q.lister.ListSegments(
		ctx,
		access.Config,
		access.DatasetID,
		documentID,
		connector.DifyKnowledgeDocumentSegmentListInput{
			Keyword: input.Keyword, Page: input.Page, PageSize: input.PageSize,
		},
	)
	if err != nil {
		return DocumentSegmentListOutput{}, fmt.Errorf("list external knowledge document segments: %w", err)
	}
	segments := make([]DocumentSegmentRecord, 0, len(page.Segments))
	for _, segment := range page.Segments {
		indexStatus, err := knowledgeDocumentSegmentIndexStatusFromDify(segment.Status)
		if err != nil {
			return DocumentSegmentListOutput{}, err
		}
		segments = append(segments, DocumentSegmentRecord{
			ID: segment.ID, Position: segment.Position, Content: segment.Content, Answer: segment.Answer,
			WordCount: segment.WordCount, HitCount: segment.HitCount, IndexStatus: indexStatus,
			CreatedAt: segment.CreatedAt,
		})
	}
	return DocumentSegmentListOutput{
		Segments: segments, Page: input.Page, PageSize: input.PageSize, Total: page.Total,
	}, nil
}

// knowledgeDocumentSegmentIndexStatusFromDify 把 Dify 分段状态映射为统一索引状态。
func knowledgeDocumentSegmentIndexStatusFromDify(
	status string,
) (domain.KnowledgeDocumentSegmentIndexStatus, error) {
	switch status {
	case "waiting":
		return domain.KnowledgeDocumentSegmentIndexStatusWaiting, nil
	case "indexing":
		return domain.KnowledgeDocumentSegmentIndexStatusIndexing, nil
	case "completed":
		return domain.KnowledgeDocumentSegmentIndexStatusCompleted, nil
	case "error":
		return domain.KnowledgeDocumentSegmentIndexStatusError, nil
	case "paused":
		return domain.KnowledgeDocumentSegmentIndexStatusPaused, nil
	case "re_segment":
		return domain.KnowledgeDocumentSegmentIndexStatusResegment, nil
	default:
		return "", fmt.Errorf("unsupported dify knowledge document segment status %q", status)
	}
}

// normalizeDocumentSegmentListInput 规范知识文档分段查询条件并校验分页范围。
func normalizeDocumentSegmentListInput(
	input DocumentSegmentListInput,
) (DocumentSegmentListInput, map[string]common.FieldCode) {
	input.Keyword = strings.TrimSpace(input.Keyword)
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
