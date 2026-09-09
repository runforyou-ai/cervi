//go:build server

package inbox

import (
	"context"
	"database/sql"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ReadWindowInput 指定已加载范围的两个闭区间边界。
type ReadWindowInput struct {
	Query       LoadInput
	StartCursor string
	EndCursor   string
}

// ReadWindow 在一个快照中重读完整旧区间，即使边界行已经移动或失权。
func (q *LoadInboxQuery) ReadWindow(ctx context.Context, identity *servermodels.Identity, input ReadWindowInput) (ConversationWindow, error) {
	query, err := normalizeLoadInput(input.Query)
	if err != nil {
		return ConversationWindow{}, err
	}
	start, err := decodeInboxCursor(input.StartCursor, identity, query)
	if err != nil {
		return ConversationWindow{}, err
	}
	end, err := decodeInboxCursor(input.EndCursor, identity, query)
	if err != nil {
		return ConversationWindow{}, err
	}
	if compareInboxPoints(start.inboxCursorPoint, end.inboxCursorPoint) > 0 {
		return ConversationWindow{}, ErrQueryInvalid
	}
	var window ConversationWindow
	err = q.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		snapshot := NewLoadInboxQuery(tx)
		candidates := constrainInboxPoint(snapshot.candidatePointsQuery(identity, query), start.inboxCursorPoint, false, true)
		candidates = constrainInboxPoint(candidates, end.inboxCursorPoint, true, true)
		var points []inboxCursorPoint
		if err := candidates.OrderExpr("last_activity_at DESC NULLS LAST, id DESC").Scan(ctx, &points); err != nil {
			return err
		}
		var err error
		// 保留请求的范围，空区间也能继续重试或向两侧查找邻域。
		window, err = snapshot.buildConversationWindow(ctx, identity, query, points, &start.inboxCursorPoint, &end.inboxCursorPoint)
		return err
	})
	return window, err
}
