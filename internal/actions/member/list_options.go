//go:build server

// Package member 实现企业成员公共查询。
package member

import (
	"context"
	"fmt"
	"strings"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Option 定义可选择的企业成员。
type Option struct {
	ID           string                    `bun:"id"`
	Type         domain.MemberIdentityType `bun:"type"`
	DisplayName  string                    `bun:"display_name"`
	AvatarFileID *string                   `bun:"avatar_file_id"`
}

// ListOptionsInput 定义成员选择项查询条件。
type ListOptionsInput struct {
	Query    string
	Page     int
	PageSize int
}

// ListOptionsOutput 定义成员选择项分页结果。
type ListOptionsOutput struct {
	Members []Option
	Page    int
	Size    int
	Total   int
}

// ListOptionsQuery 读取可被分配的企业成员。
type ListOptionsQuery struct{ db *bun.DB }

// NewListOptionsQuery 创建企业成员选择项查询。
func NewListOptionsQuery(db *bun.DB) *ListOptionsQuery { return &ListOptionsQuery{db: db} }

// Execute 返回当前企业中可分配的企业成员和 AI 员工。
func (q *ListOptionsQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ListOptionsInput) (ListOptionsOutput, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return ListOptionsOutput{}, err
	}
	input.Query = strings.TrimSpace(input.Query)
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 50
	}
	if input.PageSize > 100 {
		return ListOptionsOutput{}, fmt.Errorf("member options page size invalid")
	}
	apply := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where("om.organization_id = ?", identity.Organization.ID).
			Where("om.status = ?", domain.UserStatusActive).
			Where("om.type IN (?, ?)", domain.MemberIdentityTypeUser, domain.MemberIdentityTypeAgent)
		if input.Query != "" {
			pattern := "%" + input.Query + "%"
			query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				return group.Where("om.display_name ILIKE ?", pattern).WhereOr("u.email ILIKE ?", pattern)
			})
		}
		return query
	}
	base := func() *bun.SelectQuery {
		return q.db.NewSelect().TableExpr("organization_members AS om").
			Join("LEFT JOIN users AS u ON u.id = om.id AND u.organization_id = om.organization_id")
	}
	total, err := apply(base()).Count(ctx)
	if err != nil {
		return ListOptionsOutput{}, fmt.Errorf("count member options: %w", err)
	}
	members := make([]Option, 0)
	if err := apply(base()).
		ColumnExpr("om.id::text, om.type, om.display_name, om.avatar_file_id::text").
		OrderExpr("lower(om.display_name) ASC, om.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &members); err != nil {
		return ListOptionsOutput{}, fmt.Errorf("list member options: %w", err)
	}
	return ListOptionsOutput{Members: members, Page: input.Page, Size: input.PageSize, Total: total}, nil
}
