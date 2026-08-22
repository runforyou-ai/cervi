//go:build server

package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DirectoryMemberInput 定义团队成员目录查询条件。
type DirectoryMemberInput struct {
	Query    string
	Status   domain.UserStatus
	Page     int
	PageSize int
}

// DirectoryMember 定义团队视图中的共同成员字段。
type DirectoryMember struct {
	ID          string                    `bun:"id"`
	Type        domain.MemberIdentityType `bun:"type"`
	DisplayName string                    `bun:"display_name"`
	Status      domain.UserStatus         `bun:"status"`
	JoinedAt    time.Time                 `bun:"joined_at"`
}

// DirectoryMemberOutput 定义团队成员分页结果。
type DirectoryMemberOutput struct {
	Members []DirectoryMember
	Page    PageInfo
}

// ListMembersQuery 读取团队成员关系。
type ListMembersQuery struct{ db *bun.DB }

// NewListMembersQuery 创建团队成员目录查询。
func NewListMembersQuery(db *bun.DB) *ListMembersQuery {
	return &ListMembersQuery{db: db}
}

// Execute 返回团队中的企业成员和 AI 员工公共字段。
func (q *ListMembersQuery) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, input DirectoryMemberInput) (DirectoryMemberOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = domain.UserStatus(strings.TrimSpace(string(input.Status)))
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 50
	}
	if input.PageSize > 100 || (input.Status != "" && input.Status != domain.UserStatusActive && input.Status != domain.UserStatusInactive) {
		return DirectoryMemberOutput{}, &common.FieldError{Fields: map[string]common.FieldCode{"query": ValidationQueryInvalid}}
	}
	if err := validateIdentity(ctx, q.db, identity); err != nil {
		return DirectoryMemberOutput{}, err
	}
	if _, err := loadTeam(ctx, q.db, identity.Organization.ID, teamID); err != nil {
		return DirectoryMemberOutput{}, err
	}
	applyFilters := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.
			Where("tm.organization_id = ?", identity.Organization.ID).
			Where("tm.team_id = ?", teamID).
			Where("om.type IN (?, ?)", domain.MemberIdentityTypeUser, domain.MemberIdentityTypeAgent)
		if input.Status != "" {
			query = query.Where("om.status = ?", input.Status)
		}
		if input.Query != "" {
			query = query.Where("om.display_name ILIKE ?", "%"+input.Query+"%")
		}
		return query
	}
	base := func() *bun.SelectQuery {
		return q.db.NewSelect().TableExpr("team_members AS tm").
			Join("JOIN organization_members AS om ON om.id = tm.member_id AND om.organization_id = tm.organization_id")
	}
	total, err := applyFilters(base()).Count(ctx)
	if err != nil {
		return DirectoryMemberOutput{}, fmt.Errorf("count team members: %w", err)
	}
	members := make([]DirectoryMember, 0)
	if err := applyFilters(base()).
		ColumnExpr("om.id::text AS id, om.type, om.display_name, om.status, tm.created_at AS joined_at").
		OrderExpr("lower(om.display_name) ASC, om.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &members); err != nil {
		return DirectoryMemberOutput{}, fmt.Errorf("list team members: %w", err)
	}
	return DirectoryMemberOutput{Members: members, Page: PageInfo{Number: input.Page, Size: input.PageSize, Total: total}}, nil
}
