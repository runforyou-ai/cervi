//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"errors"
	"time"

	desktopmodels "github.com/runforyou-ai/cervi/internal/storage/desktop/models"
)

const serverURLSettingKey = "server_url"

// GetServerURL 读取桌面端保存的企业服务器地址。
func (s *Store) GetServerURL(ctx context.Context) (string, error) {
	setting := &desktopmodels.AppSetting{}
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

// SetServerURL 保存或更新桌面端企业服务器地址。
func (s *Store) SetServerURL(ctx context.Context, serverURL string) error {
	setting := &desktopmodels.AppSetting{
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
