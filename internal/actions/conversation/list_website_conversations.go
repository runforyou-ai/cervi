//go:build server

package conversation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListWebsiteConversationsQuery 读取网站访客的客户会话列表。
type ListWebsiteConversationsQuery struct {
	db *bun.DB
}

type conversationSummaryRow struct {
	ID                       string     `bun:"id"`
	Title                    *string    `bun:"title"`
	LastMessageID            *string    `bun:"last_message_id"`
	LastMessageAt            *time.Time `bun:"last_message_at"`
	Preview                  *string    `bun:"preview"`
	PreviewOriginatedAt      *time.Time `bun:"preview_originated_at"`
	ServiceSessionID         *string    `bun:"service_session_id"`
	ServiceSessionStatus     *string    `bun:"service_session_status"`
	ServiceSessionIdentityID *string    `bun:"service_session_identity_id"`
}

// NewListWebsiteConversationsQuery 创建网站访客会话列表查询。
func NewListWebsiteConversationsQuery(db *bun.DB) *ListWebsiteConversationsQuery {
	return &ListWebsiteConversationsQuery{db: db}
}

// Execute 返回当前网站渠道身份最近的客户会话。
func (q *ListWebsiteConversationsQuery) Execute(ctx context.Context, channelID, externalID string) ([]ConversationSummary, error) {
	fields := map[string]ValidationCode{}
	if !common.ValidUUID(channelID) {
		fields["channelId"] = ValidationChannelIDInvalid
	}
	if !validWebsiteExternalID(externalID) {
		fields["visitorToken"] = ValidationExternalIDInvalid
	}
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	channel, err := loadWebsiteChannel(ctx, q.db, channelID)
	if err != nil {
		return nil, err
	}
	identity := &servermodels.ContactChannelIdentity{}
	err = q.db.NewSelect().Model(identity).
		Where("cci.organization_id = ?", channel.OrganizationID).
		Where("cci.channel_id = ?", channel.ID).
		Where("cci.external_id = ?", externalID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return []ConversationSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load website conversation identity: %w", err)
	}
	contactExists, err := q.db.NewSelect().Model((*servermodels.Contact)(nil)).
		Where("organization_id = ?", channel.OrganizationID).
		Where("id = ?", identity.ContactID).
		Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check website conversation contact: %w", err)
	}
	if !contactExists {
		return nil, ErrDataInvariant
	}

	var rows []conversationSummaryRow
	err = q.db.NewSelect().
		TableExpr("customer_conversations AS cc").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("cv.last_message_id AS last_message_id").
		ColumnExpr("cv.last_message_at AS last_message_at").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("msg.originated_at AS preview_originated_at").
		ColumnExpr("latest.id AS service_session_id").
		ColumnExpr("latest.status AS service_session_status").
		ColumnExpr("latest.contact_channel_identity_id AS service_session_identity_id").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id").
		Join("LEFT JOIN messages AS msg ON msg.id = cv.last_message_id AND msg.organization_id = cv.organization_id AND msg.conversation_id = cv.id AND msg.deleted_at IS NULL").
		Join("LEFT JOIN LATERAL (SELECT ss.id, ss.status, ss.contact_channel_identity_id FROM service_sessions AS ss WHERE ss.organization_id = cv.organization_id AND ss.conversation_id = cv.id ORDER BY ss.sequence DESC LIMIT 1) AS latest ON TRUE").
		Where("cc.organization_id = ?", channel.OrganizationID).
		Where("cc.contact_channel_identity_id = ?", identity.ID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
		OrderExpr("cv.last_message_at DESC NULLS LAST, cv.id DESC").
		Limit(20).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list website conversations: %w", err)
	}
	result := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		summary, err := conversationSummaryFromRow(row, identity.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, summary)
	}
	return result, nil
}

// conversationSummaryFromRow 校验并转换网站访客会话摘要。
func conversationSummaryFromRow(row conversationSummaryRow, channelIdentityID string) (ConversationSummary, error) {
	if row.Title == nil || row.LastMessageID == nil || row.LastMessageAt == nil || row.Preview == nil || row.PreviewOriginatedAt == nil || !row.LastMessageAt.Equal(*row.PreviewOriginatedAt) ||
		row.ServiceSessionID == nil || row.ServiceSessionStatus == nil || row.ServiceSessionIdentityID == nil ||
		*row.ServiceSessionIdentityID != channelIdentityID {
		return ConversationSummary{}, ErrDataInvariant
	}
	status := domain.ServiceSessionStatus(*row.ServiceSessionStatus)
	if status != domain.ServiceSessionStatusWaiting && status != domain.ServiceSessionStatusActive && status != domain.ServiceSessionStatusPending && status != domain.ServiceSessionStatusClosed {
		return ConversationSummary{}, ErrDataInvariant
	}
	return ConversationSummary{
		ID: row.ID, Title: *row.Title, Preview: *row.Preview, LastMessageAt: *row.LastMessageAt,
		ServiceSessionID: *row.ServiceSessionID, ServiceSessionStatus: status,
	}, nil
}
