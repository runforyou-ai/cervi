//go:build server

package contact

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListContactsQuery 读取当前企业的外部联系人列表。
type ListContactsQuery struct {
	db *bun.DB
}

// NewListContactsQuery 创建联系人列表查询。
func NewListContactsQuery(db *bun.DB) *ListContactsQuery {
	return &ListContactsQuery{db: db}
}

// Execute 返回满足查询条件的分页联系人列表。
func (q *ListContactsQuery) Execute(ctx context.Context, principal *servermodels.Principal, input ListInput) (ListOutput, error) {
	input, fields := normalizeListInput(input)
	if len(fields) > 0 {
		return ListOutput{}, &ValidationError{Fields: fields}
	}

	countQuery := applyContactFilters(q.db.NewSelect().TableExpr("contacts AS c"), principal.Organization.ID, input)
	total, err := countQuery.Count(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("count contacts: %w", err)
	}

	contacts := make([]ContactSummary, 0)
	query := applyContactFilters(q.db.NewSelect().TableExpr("contacts AS c"), principal.Organization.ID, input).
		ColumnExpr("c.id::text AS id").
		ColumnExpr("c.display_name").
		ColumnExpr("c.stage").
		ColumnExpr("c.created_at").
		ColumnExpr("c.deleted_at").
		ColumnExpr("source_channel.name AS source_channel_name").
		ColumnExpr("(SELECT cm.value FROM contact_methods AS cm WHERE cm.organization_id = c.organization_id AND cm.contact_id = c.id AND cm.type = 'email' ORDER BY cm.is_primary DESC, cm.created_at ASC LIMIT 1) AS primary_email").
		ColumnExpr("(SELECT cm.value FROM contact_methods AS cm WHERE cm.organization_id = c.organization_id AND cm.contact_id = c.id AND cm.type = 'phone' ORDER BY cm.is_primary DESC, cm.created_at ASC LIMIT 1) AS primary_phone").
		Join("JOIN channels AS source_channel ON source_channel.id = c.source_channel_id AND source_channel.organization_id = c.organization_id")
	switch input.Sort {
	case SortCreatedAtDescending:
		query = query.OrderExpr("c.created_at DESC, c.id DESC")
	case SortDisplayNameAscending:
		query = query.OrderExpr("lower(coalesce(c.display_name, '')) ASC, c.id ASC")
	default:
		query = query.OrderExpr("c.updated_at DESC, c.id DESC")
	}
	if err := query.
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &contacts); err != nil {
		return ListOutput{}, fmt.Errorf("list contacts: %w", err)
	}
	return ListOutput{
		Contacts: contacts,
		Page:     PageInfo{Number: input.Page, Size: input.PageSize, Total: total},
	}, nil
}

// applyContactFilters 添加组织边界和联系人筛选条件。
func applyContactFilters(query *bun.SelectQuery, organizationID string, input ListInput) *bun.SelectQuery {
	query = query.Where("c.organization_id = ?", organizationID)
	if input.Deleted {
		query = query.Where("c.deleted_at IS NOT NULL")
	} else {
		query = query.Where("c.deleted_at IS NULL")
	}
	if input.Stage != "" {
		query = query.Where("c.stage = ?", input.Stage)
	}
	if input.ChannelID != "" {
		query = query.Where("(c.source_channel_id = ? OR EXISTS (SELECT 1 FROM contact_channel_identities AS cci WHERE cci.organization_id = c.organization_id AND cci.contact_id = c.id AND cci.channel_id = ?))", input.ChannelID, input.ChannelID)
	}
	if input.MethodType != "" {
		query = query.Where("EXISTS (SELECT 1 FROM contact_methods AS cm WHERE cm.organization_id = c.organization_id AND cm.contact_id = c.id AND cm.type = ?)", input.MethodType)
	}
	if input.Query != "" {
		pattern := "%" + input.Query + "%"
		query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
			return group.
				Where("coalesce(c.display_name, '') ILIKE ?", pattern).
				WhereOr("EXISTS (SELECT 1 FROM contact_methods AS cm WHERE cm.organization_id = c.organization_id AND cm.contact_id = c.id AND cm.normalized_value ILIKE ?)", pattern).
				WhereOr("EXISTS (SELECT 1 FROM contact_channel_identities AS cci WHERE cci.organization_id = c.organization_id AND cci.contact_id = c.id AND coalesce(cci.display_name, '') ILIKE ?)", pattern)
		})
	}
	return query
}
