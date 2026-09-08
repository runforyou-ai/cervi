//go:build server

package mcpserver

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// UpdateMCPServerAction 修改 MCP 服务。
type UpdateMCPServerAction struct {
	db *bun.DB
}

// NewUpdateMCPServerAction 创建 MCP 服务修改操作。
func NewUpdateMCPServerAction(db *bun.DB) *UpdateMCPServerAction {
	return &UpdateMCPServerAction{db: db}
}

// Execute 修改当前企业中的 MCP 服务。
func (a *UpdateMCPServerAction) Execute(ctx context.Context, identity *servermodels.Identity, mcpServerID string, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var mcpServer *servermodels.MCPServer
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		current, err := loadMCPServer(ctx, tx, identity.Organization.ID, mcpServerID, true)
		if err != nil {
			return err
		}
		current.Name = input.Name
		current.URL = input.URL
		current.ServerType = input.ServerType
		current.AuthorizationToken = input.AuthorizationToken
		if _, err := tx.NewUpdate().
			Model(current).
			Column("name", "url", "server_type", "authorization_token").
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			WherePK().
			Returning("*").
			Exec(ctx); err != nil {
			return err
		}
		mcpServer = current
		return nil
	})
	// 企业内名称不区分大小写且保持唯一。
	if pgerr.UniqueViolationOn(err, "mcp_servers_organization_name_unique") {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("update MCP server: %w", err)
	}
	output := recordFromModel(*mcpServer)
	return &output, nil
}
