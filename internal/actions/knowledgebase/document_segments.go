//go:build server

package knowledgebase

import (
	"context"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

type segmentReader interface {
	List(context.Context, knowledgeprocessing.ListInput) (knowledgeprocessing.SegmentPage, error)
}

// Segments 校验当前企业文档和固定批次后读取一页分段。
func (q *DocumentQuery) Segments(ctx context.Context, identity *servermodels.Identity, baseID, documentID string, input knowledgeprocessing.ListInput) (knowledgeprocessing.SegmentPage, error) {
	empty := knowledgeprocessing.SegmentPage{}
	record, err := q.Get(ctx, identity, baseID, documentID)
	if err != nil {
		return empty, err
	}
	if record.SegmentBatchID == "" {
		return empty, ErrSegmentsNotReady
	}
	if input.SegmentBatchID != "" && input.SegmentBatchID != record.SegmentBatchID {
		return empty, ErrSegmentStale
	}
	if input.AnchorSegmentID != "" && (input.Page != 0 || input.SegmentBatchID == "") {
		return empty, ErrSegmentQueryInvalid
	}
	if input.Page < 0 || input.PageSize < 0 || input.PageSize > 100 || (input.AnchorSegmentID != "" && !common.ValidUUID(input.AnchorSegmentID)) {
		return empty, ErrSegmentQueryInvalid
	}
	if input.Page == 0 && input.AnchorSegmentID == "" {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = 20
	}
	input.OrganizationID, input.KnowledgeBaseID, input.DocumentID, input.SegmentBatchID = identity.Organization.ID, baseID, documentID, record.SegmentBatchID
	page, err := q.segments.List(ctx, input)
	if err != nil {
		var failure *knowledgeprocessing.Error
		if errors.As(err, &failure) && failure.Code == "segment_stale" {
			return empty, ErrSegmentStale
		}
		return empty, err
	}
	// 返回前再次核验来源，拒绝已删除或换批次的内容。
	current, err := q.Get(ctx, identity, baseID, documentID)
	if err != nil {
		return empty, err
	}
	if current.SegmentBatchID != page.SegmentBatchID {
		return empty, ErrSegmentStale
	}
	return page, nil
}
