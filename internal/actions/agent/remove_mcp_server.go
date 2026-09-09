//go:build server

package agent

import (
	"context"
	"database/sql"
	"errors"
	"uuid"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RemoveMCPServerFromRevisions 在已锁定服务的事务内创建移除该服务的新版本，保留历史配置。
func RemoveMCPServerFromRevisions(ctx context.Context, tx bun.Tx, identity *servermodels.Identity, mcpServerID string) (int, error) {
	// 服务锁阻止新增引用，先确定候选员工，再按固定顺序锁定。
	revisions := tx.NewSelect().Model((*servermodels.AgentRevision)(nil)).Column("id").
		Where("organization_id = ?", identity.Organization.ID).
		Where("configuration->'mcpServerIds' @> jsonb_build_array(?::text)", mcpServerID)
	var agentIDs []string
	if err := tx.NewSelect().Model((*servermodels.Agent)(nil)).Column("id").
		Where("a.organization_id = ?", identity.Organization.ID).
		Where("a.active_revision_id IN (?)", revisions).
		Scan(ctx, &agentIDs); err != nil {
		return 0, err
	}
	if len(agentIDs) == 0 {
		return 0, nil
	}
	// 按员工编号锁定候选员工。
	if err := tx.NewSelect().Model((*servermodels.Agent)(nil)).Column("id").
		Where("a.organization_id = ?", identity.Organization.ID).Where("a.id IN (?)", bun.In(agentIDs)).
		OrderExpr("a.id ASC").For("UPDATE").Scan(ctx, &agentIDs); err != nil {
		return 0, err
	}
	count := 0
	for _, agentID := range agentIDs {
		// 等待员工锁期间可能切换了版本，取锁后重新读取并只替换 MCP 选择。
		revision := &servermodels.AgentRevision{}
		err := tx.NewSelect().Model(revision).
			Column("organization_id", "agent_id", "execution_mode", "schema_version").
			ColumnExpr("jsonb_set(ar.configuration, '{mcpServerIds}', (ar.configuration->'mcpServerIds') - ?::text) AS configuration", mcpServerID).
			Join("JOIN agents AS a ON a.active_revision_id = ar.id AND a.organization_id = ar.organization_id AND a.id = ar.agent_id").
			Where("a.organization_id = ?", identity.Organization.ID).Where("a.id = ?", agentID).
			Where("ar.configuration->'mcpServerIds' @> jsonb_build_array(?::text)", mcpServerID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		revision.ID = uuid.NewV7().String()
		revision.CreatedByUserID = identity.User.ID
		if _, err := tx.NewInsert().Model(revision).
			Column("id", "organization_id", "agent_id", "execution_mode", "schema_version", "configuration", "created_by_user_id").Exec(ctx); err != nil {
			return 0, err
		}
		if _, err := tx.NewUpdate().Model((*servermodels.Agent)(nil)).
			Set("active_revision_id = ?", revision.ID).Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).Where("id = ?", agentID).Exec(ctx); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}
