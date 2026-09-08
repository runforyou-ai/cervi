//go:build server

package mcpserver

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// loadMCPServer 读取当前企业中的 MCP 服务。
func loadMCPServer(ctx context.Context, db bun.IDB, organizationID, mcpServerID string, lock bool) (*servermodels.MCPServer, error) {
	if !common.ValidUUID(mcpServerID) {
		return nil, ErrNotFound
	}
	mcpServer := &servermodels.MCPServer{}
	query := db.NewSelect().
		Model(mcpServer).
		Where("ms.id = ?", mcpServerID).
		Where("ms.organization_id = ?", organizationID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return mcpServer, nil
}

// recordFromModel 转换 MCP 服务存储模型。
func recordFromModel(input servermodels.MCPServer) Record {
	return Record{
		ID: input.ID, Name: input.Name, URL: input.URL, ServerType: input.ServerType, AuthorizationToken: input.AuthorizationToken,
		CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}
