//go:build server

package installation

import (
	"context"
	"fmt"

	deploymentaction "github.com/runforyou-ai/cervi/internal/actions/deployment"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Status 表示部署是否已完成首次安装以及是否开放账号注册。
type Status struct {
	Installed        bool
	RegistrationOpen bool
}

// StatusQuery 查询部署的安装状态。
type StatusQuery struct {
	db *bun.DB
}

// NewStatusQuery 创建安装状态查询。
func NewStatusQuery(db *bun.DB) *StatusQuery {
	return &StatusQuery{db: db}
}

// Execute 返回部署是否已有账号及注册开关。
func (q *StatusQuery) Execute(ctx context.Context) (Status, error) {
	installed, err := q.db.NewSelect().Model((*servermodels.Account)(nil)).Exists(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("check installation: %w", err)
	}
	settings, err := deploymentaction.LoadSettings(ctx, q.db)
	if err != nil {
		return Status{}, err
	}
	return Status{Installed: installed, RegistrationOpen: settings.RegistrationOpen}, nil
}
