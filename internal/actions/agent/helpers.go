//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"

	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// loadAgent 读取当前企业中的 AI 员工详情。
func loadAgent(ctx context.Context, db bun.IDB, organizationID, agentID string) (*Agent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	agent := &Agent{}
	err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.id::text AS id, a.identity_id::text AS identity_id, oi.display_name, a.status, oi.work_status, oi.created_at").
		ColumnExpr("r.id::text AS role_id, r.kind AS role_kind, r.name AS role_name").
		Join("JOIN organization_identities AS oi ON oi.id = a.identity_id AND oi.organization_id = a.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeAgent).
		Join("JOIN roles AS r ON r.id = oi.role_id AND r.organization_id = oi.organization_id").
		Where("a.id = ?", agentID).
		Where("a.organization_id = ?", organizationID).
		Scan(ctx, agent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	agent.Teams, err = teamaction.LoadIdentityTeams(ctx, db, organizationID, agent.IdentityID)
	if err != nil {
		return nil, err
	}
	agent.Execution, err = loadAgentExecution(ctx, db, organizationID, agentID)
	return agent, err
}
