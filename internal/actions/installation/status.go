//go:build server

package installation

import (
	"context"
	"database/sql"
	"errors"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Status 表示当前实例是否已初始化以及公开的企业名称。
type Status struct {
	Installed        bool
	OrganizationName string
}

// StatusQuery 查询当前实例的安装状态。
type StatusQuery struct {
	db *bun.DB
}

// NewStatusQuery 创建安装状态查询。
func NewStatusQuery(db *bun.DB) *StatusQuery {
	return &StatusQuery{db: db}
}

// Execute 返回当前实例的初始化状态和公开企业名称。
func (q *StatusQuery) Execute(ctx context.Context) (Status, error) {
	var organization servermodels.Organization
	err := q.db.NewSelect().Model(&organization).Column("name").Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Installed: true, OrganizationName: organization.Name}, nil
}
