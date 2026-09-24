//go:build server

package aiperformance

import (
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// reportScopeSQL 定义报表的两个公共集合，%s 处拼入可选的渠道条件。
// closed 为统计范围内已关闭的周期，排除小结状态为无实质诉求的周期；resolved 取小结的是否解决，为空记为未判定；
// ai_only 表示周期由 AI 员工关闭，且周期内没有转人工、退回队列、真人领取、接管、转交或真人对客回复。
// handoffs 为这些周期内的转人工事件：AI 主动转人工取事件记录的原因，AI 员工负责时被退回队列记为 AI 员工不可用。
const reportScopeSQL = `
WITH closed AS (
	SELECT ss.id, ss.organization_id, ss.close_reason, ss.category_id, ss.rating_resolved, ss.resolved, cci.channel_id,
		coalesce(closer.type = ?, false) AND NOT EXISTS (
			SELECT 1 FROM messages m
			LEFT JOIN conversation_participants cp ON cp.id = m.sender_participant_id
			LEFT JOIN chat_subjects cs ON cs.id = cp.subject_id AND cs.kind = ?
			LEFT JOIN organization_identities oi ON oi.id = cs.source_id
			WHERE m.organization_id = ss.organization_id AND m.service_session_id = ss.id
				AND (m.system_event_type IN (?)
					OR (m.visibility = ? AND m.type IN (?) AND oi.type = ?))
		) AS ai_only
	FROM service_sessions ss
	JOIN contact_channel_identities cci ON cci.id = ss.contact_channel_identity_id
	LEFT JOIN organization_identities closer ON closer.id = ss.closed_by_identity_id
	WHERE ss.organization_id = ? AND ss.status = ? AND ss.summary_status IS DISTINCT FROM ?
		AND ss.closed_at >= now() - make_interval(days => ?)%s
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

// reportScope 返回拼好渠道条件的公共集合 SQL 与参数，调用方在其后追加本条查询。
func reportScope(identity *servermodels.Identity, input Input) (string, []any) {
	handedOff, returned := domain.ConversationSystemEventServiceSessionHandedOff, domain.ConversationSystemEventServiceSessionReturned
	args := []any{
		domain.OrganizationIdentityTypeAgent, domain.ChatSubjectKindOrganizationIdentity,
		bun.In([]domain.ConversationSystemEventType{
			handedOff, returned, domain.ConversationSystemEventServiceSessionClaimed,
			domain.ConversationSystemEventServiceSessionTakenOver, domain.ConversationSystemEventServiceSessionTransferred,
		}),
		domain.MessageVisibilityCustomerVisible, bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAttachment}),
		domain.OrganizationIdentityTypeUser,
		identity.Organization.ID, domain.ServiceSessionStatusClosed, domain.ServiceSessionSummaryNoRequest, input.Days,
	}
	// 指定渠道时追加渠道条件，全部渠道时不比较。
	channelClause := ""
	if input.ChannelID != "" {
		channelClause = " AND cci.channel_id = ?"
		args = append(args, input.ChannelID)
	}
	args = append(args, handedOff, domain.AgentHandoffReasonAgentUnavailable, returned, handedOff, returned, domain.OrganizationIdentityTypeAgent)
	return fmt.Sprintf(reportScopeSQL, channelClause), args
}
