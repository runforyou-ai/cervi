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

// CreateMCPServerAction 创建 MCP 服务。
type CreateMCPServerAction struct {
	db *bun.DB
}

// NewCreateMCPServerAction 创建 MCP 服务操作。
func NewCreateMCPServerAction(db *bun.DB) *CreateMCPServerAction {
	return &CreateMCPServerAction{db: db}
}

// Execute 在当前企业中创建 MCP 服务。
func (a *CreateMCPServerAction) Execute(ctx context.Context, identity *servermodels.Identity, input Input) (*Record, error) {
	input, fields := normalizeInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	var mcpServer servermodels.MCPServer
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		mcpServer = servermodels.MCPServer{
			OrganizationID: identity.Organization.ID, Name: input.Name,
			URL: input.URL, ServerType: input.ServerType, AuthorizationToken: input.AuthorizationToken,
		}
		_, err := tx.NewInsert().
			Model(&mcpServer).
			Column("organization_id", "name", "url", "server_type", "authorization_token").
			Returning("*").
			Exec(ctx)
		return err
	})
	// 企业内名称不区分大小写且保持唯一。
	if pgerr.UniqueViolationOn(err, "mcp_servers_organization_name_unique") {
		return nil, &ValidationError{Fields: map[string]ValidationCode{"name": ValidationNameDuplicate}}
	}
	if err != nil {
		return nil, fmt.Errorf("create MCP server: %w", err)
	}
	output := recordFromModel(mcpServer)
	return &output, nil
}
