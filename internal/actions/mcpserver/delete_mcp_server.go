//go:build server

package mcpserver

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// DeleteMCPServerAction 删除 MCP 服务。
type DeleteMCPServerAction struct {
	db *bun.DB
}

// NewDeleteMCPServerAction 创建 MCP 服务删除操作。
func NewDeleteMCPServerAction(db *bun.DB) *DeleteMCPServerAction {
	return &DeleteMCPServerAction{db: db}
}

// Execute 删除当前企业中的 MCP 服务。
func (a *DeleteMCPServerAction) Execute(ctx context.Context, identity *servermodels.Identity, mcpServerID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		mcpServer, err := loadMCPServer(ctx, tx, identity.Organization.ID, mcpServerID, true)
		if err != nil {
			return err
		}
		// 与服务删除一起清除全部员工绑定，不改写历史执行配置。
		if _, err := tx.NewDelete().Model((*servermodels.AgentMCPServer)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("mcp_server_id = ?", mcpServerID).Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewDelete().
			Model(mcpServer).
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete MCP server: %w", err)
	}
	return nil
}
