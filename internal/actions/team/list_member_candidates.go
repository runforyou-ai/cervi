//go:build server

package team

import (
	"context"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListMemberCandidatesQuery 读取尚未加入团队的企业成员。
type ListMemberCandidatesQuery struct{ db *bun.DB }

// NewListMemberCandidatesQuery 创建团队成员候选查询。
func NewListMemberCandidatesQuery(db *bun.DB) *ListMemberCandidatesQuery {
	return &ListMemberCandidatesQuery{db: db}
}

// Execute 返回尚未加入指定团队的企业成员分页列表。
func (q *ListMemberCandidatesQuery) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, input MemberCandidateInput) (MemberCandidateOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	var valid bool
	input.Page, input.PageSize, valid = normalizePage(input.Page, input.PageSize)
	if !valid {
		return MemberCandidateOutput{}, &common.FieldError{Fields: map[string]common.FieldCode{"query": ValidationQueryInvalid}}
	}
	if err := validateIdentity(ctx, q.db, identity); err != nil {
		return MemberCandidateOutput{}, err
	}
	if _, err := loadTeam(ctx, q.db, identity.Organization.ID, teamID); err != nil {
		return MemberCandidateOutput{}, err
	}

	applyFilters := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.
			Where("u.organization_id = ?", identity.Organization.ID).
			Where("u.status = 'active'").
			Where("NOT EXISTS (SELECT 1 FROM team_members AS tm WHERE tm.organization_id = u.organization_id AND tm.team_id = ? AND tm.identity_type = ? AND tm.identity_id = u.id)", teamID, domain.MemberIdentityTypeUser)
		if input.Query != "" {
			pattern := "%" + input.Query + "%"
			query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				return group.Where("u.display_name ILIKE ?", pattern).WhereOr("u.email ILIKE ?", pattern)
			})
		}
		return query
	}

	total, err := applyFilters(q.db.NewSelect().Model((*servermodels.User)(nil))).Count(ctx)
	if err != nil {
		return MemberCandidateOutput{}, fmt.Errorf("count team member candidates: %w", err)
	}
	members := make([]MemberCandidate, 0)
	if err := applyFilters(q.db.NewSelect().TableExpr("users AS u")).
		ColumnExpr("? AS identity_type", domain.MemberIdentityTypeUser).
		ColumnExpr("u.id::text AS identity_id, u.display_name, u.avatar_file_id::text").
		ColumnExpr("r.id::text AS role_id, r.kind AS role_kind, r.name AS role_name").
		Join("JOIN roles AS r ON r.id = u.role_id AND r.organization_id = u.organization_id").
		OrderExpr("lower(u.display_name) ASC, u.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &members); err != nil {
		return MemberCandidateOutput{}, fmt.Errorf("list team member candidates: %w", err)
	}
	return MemberCandidateOutput{Members: members, Page: PageInfo{Number: input.Page, Size: input.PageSize, Total: total}}, nil
}
