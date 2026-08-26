//go:build server

package businesssystem

import (
	"context"
	"fmt"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ListBusinessSystemsQuery 查询当前企业的业务系统。
type ListBusinessSystemsQuery struct {
	db *bun.DB
}

// NewListBusinessSystemsQuery 创建业务系统列表查询。
func NewListBusinessSystemsQuery(db *bun.DB) *ListBusinessSystemsQuery {
	return &ListBusinessSystemsQuery{db: db}
}

// Execute 返回当前企业的业务系统列表。
func (q *ListBusinessSystemsQuery) Execute(ctx context.Context, identity *servermodels.Identity) ([]Record, error) {
	if err := identityaction.Validate(ctx, q.db, identity); err != nil {
		return nil, err
	}
	records := make([]servermodels.BusinessSystem, 0)
	if err := q.db.NewSelect().
		Model(&records).
		Where("bs.organization_id = ?", identity.Organization.ID).
		Order("bs.created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list business systems: %w", err)
	}
	output := make([]Record, 0, len(records))
	for _, record := range records {
		output = append(output, recordFromModel(record))
	}
	return output, nil
}
