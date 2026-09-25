//go:build server

package servicesummary

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/uptrace/bun"
)

const (
	// historyLimit 是注入 AI 上下文的客户历史小结条数上限。
	historyLimit = 5
	// historyHitLimit 是客户历史检索返回的命中消息条数上限。
	historyHitLimit = 6
	// historyContextMessages 是命中消息前后各带出的对客消息条数。
	historyContextMessages = 2
	// historyMessageMaxRunes 是客户历史检索结果中单条消息的最大字符数。
	historyMessageMaxRunes = 500
)

// closedCustomerSessions 返回与指定周期同一客户的其他已关闭周期，周期别名为 ss。
func closedCustomerSessions(db bun.IDB, organizationID, serviceSessionID string) *bun.SelectQuery {
	return db.NewSelect().
		TableExpr("service_sessions AS cur").
		Join("JOIN contact_channel_identities AS cur_cci ON cur_cci.id = cur.contact_channel_identity_id AND cur_cci.organization_id = cur.organization_id").
		Join("JOIN contact_channel_identities AS cci ON cci.contact_id = cur_cci.contact_id AND cci.organization_id = cur_cci.organization_id").
		Join("JOIN service_sessions AS ss ON ss.contact_channel_identity_id = cci.id AND ss.organization_id = cci.organization_id").
		Where("cur.organization_id = ? AND cur.id = ?", organizationID, serviceSessionID).
		Where("ss.id <> cur.id AND ss.status = ?", domain.ServiceSessionStatusClosed)
}

// RecentHistory 返回与指定周期同一客户的其他已关闭周期中最近几条有正文的小结，按关闭时间从新到旧排列。
func RecentHistory(ctx context.Context, db bun.IDB, organizationID, serviceSessionID string) ([]agentruntime.CustomerHistorySummary, error) {
	rows := make([]struct {
		ClosedAt time.Time `bun:"closed_at"`
		Summary  string    `bun:"summary"`
		Category *string   `bun:"category"`
		Resolved *bool     `bun:"resolved"`
	}, 0, historyLimit)
	if err := closedCustomerSessions(db, organizationID, serviceSessionID).
		ColumnExpr("ss.closed_at, ss.summary, sc.name AS category, ss.resolved").
		Join("LEFT JOIN service_categories AS sc ON sc.id = ss.category_id AND sc.organization_id = ss.organization_id").
		Where("ss.summary_status = ? AND ss.summary IS NOT NULL", domain.ServiceSessionSummaryReady).
		OrderExpr("ss.closed_at DESC, ss.id DESC").
		Limit(historyLimit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load customer service history: %w", err)
	}
	history := make([]agentruntime.CustomerHistorySummary, 0, len(rows))
	for _, row := range rows {
		entry := agentruntime.CustomerHistorySummary{ClosedAt: row.ClosedAt, Summary: row.Summary, Resolved: row.Resolved}
		if row.Category != nil {
			entry.Category = *row.Category
		}
		history = append(history, entry)
	}
	return history, nil
}

