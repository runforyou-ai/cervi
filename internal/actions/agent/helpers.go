//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
)

// loadDirectoryAgent 读取当前企业中的 AI 员工及所属团队。
func loadDirectoryAgent(ctx context.Context, db bun.IDB, organizationID, agentID string) (*DirectoryAgent, error) {
	if !common.ValidUUID(agentID) {
		return nil, ErrNotFound
	}
	agent := &DirectoryAgent{}
	err := db.NewSelect().TableExpr("agents AS a").
		ColumnExpr("a.id::text AS id, a.identity_id::text AS identity_id, oi.display_name, a.status, oi.created_at").
		Join("JOIN organization_identities AS oi ON oi.id = a.identity_id AND oi.organization_id = a.organization_id AND oi.type = ?", domain.OrganizationIdentityTypeAgent).
		Where("a.id = ?", agentID).
		Where("a.organization_id = ?", organizationID).
		Scan(ctx, agent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	agent.Teams, err = loadAgentTeams(ctx, db, organizationID, agent.IdentityID)
	return agent, err
}
