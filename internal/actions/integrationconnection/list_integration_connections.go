//go:build server

package integrationconnection

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListIntegrationConnectionsQuery 查询当前企业的连接器。
type ListIntegrationConnectionsQuery struct {
	db *bun.DB
}

// NewListIntegrationConnectionsQuery 创建连接器列表查询。
func NewListIntegrationConnectionsQuery(db *bun.DB) *ListIntegrationConnectionsQuery {
	return &ListIntegrationConnectionsQuery{db: db}
}

// Execute 返回当前企业的全部连接器。
func (q *ListIntegrationConnectionsQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]Summary, error) {
	connections := make([]servermodels.IntegrationConnection, 0)
	if err := q.db.NewSelect().
		Model(&connections).
		Column("id", "connector_type", "name", "description", "status", "last_tested_at").
		Where("ic.organization_id = ?", identity.Organization.ID).
		Order("ic.created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list integration connections: %w", err)
	}
	output := make([]Summary, 0, len(connections))
	for _, connection := range connections {
		output = append(output, Summary{
			ID: connection.ID, Type: domain.IntegrationConnectionType(connection.Type),
			Name: connection.Name, Description: connection.Description,
			Status: domain.IntegrationConnectionStatus(connection.Status), LastTestedAt: connection.LastTestedAt,
		})
	}
	return output, nil
}
