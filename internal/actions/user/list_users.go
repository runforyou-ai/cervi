//go:build server

// Package user 实现企业内部用户目录查询。
package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListUsersQuery 读取当前企业的内部用户列表。
type ListUsersQuery struct {
	db *bun.DB
}

// NewListUsersQuery 创建企业成员列表查询。
func NewListUsersQuery(db *bun.DB) *ListUsersQuery {
	return &ListUsersQuery{db: db}
}

// Execute 返回满足条件的企业成员分页列表。
func (q *ListUsersQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ListInput) (ListOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = domain.UserStatus(strings.TrimSpace(string(input.Status)))
	input.Role = domain.UserRole(strings.TrimSpace(string(input.Role)))
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 50
	}
	if input.PageSize > 100 ||
		(input.Status != "" && input.Status != domain.UserStatusActive && input.Status != domain.UserStatusInactive) ||
		(input.Role != "" && input.Role != domain.UserRoleOwner && input.Role != domain.UserRoleMember) {
		return ListOutput{}, ErrQueryInvalid
	}

	applyFilters := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where("u.organization_id = ?", identity.Organization.ID)
		if input.Status != "" {
			query = query.Where("u.status = ?", input.Status)
		}
		if input.Role != "" {
			query = query.Where("u.role = ?", input.Role)
		}
		if input.Query != "" {
			pattern := "%" + input.Query + "%"
			query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				return group.
					Where("u.display_name ILIKE ?", pattern).
					WhereOr("u.email ILIKE ?", pattern)
			})
		}
		return query
	}

	total, err := applyFilters(q.db.NewSelect().Model((*servermodels.User)(nil))).Count(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("count users: %w", err)
	}
	users := make([]DirectoryUser, 0)
	if err := applyFilters(q.db.NewSelect().TableExpr("users AS u")).
		ColumnExpr("u.id::text AS id").
		Column("email", "display_name", "role", "status", "created_at").
		OrderExpr("lower(u.display_name) ASC, u.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &users); err != nil {
		return ListOutput{}, fmt.Errorf("list users: %w", err)
	}
	return ListOutput{Users: users, Page: PageInfo{Number: input.Page, Size: input.PageSize, Total: total}}, nil
}
