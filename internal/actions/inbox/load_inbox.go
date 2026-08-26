//go:build server

// Package inbox 实现统一收件箱领域的应用查询。
package inbox

import (
	"context"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ConversationSummary 定义成员收件箱中的客户会话摘要。
type ConversationSummary struct {
	ID                   string
	Title                string
	ContactName          *string
	ChannelType          domain.ChannelType
	ChannelName          string
	Preview              string
	LastMessageAt        time.Time
	ServiceSessionStatus domain.ServiceSessionStatus
}

// LoadInboxQuery 读取当前企业的客户会话工作队列。
type LoadInboxQuery struct {
	db *bun.DB
}

type inboxConversationRow struct {
	ID                   string    `bun:"id"`
	Title                string    `bun:"title"`
	ContactName          *string   `bun:"contact_name"`
	ChannelType          string    `bun:"channel_type"`
	ChannelName          string    `bun:"channel_name"`
	Preview              string    `bun:"preview"`
	LastMessageAt        time.Time `bun:"last_message_at"`
	ServiceSessionStatus string    `bun:"service_session_status"`
}

// NewLoadInboxQuery 创建成员收件箱查询。
func NewLoadInboxQuery(db *bun.DB) *LoadInboxQuery {
	return &LoadInboxQuery{db: db}
}

// Execute 按企业边界返回最新处理批次未结束的客户会话。
func (q *LoadInboxQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]ConversationSummary, error) {
	var rows []inboxConversationRow
	err := q.db.NewSelect().
		TableExpr("customer_conversations AS cc").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("COALESCE(cci.display_name, c.display_name) AS contact_name").
		ColumnExpr("ch.type AS channel_type").
		ColumnExpr("ch.name AS channel_name").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("latest.status AS service_session_status").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id").
		Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
		Join("JOIN contacts AS c ON c.id = cci.contact_id AND c.organization_id = cc.organization_id").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cc.organization_id").
		Join("JOIN messages AS msg ON msg.id = cv.last_message_id AND msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.deleted_at IS NULL").
		Join("JOIN LATERAL (SELECT ss.status FROM service_sessions AS ss WHERE ss.organization_id = cv.organization_id AND ss.conversation_id = cv.id ORDER BY ss.sequence DESC LIMIT 1) AS latest ON TRUE").
		Where("cc.organization_id = ?", identity.Organization.ID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Where("latest.status IN (?)", bun.In([]domain.ServiceSessionStatus{
			domain.ServiceSessionStatusWaiting,
			domain.ServiceSessionStatusActive,
			domain.ServiceSessionStatusPending,
		})).
		OrderExpr("cv.last_message_at DESC NULLS LAST, cv.id DESC").
		Limit(50).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list inbox conversations: %w", err)
	}
	result := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, ConversationSummary{
			ID:                   row.ID,
			Title:                row.Title,
			ContactName:          row.ContactName,
			ChannelType:          domain.ChannelType(row.ChannelType),
			ChannelName:          row.ChannelName,
			Preview:              row.Preview,
			LastMessageAt:        row.LastMessageAt,
			ServiceSessionStatus: domain.ServiceSessionStatus(row.ServiceSessionStatus),
		})
	}
	return result, nil
}
