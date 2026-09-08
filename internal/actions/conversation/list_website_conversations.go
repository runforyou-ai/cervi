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
	LastMessageSeq            int64                            `bun:"last_message_seq"`
	ID                        string                           `bun:"id"`
	Title                     string                           `bun:"title"`
	LastMessageAt             time.Time                        `bun:"last_message_at"`
	Preview                   string                           `bun:"preview"`
	PreviewSenderIdentityType *domain.OrganizationIdentityType `bun:"preview_sender_identity_type"`
	ServiceSessionID          string                           `bun:"service_session_id"`
	ServiceSessionStatus      string                           `bun:"service_session_status"`
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
	var rows []conversationSummaryRow
	err = q.db.NewSelect().
		TableExpr("customer_conversations AS cc").
		ColumnExpr("cv.id AS id").
		ColumnExpr("cv.title AS title").
		ColumnExpr("msg.originated_at AS last_message_at").
		ColumnExpr("msg.message_seq AS last_message_seq").
		ColumnExpr("msg.body AS preview").
		ColumnExpr("preview_oi.type AS preview_sender_identity_type").
		ColumnExpr("current.id AS service_session_id").
		ColumnExpr("current.status AS service_session_status").
		Join("JOIN conversations AS cv ON cv.id = cc.conversation_id AND cv.organization_id = cc.organization_id").
		Join(`JOIN LATERAL (
 SELECT visible.* FROM messages AS visible
 WHERE visible.organization_id = cv.organization_id AND visible.conversation_id = cv.id AND visible.type = ? AND visible.deleted_at IS NULL
 ORDER BY visible.message_seq DESC LIMIT 1
 ) AS msg ON TRUE`, domain.MessageTypeText).
		Join("LEFT JOIN conversation_participants AS preview_cp ON preview_cp.id = msg.sender_participant_id AND preview_cp.organization_id = msg.organization_id AND preview_cp.conversation_id = msg.conversation_id").
		Join("LEFT JOIN chat_subjects AS preview_cs ON preview_cs.id = preview_cp.subject_id AND preview_cs.organization_id = preview_cp.organization_id").
		Join("LEFT JOIN organization_identities AS preview_oi ON preview_oi.id = preview_cs.source_id AND preview_oi.organization_id = preview_cs.organization_id AND preview_cs.kind = ?", domain.ChatSubjectKindOrganizationIdentity).
		Join("JOIN service_sessions AS current ON current.organization_id = cc.organization_id AND current.conversation_id = cc.conversation_id AND current.id = cc.current_service_session_id").
		Where("cc.organization_id = ?", channel.OrganizationID).
		Where("cc.contact_channel_identity_id = ?", identity.ID).
		Where("current.contact_channel_identity_id = ?", identity.ID).
		Where("cv.type = ?", domain.ConversationTypeCustomer).
		Where("cv.status IN (?, ?)", domain.ConversationStatusActive, domain.ConversationStatusArchived).
		OrderExpr("msg.originated_at DESC, cv.id DESC").
		Limit(20).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list website conversations: %w", err)
	}
	result := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, conversationSummaryFromRow(row))
	}
	return result, nil
}

// conversationSummaryFromRow 转换网站访客会话摘要。
func conversationSummaryFromRow(row conversationSummaryRow) ConversationSummary {
	return ConversationSummary{
		ID: row.ID, Title: row.Title, Preview: row.Preview, PreviewSenderIdentityType: row.PreviewSenderIdentityType, LastMessageSeq: row.LastMessageSeq, LastMessageAt: row.LastMessageAt,
		ServiceSessionID: row.ServiceSessionID, ServiceSessionStatus: domain.ServiceSessionStatus(row.ServiceSessionStatus),
	}
}
