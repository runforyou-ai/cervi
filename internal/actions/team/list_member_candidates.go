//go:build server

package team

import (
	"context"
	"fmt"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
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
			Where("om.organization_id = ?", identity.Organization.ID).
			Where("om.status = 'active'").
			Where("NOT EXISTS (SELECT 1 FROM team_members AS tm WHERE tm.organization_id = om.organization_id AND tm.team_id = ? AND tm.member_id = om.id)", teamID)
		if input.Query != "" {
			pattern := "%" + input.Query + "%"
			query = query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
				return group.Where("om.display_name ILIKE ?", pattern).WhereOr("u.email ILIKE ?", pattern)
			})
		}
		return query
	}

	total, err := applyFilters(q.db.NewSelect().TableExpr("organization_members AS om").Join("LEFT JOIN users AS u ON u.id = om.id AND u.organization_id = om.organization_id")).Count(ctx)
	if err != nil {
		return MemberCandidateOutput{}, fmt.Errorf("count team member candidates: %w", err)
	}
	members := make([]MemberCandidate, 0)
	if err := applyFilters(q.db.NewSelect().TableExpr("organization_members AS om")).
		ColumnExpr("om.type AS identity_type").
		ColumnExpr("om.id::text AS identity_id, om.display_name, om.avatar_file_id::text").
		Join("LEFT JOIN users AS u ON u.id = om.id AND u.organization_id = om.organization_id").
		OrderExpr("lower(om.display_name) ASC, om.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &members); err != nil {
		return MemberCandidateOutput{}, fmt.Errorf("list team member candidates: %w", err)
	}
	return MemberCandidateOutput{Members: members, Page: PageInfo{Number: input.Page, Size: input.PageSize, Total: total}}, nil
}
