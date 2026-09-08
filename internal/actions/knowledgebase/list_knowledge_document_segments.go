//go:build server

package knowledgebase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
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
	documentID = strings.TrimSpace(documentID)
	access, err := loadDifyKnowledgeAccess(ctx, q.db, identity.Organization.ID, knowledgeBaseID)
	if err != nil {
		return DocumentSegmentListOutput{}, err
	}
	if documentID == "" {
		return DocumentSegmentListOutput{}, ErrDocumentNotFound
	}
	page, pageNumber, err := q.readPage(ctx, access, documentID, input)
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
		Segments: segments, Page: pageNumber, PageSize: input.PageSize, Total: page.Total,
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
	input.SegmentID = strings.TrimSpace(input.SegmentID)
	input.Keyword = strings.TrimSpace(input.Keyword)
	input.Status = domain.KnowledgeDocumentSegmentIndexStatus(strings.TrimSpace(string(input.Status)))
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = defaultKnowledgeDocumentPageSize
	}
	fields := make(map[string]common.FieldCode)
	if input.SegmentID != "" && (input.Position <= 0 || input.Keyword != "" || input.Status != "") {
		fields["segmentId"] = ValidationDocumentQueryInvalid
	}
	if input.PageSize > 100 {
		fields["pageSize"] = ValidationDocumentQueryInvalid
	}
	if input.Status != "" {
		if _, err := knowledgeDocumentSegmentIndexStatusFromDify(string(input.Status)); err != nil {
			fields["status"] = ValidationDocumentQueryInvalid
		}
	}
	return input, fields
}

// readPage 在按位置排序的分段中定位命中页，允许分段删除后序号不连续。
func (q *ListKnowledgeDocumentSegmentsQuery) readPage(ctx context.Context, access difyKnowledgeAccess, documentID string, input DocumentSegmentListInput) (connector.DifyKnowledgeDocumentSegmentPage, int, error) {
	pageNumber := input.Page
	if input.SegmentID != "" {
		pageNumber = (input.Position-1)/input.PageSize + 1
	}
	low, high := 1, pageNumber
	for low <= high {
		page, err := q.lister.ListSegments(ctx, access.Config, access.DatasetID, documentID, connector.DifyKnowledgeDocumentSegmentListInput{Keyword: input.Keyword, Status: string(input.Status), Page: pageNumber, PageSize: input.PageSize})
		if err != nil {
			return page, pageNumber, err
		}
		if input.SegmentID == "" {
			return page, pageNumber, nil
		}
		for _, segment := range page.Segments {
			if segment.ID == input.SegmentID && segment.Position == input.Position {
				return page, pageNumber, nil
			}
		}
		high = min(high, (page.Total+input.PageSize-1)/input.PageSize)
		if len(page.Segments) == 0 || page.Segments[0].Position > input.Position {
			high = min(high, pageNumber-1)
		} else if page.Segments[len(page.Segments)-1].Position < input.Position {
			low = pageNumber + 1
		} else {
			break
		}
		pageNumber = (low + high) / 2
	}
	return connector.DifyKnowledgeDocumentSegmentPage{}, 0, connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureNotFound, errors.New("knowledge segment no longer exists at the requested position"))
}
