//go:build server

package aiperformance

import (
	"context"
	"fmt"
	"slices"

	"github.com/runforyou-ai/cervi/internal/actions/knowledgegap"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// OverviewQuery 读取 AI 表现报表概览。
type OverviewQuery struct{ db *bun.DB }

// NewOverviewQuery 创建 AI 表现报表概览查询。
func NewOverviewQuery(db *bun.DB) *OverviewQuery { return &OverviewQuery{db: db} }

// Execute 汇总统计范围内已关闭周期的整体计数与转人工原因分布，以及所选渠道下全部待处理的待补知识条数。
func (q *OverviewQuery) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Overview, error) {
	scope, args := reportScope(identity, input)
	overview := &Overview{HandoffReasons: []ReasonCount{}}
	if err := q.db.NewRaw(scope+`
SELECT count(*) AS closed,
	count(*) FILTER (WHERE resolved) AS resolved,
	count(*) FILTER (WHERE NOT resolved) AS unresolved,
	count(*) FILTER (WHERE ai_only) AS ai_only,
	count(*) FILTER (WHERE ai_only AND resolved) AS ai_resolved,
	count(*) FILTER (WHERE ai_only AND NOT resolved) AS ai_unresolved,
	(SELECT count(DISTINCT service_session_id) FROM handoffs) AS handed_off,
	count(*) FILTER (WHERE close_reason = ?) AS close_ai_resolved,
	count(*) FILTER (WHERE close_reason = ?) AS customer_unresponsive,
	count(*) FILTER (WHERE close_reason = ?) AS manual,
	count(rating_resolved) AS rated,
	count(*) FILTER (WHERE rating_resolved) AS rated_resolved
FROM closed`, slices.Concat(args, []any{domain.ServiceSessionCloseAIResolved, domain.ServiceSessionCloseCustomerUnresponsive, domain.ServiceSessionCloseManual})...).
		Scan(ctx, &overview.Summary); err != nil {
		return nil, fmt.Errorf("summarize closed service sessions: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT reason, count(*) AS count
FROM handoffs
GROUP BY reason
ORDER BY count DESC, reason ASC`, args...).Scan(ctx, &overview.HandoffReasons); err != nil {
		return nil, fmt.Errorf("count handoff reasons: %w", err)
	}
	total, err := knowledgegap.PendingCount(ctx, q.db, identity.Organization.ID, input.ChannelID)
	if err != nil {
		return nil, err
	}
	overview.KnowledgeGapTotal = total
	return overview, nil
}
