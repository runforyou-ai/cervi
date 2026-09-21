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

// MemberListInput 定义团队成员列表查询条件。
type MemberListInput struct {
	Query      string
	WorkStatus domain.WorkStatus
	Page       int
	PageSize   int
}

// Member 定义团队成员信息。UserID 与 AgentID 按身份类型二选一，另一个为空。
type Member struct {
	IdentityID   string                          `bun:"identity_id"`
	IdentityType domain.OrganizationIdentityType `bun:"identity_type"`
	UserID       *string                         `bun:"user_id"`
	AgentID      *string                         `bun:"agent_id"`
	DisplayName  string                          `bun:"display_name"`
	AvatarFileID *string                         `bun:"avatar_file_id"`
	WorkStatus   domain.WorkStatus               `bun:"work_status"`
	JoinedAt     time.Time                       `bun:"joined_at"`
}

// MemberListOutput 定义团队成员分页结果。
type MemberListOutput struct {
	Members []Member
	Page    PageInfo
}

// ListMembersQuery 读取团队成员关系。
type ListMembersQuery struct{ db *bun.DB }

// NewListMembersQuery 创建团队成员列表查询。
func NewListMembersQuery(db *bun.DB) *ListMembersQuery {
	return &ListMembersQuery{db: db}
}

// Execute 返回团队成员分页列表。
func (q *ListMembersQuery) Execute(ctx context.Context, identity *servermodels.Identity, teamID string, input MemberListInput) (MemberListOutput, error) {
	input, err := normalizeMemberListInput(input)
	if err != nil {
		return MemberListOutput{}, err
	}
	if _, err := loadTeam(ctx, q.db, identity.Organization.ID, teamID); err != nil {
		return MemberListOutput{}, err
	}
	return q.list(ctx, identity, teamID, input)
}

// ExecuteAll 返回企业所有团队的成员分页列表，同一身份只列一次，加入时间取最早加入团队的时间。
func (q *ListMembersQuery) ExecuteAll(ctx context.Context, identity *servermodels.Identity, input MemberListInput) (MemberListOutput, error) {
	input, err := normalizeMemberListInput(input)
	if err != nil {
		return MemberListOutput{}, err
	}
	return q.list(ctx, identity, "", input)
}

// normalizeMemberListInput 规范化并校验团队成员列表查询条件。
func normalizeMemberListInput(input MemberListInput) (MemberListInput, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.WorkStatus = domain.WorkStatus(strings.TrimSpace(string(input.WorkStatus)))
	var pageValid bool
	input.Page, input.PageSize, pageValid = common.NormalizePagination(input.Page, input.PageSize)
	if !pageValid {
		return input, &common.FieldError{Fields: map[string]common.FieldCode{"query": ValidationQueryInvalid}}
	}
	if input.WorkStatus != "" && input.WorkStatus != domain.WorkStatusWorking && input.WorkStatus != domain.WorkStatusAway && input.WorkStatus != domain.WorkStatusOffDuty {
		return input, &common.FieldError{Fields: map[string]common.FieldCode{"workStatus": ValidationWorkStatusInvalid}}
	}
	return input, nil
}

// list 按团队读取成员分页列表；teamID 为空时读取所有团队并按身份去重。
func (q *ListMembersQuery) list(ctx context.Context, identity *servermodels.Identity, teamID string, input MemberListInput) (MemberListOutput, error) {
	applyFilters := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.
			Where("tm.organization_id = ?", identity.Organization.ID).
			Where("oi.type IN (?, ?)", domain.OrganizationIdentityTypeUser, domain.OrganizationIdentityTypeAgent).
			Where("((oi.type = ? AND u.status = ?) OR (oi.type = ? AND a.status = ?))", domain.OrganizationIdentityTypeUser, domain.UserStatusActive, domain.OrganizationIdentityTypeAgent, domain.UserStatusActive)
		if teamID != "" {
			query = query.Where("tm.team_id = ?", teamID)
		}
		if input.WorkStatus != "" {
			query = query.Where("oi.work_status = ?", input.WorkStatus)
		}
		if input.Query != "" {
			query = query.Where("oi.display_name ILIKE ?", "%"+input.Query+"%")
		}
		return query
	}
	base := func() *bun.SelectQuery {
		return q.db.NewSelect().TableExpr("team_members AS tm").
			Join("JOIN organization_identities AS oi ON oi.id = tm.identity_id AND oi.organization_id = tm.organization_id").
			Join("LEFT JOIN users AS u ON u.identity_id = oi.id AND u.organization_id = oi.organization_id").
			Join("LEFT JOIN agents AS a ON a.identity_id = oi.id AND a.organization_id = oi.organization_id")
	}
	// 按身份去重计数，单个团队内每个身份只出现一次。
	var total int
	if err := applyFilters(base()).ColumnExpr("count(DISTINCT oi.id)").Scan(ctx, &total); err != nil {
		return MemberListOutput{}, fmt.Errorf("count team members: %w", err)
	}
	members := make([]Member, 0)
	if err := applyFilters(base()).
		ColumnExpr("oi.id::text AS identity_id, oi.type AS identity_type, u.id::text AS user_id, a.id::text AS agent_id, oi.display_name, oi.avatar_file_id::text AS avatar_file_id, oi.work_status, min(tm.created_at) AS joined_at").
		GroupExpr("oi.id, oi.type, u.id, a.id, oi.display_name, oi.avatar_file_id, oi.work_status").
		OrderExpr("lower(oi.display_name) ASC, oi.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &members); err != nil {
		return MemberListOutput{}, fmt.Errorf("list team members: %w", err)
	}
	return MemberListOutput{Members: members, Page: PageInfo{Number: input.Page, Size: input.PageSize, Total: total}}, nil
}
