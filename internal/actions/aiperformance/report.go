//go:build server

package aiperformance

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// reportScopeSQL 定义报表的两个公共集合，%s 处拼入可选的渠道条件。
// closed 为统计范围内已关闭的周期；ai_resolved 为结束方式是 AI 解决，且周期内没有转人工、退回队列、真人领取、接管、转交或真人对客回复。
// handoffs 为这些周期内的转人工事件：AI 主动转人工取事件记录的原因，AI 员工负责时被退回队列记为 AI 员工不可用。
const reportScopeSQL = `
WITH closed AS (
	SELECT ss.id, ss.organization_id, ss.close_reason, ss.category_id, ss.rating_resolved, cci.channel_id,
		ss.close_reason = ? AND NOT EXISTS (
			SELECT 1 FROM messages m
			LEFT JOIN conversation_participants cp ON cp.id = m.sender_participant_id
			LEFT JOIN chat_subjects cs ON cs.id = cp.subject_id AND cs.kind = ?
			LEFT JOIN organization_identities oi ON oi.id = cs.source_id
			WHERE m.organization_id = ss.organization_id AND m.service_session_id = ss.id
				AND (m.system_event_type IN (?)
					OR (m.visibility = ? AND m.type IN (?) AND oi.type = ?))
		) AS ai_resolved
	FROM service_sessions ss
	JOIN contact_channel_identities cci ON cci.id = ss.contact_channel_identity_id
	WHERE ss.organization_id = ? AND ss.status = ? AND ss.closed_at >= now() - make_interval(days => ?)%s
),
handoffs AS (
	SELECT m.id, m.organization_id, m.conversation_id, m.service_session_id, m.message_seq, m.created_at AS occurred_at,
		CASE WHEN m.system_event_type = ? THEN m.system_event_payload->>'reason' ELSE ? END AS reason
	FROM closed
	JOIN messages m ON m.organization_id = closed.organization_id AND m.service_session_id = closed.id
	LEFT JOIN organization_identities oi ON oi.organization_id = m.organization_id
		AND m.system_event_type = ? AND oi.id = (m.system_event_payload->>'fromIdentityId')::uuid
	WHERE m.system_event_type = ? OR (m.system_event_type = ? AND oi.type = ?)
)`

// ReportQuery 读取当前企业的 AI 表现报表。
type ReportQuery struct{ db *bun.DB }

// NewReportQuery 创建 AI 表现报表查询。
func NewReportQuery(db *bun.DB) *ReportQuery { return &ReportQuery{db: db} }

// Execute 按统计范围汇总已关闭周期，以及这些周期内的转人工原因和知识缺口。
func (q *ReportQuery) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Report, error) {
	handedOff, returned := domain.ConversationSystemEventServiceSessionHandedOff, domain.ConversationSystemEventServiceSessionReturned
	scopeArgs := []any{
		domain.ServiceSessionCloseAIResolved, domain.ChatSubjectKindOrganizationIdentity,
		bun.In([]domain.ConversationSystemEventType{
			handedOff, returned, domain.ConversationSystemEventServiceSessionClaimed,
			domain.ConversationSystemEventServiceSessionTakenOver, domain.ConversationSystemEventServiceSessionTransferred,
		}),
		domain.MessageVisibilityCustomerVisible, bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAttachment}),
		domain.OrganizationIdentityTypeUser,
		identity.Organization.ID, domain.ServiceSessionStatusClosed, input.Days,
	}
	// 指定渠道时追加渠道条件，全部渠道时不比较。
	channelClause := ""
	if input.ChannelID != "" {
		channelClause = " AND cci.channel_id = ?"
		scopeArgs = append(scopeArgs, input.ChannelID)
	}
	scopeArgs = append(scopeArgs, handedOff, domain.AgentHandoffReasonAgentUnavailable, returned, handedOff, returned, domain.OrganizationIdentityTypeAgent)
	scope := fmt.Sprintf(reportScopeSQL, channelClause)
	// withArgs 返回公共集合参数与本条查询参数拼接后的新切片。
	withArgs := func(args ...any) []any { return append(append([]any{}, scopeArgs...), args...) }
	gapReasons := bun.In([]domain.AgentHandoffReason{domain.AgentHandoffReasonKnowledgeGap, domain.AgentHandoffReasonInsufficientEvidence})
	report := &Report{Channels: []Breakdown{}, Categories: []Breakdown{}, HandoffReasons: []ReasonCount{}, KnowledgeGaps: []KnowledgeGap{}}

	if err := q.db.NewRaw(scope+`
SELECT count(*) AS closed,
	count(*) FILTER (WHERE ai_resolved) AS ai_resolved,
	(SELECT count(DISTINCT service_session_id) FROM handoffs) AS handed_off,
	count(*) FILTER (WHERE close_reason = ?) AS close_ai_resolved,
	count(*) FILTER (WHERE close_reason = ?) AS customer_unresponsive,
	count(*) FILTER (WHERE close_reason = ?) AS manual,
	count(rating_resolved) AS rated,
	count(*) FILTER (WHERE rating_resolved) AS rated_resolved
FROM closed`, withArgs(domain.ServiceSessionCloseAIResolved, domain.ServiceSessionCloseCustomerUnresponsive, domain.ServiceSessionCloseManual)...).
		Scan(ctx, &report.Summary); err != nil {
		return nil, fmt.Errorf("summarize closed service sessions: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT c.id, c.name, count(*) AS closed, count(*) FILTER (WHERE closed.ai_resolved) AS ai_resolved
FROM closed JOIN channels c ON c.id = closed.channel_id
GROUP BY c.id, c.name
ORDER BY 3 DESC, c.name ASC`, withArgs()...).Scan(ctx, &report.Channels); err != nil {
		return nil, fmt.Errorf("break down closed service sessions by channel: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT sc.id, coalesce(sc.name, '') AS name, count(*) AS closed, count(*) FILTER (WHERE closed.ai_resolved) AS ai_resolved
FROM closed LEFT JOIN service_categories sc ON sc.id = closed.category_id
GROUP BY sc.id, sc.name
ORDER BY 3 DESC, sc.name ASC NULLS LAST`, withArgs()...).Scan(ctx, &report.Categories); err != nil {
		return nil, fmt.Errorf("break down closed service sessions by category: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT reason, count(*) AS count
FROM handoffs
GROUP BY reason
ORDER BY count DESC, reason ASC`, withArgs()...).Scan(ctx, &report.HandoffReasons); err != nil {
		return nil, fmt.Errorf("count handoff reasons: %w", err)
	}
	if err := q.db.NewRaw(scope+`
SELECT count(*) FROM handoffs WHERE reason IN (?)`, withArgs(gapReasons)...).Scan(ctx, &report.KnowledgeGapTotal); err != nil {
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
LIMIT ?`, withArgs(domain.MessageTypeText, domain.ChatSubjectKindContact, gapReasons, KnowledgeGapLimit)...).Scan(ctx, &report.KnowledgeGaps); err != nil {
		return nil, fmt.Errorf("list knowledge gaps: %w", err)
	}
	return report, nil
}
