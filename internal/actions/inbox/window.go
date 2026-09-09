//go:build server

package inbox

import (
	"context"
	"fmt"
	"slices"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ConversationWindow 保存连续列表窗口及可双向续读的原始边界。
type ConversationWindow struct {
	Conversations []ConversationSummary
	StartCursor   string
	EndCursor     string
	HasBefore     bool
	HasAfter      bool
}

// candidatePointsQuery 复用所有列表窗口的资格和最小排序投影。
func (q *LoadInboxQuery) candidatePointsQuery(identity *servermodels.Identity, input LoadInput) *bun.SelectQuery {
	return q.db.NewSelect().TableExpr("(?) AS candidates", q.listCandidates(identity, input)).ColumnExpr("id, last_activity_at")
}

// constrainInboxPoint 按活动降序施加前后边界，包含边界时也适用于闭区间重读。
func constrainInboxPoint(query *bun.SelectQuery, point inboxCursorPoint, before, inclusive bool) *bun.SelectQuery {
	operator := "<"
	if before {
		operator = ">"
	}
	if inclusive {
		operator += "="
	}
	if point.LastActivityAt == nil {
		if before {
			return query.Where("(last_activity_at IS NOT NULL OR (last_activity_at IS NULL AND id "+operator+" ?))", point.ID)
		}
		return query.Where("last_activity_at IS NULL AND id "+operator+" ?", point.ID)
	}
	if before {
		return query.Where("(last_activity_at, id) "+operator+" (?, ?)", *point.LastActivityAt, point.ID)
	}
	return query.Where("((last_activity_at, id) "+operator+" (?, ?) OR last_activity_at IS NULL)", *point.LastActivityAt, point.ID)
}

// readNeighborPoints 从边界向指定方向读取最近的候选，并统一返回活动降序。
func (q *LoadInboxQuery) readNeighborPoints(ctx context.Context, identity *servermodels.Identity, input LoadInput, point *inboxCursorPoint, before bool, limit int) ([]inboxCursorPoint, error) {
	query := q.candidatePointsQuery(identity, input)
	if point != nil {
		query = constrainInboxPoint(query, *point, before, false)
	}
	if before {
		query.OrderExpr("last_activity_at ASC NULLS FIRST, id ASC")
	} else {
		query.OrderExpr("last_activity_at DESC NULLS LAST, id DESC")
	}
	var points []inboxCursorPoint
	if err := query.Limit(limit).Scan(ctx, &points); err != nil {
		return nil, fmt.Errorf("read inbox neighbors: %w", err)
	}
	if before {
		slices.Reverse(points)
	}
	return points, nil
}

// buildConversationWindow 在调用方快照内读取摘要、位置游标及窗口外资格。
func (q *LoadInboxQuery) buildConversationWindow(ctx context.Context, identity *servermodels.Identity, input LoadInput, points []inboxCursorPoint, start, end *inboxCursorPoint) (ConversationWindow, error) {
	window := ConversationWindow{Conversations: make([]ConversationSummary, 0, len(points))}
	if start != nil {
		var err error
		window.StartCursor, err = encodeInboxCursor(identity, input, *start)
		if err != nil {
			return window, err
		}
		window.EndCursor, err = encodeInboxCursor(identity, input, *end)
		if err != nil {
			return window, err
		}
		window.HasBefore, err = constrainInboxPoint(q.candidatePointsQuery(identity, input), *start, true, false).Exists(ctx)
		if err != nil {
			return window, fmt.Errorf("probe inbox before window: %w", err)
		}
		window.HasAfter, err = constrainInboxPoint(q.candidatePointsQuery(identity, input), *end, false, false).Exists(ctx)
		if err != nil {
			return window, fmt.Errorf("probe inbox after window: %w", err)
		}
	}
	if len(points) == 0 {
		return window, nil
	}
	ids := make([]string, len(points))
	for index, point := range points {
		ids[index] = point.ID
	}
	summaries, err := q.readSummaries(ctx, identity, ids)
	if err != nil {
		return window, err
	}
	// 候选与摘要共用阅读资格和快照，位置游标保留当前查询及数据库精度。
	for _, point := range points {
		summary := *summaries[point.ID]
		summary.PositionCursor, err = encodeInboxCursor(identity, input, point)
		if err != nil {
			return window, err
		}
		window.Conversations = append(window.Conversations, summary)
	}
	return window, nil
}
