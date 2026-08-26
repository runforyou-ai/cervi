//go:build server

package businesssystem

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetBusinessSystemQuery 查询当前企业中的业务系统。
type GetBusinessSystemQuery struct {
	db *bun.DB
}

// NewGetBusinessSystemQuery 创建业务系统详情查询。
func NewGetBusinessSystemQuery(db *bun.DB) *GetBusinessSystemQuery {
	return &GetBusinessSystemQuery{db: db}
}

// Execute 返回当前企业中的业务系统详情。
func (q *GetBusinessSystemQuery) Execute(ctx context.Context, identity *servermodels.Identity, businessSystemID string) (*Record, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	businessSystem, err := loadBusinessSystem(ctx, q.db, identity.Organization.ID, businessSystemID, false)
	if err != nil {
		return nil, fmt.Errorf("get business system: %w", err)
	}
	output := recordFromModel(*businessSystem)
	return &output, nil
}