// SearchHistory 在与指定周期同一客户的其他已关闭周期中检索对客消息，按命中相关度列出周期小结与命中消息前后的对客消息。
func SearchHistory(ctx context.Context, db bun.IDB, organizationID, serviceSessionID, text string) (agentruntime.CustomerHistoryResult, error) {
	result := agentruntime.CustomerHistoryResult{Sessions: []agentruntime.CustomerHistorySession{}}
	query, searchable := searchtext.ParseKeywords(text)
	if !searchable {
		result.Message = "检索词没有可检索的内容，请改用订单号、商品名或问题关键词。"
		return result, nil
	}
	tsquery := query.TSQuery()
	var hits []struct {
		SessionID string `bun:"session_id"`
		MessageID string `bun:"message_id"`
	}
	if err := closedCustomerSessions(db, organizationID, serviceSessionID).
		ColumnExpr("ss.id::text AS session_id, msg.id::text AS message_id").
		Join("JOIN messages AS msg ON msg.organization_id = ss.organization_id AND msg.conversation_id = ss.conversation_id AND msg.service_session_id = ss.id").
		Where("msg.deleted_at IS NULL AND msg.visibility = ?", domain.MessageVisibilityCustomerVisible).
		Where("msg.type IN (?)", bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAttachment})).
		Where("msg.search_vector @@ ?::tsquery", tsquery).
		OrderExpr("ts_rank_cd(msg.search_vector, ?::tsquery) DESC, msg.originated_at DESC, msg.id", tsquery).
		Limit(historyHitLimit).
		Scan(ctx, &hits); err != nil {
		return result, fmt.Errorf("search customer history messages: %w", err)
	}
	if len(hits) == 0 {
		result.Message = "没有找到相关的以往沟通记录。"
		return result, nil
	}
	// 周期按其最相关命中的顺序排列。
	var sessionIDs, hitIDs []string
	for _, hit := range hits {
		hitIDs = append(hitIDs, hit.MessageID)
		if !slices.Contains(sessionIDs, hit.SessionID) {
			sessionIDs = append(sessionIDs, hit.SessionID)
		}
	}
	var sessions []struct {
		ID       string    `bun:"id"`
		ClosedAt time.Time `bun:"closed_at"`
		Summary  *string   `bun:"summary"`
		Category *string   `bun:"category"`
		Resolved *bool     `bun:"resolved"`
	}
	if err := db.NewSelect().TableExpr("service_sessions AS ss").
		ColumnExpr("ss.id::text AS id, ss.closed_at, sc.name AS category, ss.resolved").
		ColumnExpr("CASE WHEN ss.summary_status = ? THEN ss.summary END AS summary", domain.ServiceSessionSummaryReady).
		Join("LEFT JOIN service_categories AS sc ON sc.id = ss.category_id AND sc.organization_id = ss.organization_id").
		Where("ss.organization_id = ? AND ss.id IN (?)", organizationID, bun.In(sessionIDs)).
		Scan(ctx, &sessions); err != nil {
		return result, fmt.Errorf("load customer history sessions: %w", err)
	}
	var messages []struct {
		ID           string    `bun:"id"`
		SessionID    string    `bun:"session_id"`
		Sender       string    `bun:"sender"`
		Body         string    `bun:"body"`
		Attachment   *string   `bun:"attachment"`
		OriginatedAt time.Time `bun:"originated_at"`
	}
	if err := db.NewSelect().TableExpr("messages AS msg").
		ColumnExpr("msg.id::text AS id, msg.service_session_id::text AS session_id, msg.originated_at, msg.body, ma.name AS attachment").
		ColumnExpr("CASE WHEN cs.kind = ? THEN 'customer' WHEN oi.type = ? THEN 'agent' ELSE 'member' END AS sender",
			domain.ChatSubjectKindContact, domain.OrganizationIdentityTypeAgent).
		Join("LEFT JOIN message_attachments AS ma ON ma.organization_id = msg.organization_id AND ma.message_id = msg.id").
		Join("LEFT JOIN conversation_participants AS cp ON cp.id = msg.sender_participant_id AND cp.organization_id = msg.organization_id AND cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS cs ON cs.id = cp.subject_id AND cs.organization_id = cp.organization_id").
		Join("LEFT JOIN organization_identities AS oi ON oi.id = cs.source_id AND oi.organization_id = cs.organization_id AND cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Where("msg.organization_id = ? AND msg.service_session_id IN (?)", organizationID, bun.In(sessionIDs)).
		Where("msg.deleted_at IS NULL AND msg.visibility = ?", domain.MessageVisibilityCustomerVisible).
		Where("msg.type IN (?)", bun.In([]domain.MessageType{domain.MessageTypeText, domain.MessageTypeAttachment})).
		OrderExpr("msg.message_seq").
		Scan(ctx, &messages); err != nil {
		return result, fmt.Errorf("load customer history messages: %w", err)
	}
	positions := make(map[string]int, len(sessions))
	for index, session := range sessions {
		positions[session.ID] = index
	}
	for _, id := range sessionIDs {
		row := sessions[positions[id]]
		session := agentruntime.CustomerHistorySession{
			CustomerHistorySummary: agentruntime.CustomerHistorySummary{ClosedAt: row.ClosedAt, Resolved: row.Resolved},
			Messages:               []agentruntime.CustomerHistoryMessage{},
		}
		if row.Summary != nil {
			session.Summary = *row.Summary
		}
		if row.Category != nil {
			session.Category = *row.Category
		}
		// 按周期内对客消息的先后计数，保留命中消息及其前后各若干条。
		var timeline []int
		for index, message := range messages {
			if message.SessionID == id {
				timeline = append(timeline, index)
			}
		}
		keep := make([]bool, len(timeline))
		for position, index := range timeline {
			if !slices.Contains(hitIDs, messages[index].ID) {
				continue
			}
			for near := max(0, position-historyContextMessages); near <= min(len(timeline)-1, position+historyContextMessages); near++ {
				keep[near] = true
			}
		}
		for position, index := range timeline {
			if !keep[position] {
				continue
			}
			message := messages[index]
			item := agentruntime.CustomerHistoryMessage{
				Sender: message.Sender, Body: query.Window(message.Body, historyMessageMaxRunes), SentAt: message.OriginatedAt,
			}
			if message.Attachment != nil {
				item.Attachment = *message.Attachment
			}
			session.Messages = append(session.Messages, item)
		}
		result.Sessions = append(result.Sessions, session)
	}
	return result, nil
}
