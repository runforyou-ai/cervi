//go:build server

package agent

import (
	"context"
	"slices"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// MCPServerOption 定义 AI 员工配置使用的 MCP 服务选项。
type MCPServerOption struct {
	ID        string
	Name      string
	ToolCount int
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
		ColumnExpr("ms.id, ms.name, jsonb_array_length(ms.tools) AS tool_count").
		Where("ms.organization_id = ?", identity.Organization.ID).
		OrderExpr("lower(ms.name) ASC, ms.id ASC").Scan(ctx, &options)
	return options, err
}

// validateAndLockMCPServers 规范化并锁定企业服务，保证新版本不会引用已删除的服务。
func validateAndLockMCPServers(ctx context.Context, tx bun.Tx, organizationID string, values []string) ([]string, error) {
	ids, valid := common.NormalizeUUIDs(values)
	if !valid {
		return nil, &common.FieldError{Fields: map[string]common.FieldCode{"mcpServerIds": ValidationMCPServerInvalid}}
	}
	slices.Sort(ids)
	if len(ids) > 0 {
		var found []string
		if err := tx.NewSelect().Model((*servermodels.MCPServer)(nil)).Column("id").
			Where("ms.organization_id = ?", organizationID).Where("ms.id IN (?)", bun.In(ids)).
			OrderExpr("ms.id ASC").For("KEY SHARE").Scan(ctx, &found); err != nil {
			return nil, err
		}
		if len(found) != len(ids) {
			return nil, &common.FieldError{Fields: map[string]common.FieldCode{"mcpServerIds": ValidationMCPServerInvalid}}
		}
	}
	return ids, nil
}
