//go:build server

package setting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetS3SettingQuery 读取当前企业的 S3 配置。
type GetS3SettingQuery struct {
	db *bun.DB
}

// NewGetS3SettingQuery 创建 S3 配置查询。
func NewGetS3SettingQuery(db *bun.DB) *GetS3SettingQuery {
	return &GetS3SettingQuery{db: db}
}

// Execute 返回当前企业的 S3 配置，尚未配置时返回默认配置。
func (q *GetS3SettingQuery) Execute(ctx context.Context, identity *servermodels.Identity) (S3Setting, error) {
	record := &servermodels.Setting{}
	err := q.db.NewSelect().
		Model(record).
		Column("value").
		Where("organization_id = ?", identity.Organization.ID).
		Where("key = ?", s3SettingKey).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultS3Setting(), nil
	}
	if err != nil {
		return S3Setting{}, fmt.Errorf("get S3 setting: %w", err)
	}

	var setting S3Setting
	if err := json.Unmarshal(record.Value, &setting); err != nil {
		return S3Setting{}, fmt.Errorf("decode S3 setting: %w", err)
	}
	return setting, nil
}
