//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"errors"

	desktopmodels "github.com/runforyou-ai/cervi/internal/storage/desktop/models"
)

const serverURLSettingKey = "server_url"

// GetServerURL 读取桌面端保存的企业服务器地址。
func (s *Store) GetServerURL(ctx context.Context) (string, error) {
	setting := &desktopmodels.AppSetting{}
	err := s.db.NewSelect().
		Model(setting).
		Where("key = ?", serverURLSettingKey).
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return setting.Value, err
}

// SetServerURL 保存或更新桌面端企业服务器地址。
func (s *Store) SetServerURL(ctx context.Context, serverURL string) error {
	setting := &desktopmodels.AppSetting{Key: serverURLSettingKey, Value: serverURL}
	_, err := s.db.NewInsert().
		Model(setting).
		Column("key", "value").
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	return err
}
