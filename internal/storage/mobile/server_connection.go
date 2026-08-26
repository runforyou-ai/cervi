//go:build !server && (ios || android)

package mobile

import (
	"context"
	"database/sql"
	"errors"
	"time"

	mobilemodels "github.com/runforyou-ai/cervi/internal/storage/mobile/models"
)

const serverURLSettingKey = "server_url"

// GetServerURL 读取移动端保存的企业服务器地址。
func (s *Store) GetServerURL(ctx context.Context) (string, error) {
	setting := &mobilemodels.AppSetting{}
	err := s.db.NewSelect().
		Model(setting).
		Where("key = ?", serverURLSettingKey).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetServerURL 保存或更新移动端企业服务器地址。
func (s *Store) SetServerURL(ctx context.Context, serverURL string) error {
	setting := &mobilemodels.AppSetting{
		Key:       serverURLSettingKey,
		Value:     serverURL,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.NewInsert().
		Model(setting).
		Column("key", "value", "updated_at").
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}
