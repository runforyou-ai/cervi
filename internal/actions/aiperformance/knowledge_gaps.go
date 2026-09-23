//go:build server

package aiperformance

import (
	"context"
	"fmt"
	"slices"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// KnowledgeGapsQuery 读取知识缺口清单。
type KnowledgeGapsQuery struct{ db *bun.DB }

// NewKnowledgeGapsQuery 创建知识缺口清单查询。
func NewKnowledgeGapsQuery(db *bun.DB) *KnowledgeGapsQuery { return &KnowledgeGapsQuery{db: db} }

// Execute 按转人工时间倒序返回一页知识缺口及对应的客户提问。
func (q *KnowledgeGapsQuery) Execute(ctx context.Context, identity *servermodels.Identity, input KnowledgeGapInput) (*KnowledgeGapList, error) {
	page, pageSize, valid := common.NormalizePagination(input.Page, input.PageSize)
	if !valid {
		return nil, ErrPageSizeInvalid
	}
	list := &KnowledgeGapList{Gaps: []KnowledgeGap{}, Page: page, PageSize: pageSize}
	scope, args := reportScope(identity, input.Input)
	if err := q.db.NewRaw(scope+`
SELECT count(*) FROM handoffs WHERE reason IN (?)`, slices.Concat(args, []any{bun.In(gapReasons)})...).Scan(ctx, &list.Total); err != nil {
		return nil, fmt.Errorf("count knowledge gaps: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT h.id AS event_id, h.conversation_id, question.id AS message_id, coalesce(question.body, '') AS question,
	h.reason, sc.name AS category_name, h.occurred_at
FROM handoffs h
JOIN closed ON closed.id = h.service_session_id
LEFT JOIN service_categories sc ON sc.id = closed.category_id
LEFT JOIN LATERAL (
	SELECT m.id, m.body FROM messages m
	JOIN conversation_participants cp ON cp.id = m.sender_participant_id
	JOIN chat_subjects cs ON cs.id = cp.subject_id
	WHERE m.organization_id = h.organization_id AND m.service_session_id = h.service_session_id AND m.message_seq < h.message_seq
		AND m.type = ? AND m.deleted_at IS NULL AND cs.kind = ?
	ORDER BY m.message_seq DESC
	LIMIT 1
) question ON true
WHERE h.reason IN (?)
ORDER BY h.occurred_at DESC, h.id DESC
LIMIT ? OFFSET ?`, slices.Concat(args, []any{domain.MessageTypeText, domain.ChatSubjectKindContact, bun.In(gapReasons), pageSize, (page - 1) * pageSize})...).
		Scan(ctx, &list.Gaps); err != nil {
		return nil, fmt.Errorf("list knowledge gaps: %w", err)
	}
	return list, nil
}
