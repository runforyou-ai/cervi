//go:build server

package integrationconnection

import (
	"context"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetIntegrationConnectionQuery 查询当前企业中的连接器。
type GetIntegrationConnectionQuery struct {
	db *bun.DB
}

// NewGetIntegrationConnectionQuery 创建连接器详情查询。
func NewGetIntegrationConnectionQuery(db *bun.DB) *GetIntegrationConnectionQuery {
	return &GetIntegrationConnectionQuery{db: db}
}

// Execute 返回连接器及其认证配置。
func (q *GetIntegrationConnectionQuery) Execute(ctx context.Context, identity *servermodels.Identity, connectionID string) (*Record, error) {
	connection, err := loadConnection(ctx, q.db, identity.Organization.ID, connectionID, false)
	if err != nil {
		return nil, fmt.Errorf("get integration connection: %w", err)
	}
	output := recordFromModel(*connection)
	return &output, nil
}
