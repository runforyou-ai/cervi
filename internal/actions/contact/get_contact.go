//go:build server

package contact

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetContactQuery 读取当前企业的联系人详情。
type GetContactQuery struct {
	db *bun.DB
}

// NewGetContactQuery 创建联系人详情查询。
func NewGetContactQuery(db *bun.DB) *GetContactQuery {
	return &GetContactQuery{db: db}
}

// Execute 返回当前企业中未删除的联系人详情。
func (q *GetContactQuery) Execute(ctx context.Context, principal *servermodels.Principal, contactID string) (*ContactDetail, error) {
	contact, err := loadContact(ctx, q.db, principal.Organization.ID, contactID)
	if err != nil {
		return nil, err
	}
	sourceChannel := SourceChannel{}
	if err := q.db.NewSelect().
		TableExpr("channels AS ch").
		ColumnExpr("ch.id::text AS id").
		Column("type", "name").
		Where("ch.organization_id = ?", principal.Organization.ID).
		Where("ch.id = ?", contact.SourceChannelID).
		Scan(ctx, &sourceChannel); err != nil {
		return nil, fmt.Errorf("read contact source channel: %w", err)
	}

	methods := make([]ContactMethod, 0)
	if err := q.db.NewSelect().
		TableExpr("contact_methods AS cm").
		Column("type", "value", "label", "is_primary").
		Where("cm.organization_id = ?", principal.Organization.ID).
		Where("cm.contact_id = ?", contactID).
		OrderExpr("cm.type ASC, cm.is_primary DESC, cm.created_at ASC").
		Scan(ctx, &methods); err != nil {
		return nil, fmt.Errorf("list contact methods: %w", err)
	}

	identities := make([]ChannelIdentity, 0)
	if err := q.db.NewSelect().
		TableExpr("contact_channel_identities AS cci").
		ColumnExpr("cci.channel_id::text AS channel_id").
		ColumnExpr("ch.name AS channel_name").
		ColumnExpr("cci.external_id").
		ColumnExpr("cci.display_name").
		Join("JOIN channels AS ch ON ch.id = cci.channel_id AND ch.organization_id = cci.organization_id").
		Where("cci.organization_id = ?", principal.Organization.ID).
		Where("cci.contact_id = ?", contactID).
		OrderExpr("cci.updated_at DESC, cci.id DESC").
		Scan(ctx, &identities); err != nil {
		return nil, fmt.Errorf("list contact channel identities: %w", err)
	}

	return &ContactDetail{Contact: *contact, SourceChannel: sourceChannel, Methods: methods, ChannelIdentities: identities}, nil
}
