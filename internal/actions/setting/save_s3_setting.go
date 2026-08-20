//go:build server

package setting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// SaveS3SettingAction 保存当前企业的 S3 配置。
type SaveS3SettingAction struct {
	db *bun.DB
}

// NewSaveS3SettingAction 创建 S3 配置保存操作。
func NewSaveS3SettingAction(db *bun.DB) *SaveS3SettingAction {
	return &SaveS3SettingAction{db: db}
}

// Execute 校验并保存当前企业的 S3 配置。
func (a *SaveS3SettingAction) Execute(ctx context.Context, identity *servermodels.Identity, input S3Setting) (S3Setting, error) {
	input, fields := normalizeS3Setting(input)
	if len(fields) > 0 {
		return S3Setting{}, &ValidationError{Fields: fields}
	}
	if identity == nil ||
		!common.ValidUUID(identity.Organization.ID) ||
		!common.ValidUUID(identity.User.ID) ||
		identity.User.OrganizationID != identity.Organization.ID {
		return S3Setting{}, common.ErrIdentityInvalid
	}

	value, err := json.Marshal(input)
	if err != nil {
		return S3Setting{}, fmt.Errorf("encode S3 setting: %w", err)
	}
	record := &servermodels.Setting{
		OrganizationID: identity.Organization.ID,
		Key:            s3SettingKey,
		Value:          value,
	}
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		organization := &servermodels.Organization{}
		if err := tx.NewSelect().
			Model(organization).
			Column("id").
			Where("o.id = ?", identity.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
			return err
		}

		user := &servermodels.User{}
		if err := tx.NewSelect().
			Model(user).
			Column("id").
			Where("u.id = ?", identity.User.ID).
			Where("u.organization_id = ?", identity.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return common.ErrIdentityInvalid
			}
			return err
		}

		_, err := tx.NewInsert().
			Model(record).
			Column("organization_id", "key", "value").
			On("CONFLICT (organization_id, key) DO UPDATE").
			Set("value = EXCLUDED.value").
			Set("updated_at = now()").
			Exec(ctx)
		return err
	})
	if errors.Is(err, common.ErrIdentityInvalid) {
		return S3Setting{}, common.ErrIdentityInvalid
	}
	if err != nil {
		return S3Setting{}, fmt.Errorf("save S3 setting: %w", err)
	}
	return input, nil
}
