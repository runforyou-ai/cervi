//go:build server

package businesssystem

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// loadBusinessSystem 读取当前企业中的业务系统。
func loadBusinessSystem(ctx context.Context, db bun.IDB, organizationID, businessSystemID string, lock bool) (*servermodels.BusinessSystem, error) {
	if !common.ValidUUID(businessSystemID) {
		return nil, ErrNotFound
	}
	businessSystem := &servermodels.BusinessSystem{}
	query := db.NewSelect().
		Model(businessSystem).
		Where("bs.id = ?", businessSystemID).
		Where("bs.organization_id = ?", organizationID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return businessSystem, nil
}

// recordFromModel 转换业务系统存储模型。
func recordFromModel(input servermodels.BusinessSystem) Record {
	return Record{
		ID: input.ID, Name: input.Name, Description: input.Description, URL: input.URL, Enabled: input.Enabled,
		CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}

// isNameConflict 判断企业内业务系统名称是否重复。
func isNameConflict(err error) bool {
	return pgerr.UniqueViolationOn(err, "business_systems_organization_name_unique")
}
