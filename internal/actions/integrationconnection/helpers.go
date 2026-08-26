//go:build server

package integrationconnection

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
)

// loadConnection 读取当前企业中的连接器。
func loadConnection(ctx context.Context, db bun.IDB, organizationID, connectionID string, lock bool) (*servermodels.IntegrationConnection, error) {
	if !common.ValidUUID(connectionID) {
		return nil, ErrNotFound
	}
	connection := &servermodels.IntegrationConnection{}
	query := db.NewSelect().
		Model(connection).
		Where("ic.id = ?", connectionID).
		Where("ic.organization_id = ?", organizationID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return connection, nil
}

// recordFromModel 转换连接器存储模型。
func recordFromModel(connection servermodels.IntegrationConnection) Record {
	return Record{
		ID: connection.ID, Type: domain.IntegrationConnectionType(connection.Type),
		Name: connection.Name, Description: connection.Description,
		Configuration: Configuration{
			APIURL: connection.Configuration.APIURL,
			APIKey: connection.Configuration.APIKey,
		},
		Status: domain.IntegrationConnectionStatus(connection.Status), LastTestedAt: connection.LastTestedAt,
	}
}

// connectionInUse 判断连接器是否已被知识库使用。
func connectionInUse(ctx context.Context, db bun.IDB, organizationID, connectionID string) (bool, error) {
	return db.NewSelect().TableExpr("knowledge_bases AS kb").
		Where("kb.organization_id = ?", organizationID).
		Where("kb.integration_connection_id = ?", connectionID).
		Exists(ctx)
}

// isNameConflict 判断企业内连接名称是否重复。
func isNameConflict(err error) bool {
	return pgerr.UniqueViolationOn(err, "integration_connections_organization_name_unique")
}
