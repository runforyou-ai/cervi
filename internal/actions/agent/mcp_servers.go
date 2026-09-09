//go:build server

package agent

import (
	"context"
	"slices"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// MCPServerOption 定义 AI 员工配置使用的 MCP 服务选项。
type MCPServerOption struct {
	ID         string
	Name       string
	ServerType domain.MCPServerType
	ToolCount  int
}

// ListMCPServerOptionsQuery 读取当前企业的 MCP 服务目录摘要。
type ListMCPServerOptionsQuery struct{ db *bun.DB }

// NewListMCPServerOptionsQuery 创建 MCP 服务选项查询。
func NewListMCPServerOptionsQuery(db *bun.DB) *ListMCPServerOptionsQuery {
	return &ListMCPServerOptionsQuery{db: db}
}

// Execute 返回全部已配置服务，不探测远端连接或工具可用性。
func (q *ListMCPServerOptionsQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]MCPServerOption, error) {
	options := make([]MCPServerOption, 0)
	err := q.db.NewSelect().TableExpr("mcp_servers AS ms").
		ColumnExpr("ms.id, ms.name, ms.server_type, jsonb_array_length(ms.tools) AS tool_count").
		Where("ms.organization_id = ?", identity.Organization.ID).
		OrderExpr("lower(ms.name) ASC, ms.id ASC").Scan(ctx, &options)
	return options, err
}

// replaceMCPServers 锁定企业服务并按差异更新绑定，避免删除与保存并发产生悬空关系。
func replaceMCPServers(ctx context.Context, tx bun.Tx, organizationID, agentID string, values []string) error {
	ids, valid := common.NormalizeUUIDs(values)
	if !valid {
		return &common.FieldError{Fields: map[string]common.FieldCode{"mcpServerIds": ValidationMCPServerInvalid}}
	}
	slices.Sort(ids)
	if len(ids) > 0 {
		var found []string
		if err := tx.NewSelect().Model((*servermodels.MCPServer)(nil)).Column("id").
			Where("ms.organization_id = ?", organizationID).Where("ms.id IN (?)", bun.In(ids)).
			OrderExpr("ms.id ASC").For("KEY SHARE").Scan(ctx, &found); err != nil {
			return err
		}
		if len(found) != len(ids) {
			return &common.FieldError{Fields: map[string]common.FieldCode{"mcpServerIds": ValidationMCPServerInvalid}}
		}
	}
	remove := tx.NewDelete().Model((*servermodels.AgentMCPServer)(nil)).
		Where("organization_id = ?", organizationID).Where("agent_id = ?", agentID)
	if len(ids) > 0 {
		remove = remove.Where("mcp_server_id NOT IN (?)", bun.In(ids))
	}
	if _, err := remove.Exec(ctx); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	relations := make([]servermodels.AgentMCPServer, 0, len(ids))
	for _, id := range ids {
		relations = append(relations, servermodels.AgentMCPServer{OrganizationID: organizationID, AgentID: agentID, MCPServerID: id})
	}
	_, err := tx.NewInsert().Model(&relations).
		Column("organization_id", "agent_id", "mcp_server_id").
		On("CONFLICT (organization_id, agent_id, mcp_server_id) DO NOTHING").Exec(ctx)
	return err
}
