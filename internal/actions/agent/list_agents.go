//go:build server

package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListInput 定义 AI 员工目录查询条件。
type ListInput struct {
	Query    string
	Status   domain.UserStatus
	Page     int
	PageSize int
}

// Agent 定义 AI 员工信息。
type Agent struct {
	ID          string            `bun:"id"`
	IdentityID  string            `bun:"identity_id"`
	DisplayName string            `bun:"display_name"`
	Status      domain.UserStatus `bun:"status"`
	WorkStatus  domain.WorkStatus `bun:"work_status"`
	Teams       []TeamSummary
	Capability  Capability
	CreatedAt   time.Time `bun:"created_at"`
}

// ListItem 定义 AI 员工目录项。
type ListItem struct {
	ID          string            `bun:"id"`
	IdentityID  string            `bun:"identity_id"`
	DisplayName string            `bun:"display_name"`
	Status      domain.UserStatus `bun:"status"`
	WorkStatus  domain.WorkStatus `bun:"work_status"`
	Teams       []TeamSummary
	Capability  CapabilitySummary
	CreatedAt   time.Time `bun:"created_at"`
}

// ListOutput 定义 AI 员工分页结果。
type ListOutput struct {
	Agents []ListItem
	Page   int
	Size   int
	Total  int
}

// ListAgentsQuery 读取企业 AI 员工目录。
type ListAgentsQuery struct{ db *bun.DB }

// NewListAgentsQuery 创建 AI 员工目录查询。
func NewListAgentsQuery(db *bun.DB) *ListAgentsQuery {
	return &ListAgentsQuery{db: db}
}

// Execute 返回满足条件的 AI 员工分页列表。
func (q *ListAgentsQuery) Execute(ctx context.Context, identity *servermodels.Identity, input ListInput) (ListOutput, error) {
	input.Query = strings.TrimSpace(input.Query)
	input.Status = domain.UserStatus(strings.TrimSpace(string(input.Status)))
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 50
	}
	if input.PageSize > 100 || (input.Status != "" && input.Status != domain.UserStatusActive && input.Status != domain.UserStatusInactive) {
		return ListOutput{}, ErrQueryInvalid
	}
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return ListOutput{}, err
	}
	applyFilters := func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.
			Where("a.organization_id = ?", identity.Organization.ID).
			Where("oi.type = ?", domain.OrganizationIdentityTypeAgent)
		if input.Status != "" {
			query = query.Where("a.status = ?", input.Status)
		}
		if input.Query != "" {
			query = query.Where("oi.display_name ILIKE ?", "%"+input.Query+"%")
		}
		return query
	}
	base := func() *bun.SelectQuery {
		return q.db.NewSelect().TableExpr("agents AS a").
			Join("JOIN organization_identities AS oi ON oi.id = a.identity_id AND oi.organization_id = a.organization_id")
	}
	total, err := applyFilters(base()).Count(ctx)
	if err != nil {
		return ListOutput{}, fmt.Errorf("count agents: %w", err)
	}
	agents := make([]ListItem, 0)
	if err := applyFilters(base()).
		ColumnExpr("a.id::text AS id, a.identity_id::text AS identity_id, oi.display_name, a.status, oi.work_status, oi.created_at").
		OrderExpr("lower(oi.display_name) ASC, a.id ASC").
		Limit(input.PageSize).
		Offset((input.Page-1)*input.PageSize).
		Scan(ctx, &agents); err != nil {
		return ListOutput{}, fmt.Errorf("list agents: %w", err)
	}
	agentIDs := make([]string, 0, len(agents))
	for _, agent := range agents {
		agentIDs = append(agentIDs, agent.ID)
	}
	capabilities, err := loadAgentCapabilitySummaries(ctx, q.db, identity.Organization.ID, agentIDs)
	if err != nil {
		return ListOutput{}, fmt.Errorf("load agent capability summaries: %w", err)
	}
	for index := range agents {
		teams, err := loadAgentTeams(ctx, q.db, identity.Organization.ID, agents[index].IdentityID)
		if err != nil {
			return ListOutput{}, fmt.Errorf("load agent teams: %w", err)
		}
		agents[index].Teams = teams
		agents[index].Capability = capabilities[agents[index].ID]
	}
	return ListOutput{Agents: agents, Page: input.Page, Size: input.PageSize, Total: total}, nil
}

// loadAgentTeams 读取 AI 员工所属团队。
func loadAgentTeams(ctx context.Context, db bun.IDB, organizationID, organizationIdentityID string) ([]TeamSummary, error) {
	teams := make([]TeamSummary, 0)
	err := db.NewSelect().TableExpr("team_members AS tm").
		ColumnExpr("t.id::text AS id, t.name").
		Join("JOIN teams AS t ON t.id = tm.team_id AND t.organization_id = tm.organization_id").
		Where("tm.organization_id = ?", organizationID).
		Where("tm.identity_id = ?", organizationIdentityID).
		OrderExpr("lower(t.name) ASC, t.id ASC").
		Scan(ctx, &teams)
	return teams, err
}
