//go:build server

package agentrun

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// loadRunMCPServers 读取本次运行的配置版本绑定且仍存在的同企业 MCP 服务。
func loadRunMCPServers(ctx context.Context, db bun.IDB, run *servermodels.AgentRun) ([]agentruntime.MCPServer, error) {
	services := make([]servermodels.MCPServer, 0)
	err := db.NewSelect().Model(&services).
		Join("JOIN agent_revisions AS ar ON ar.id = ? AND ar.organization_id = ms.organization_id", run.AgentRevisionID).
		Where("ms.organization_id = ?", run.OrganizationID).
		Where("ar.configuration->'mcpServerIds' @> jsonb_build_array(ms.id::text)").
		Order("ms.name").Scan(ctx)
	if err != nil {
		return nil, err
	}
	servers := make([]agentruntime.MCPServer, 0, len(services))
	for _, service := range services {
		servers = append(servers, agentruntime.MCPServer{Name: service.Name, Config: mcpintegration.Config{
			URL: service.URL, ServerType: service.ServerType, AuthorizationToken: service.AuthorizationToken,
		}})
	}
	return servers, nil
}
