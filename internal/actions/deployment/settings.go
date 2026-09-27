//go:build server

// Package deployment 实现部署级设置的读取与维护。
package deployment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ErrAdminRequired 表示操作需要部署管理员账号。
var ErrAdminRequired = errors.New("deployment administrator required")

// Settings 表示部署级设置。
type Settings struct {
	RegistrationOpen bool
}

// LoadSettings 读取部署级设置，没有记录时返回默认值。
func LoadSettings(ctx context.Context, db bun.IDB) (Settings, error) {
	record := &servermodels.DeploymentSetting{}
	err := db.NewSelect().Model(record).Where("ds.id = 1").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("load deployment settings: %w", err)
	}
	return Settings{RegistrationOpen: record.RegistrationOpen}, nil
}

// GetSettingsQuery 读取部署级设置。
type GetSettingsQuery struct {
	db *bun.DB
}

// NewGetSettingsQuery 创建部署设置查询。
func NewGetSettingsQuery(db *bun.DB) *GetSettingsQuery {
	return &GetSettingsQuery{db: db}
}

// Execute 返回部署级设置。
func (q *GetSettingsQuery) Execute(ctx context.Context) (Settings, error) {
	return LoadSettings(ctx, q.db)
}

// UpdateSettingsAction 由部署管理员修改部署级设置。
type UpdateSettingsAction struct {
	db *bun.DB
}

// NewUpdateSettingsAction 创建部署设置修改操作。
func NewUpdateSettingsAction(db *bun.DB) *UpdateSettingsAction {
	return &UpdateSettingsAction{db: db}
}

// Execute 校验部署管理员身份并保存部署级设置。
func (a *UpdateSettingsAction) Execute(ctx context.Context, identity *servermodels.AccountIdentity, input Settings) (Settings, error) {
	if !identity.Account.IsDeploymentAdmin {
		return Settings{}, ErrAdminRequired
	}
	record := &servermodels.DeploymentSetting{ID: 1, RegistrationOpen: input.RegistrationOpen}
	if _, err := a.db.NewInsert().Model(record).
		Column("id", "registration_open").
		On("CONFLICT (id) DO UPDATE").
		Set("registration_open = EXCLUDED.registration_open").
		Set("updated_at = now()").
		Exec(ctx); err != nil {
		return Settings{}, fmt.Errorf("save deployment settings: %w", err)
	}
	return input, nil
}
