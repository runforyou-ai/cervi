//go:build !server && !ios && !android

package desktop

import (
	"context"
	"database/sql"
	"errors"
	"time"

	desktopmodels "github.com/runforyou-ai/cervi/internal/storage/desktop/models"
	"uuid"
)

const deviceInstallIDSettingKey = "device_install_id"

// DeviceInstallID 读取本机安装标识，尚未生成时创建并保存。
func (s *Store) DeviceInstallID(ctx context.Context) (string, error) {
	setting := &desktopmodels.AppSetting{}
	err := s.db.NewSelect().
		Model(setting).
		Where("key = ?", deviceInstallIDSettingKey).
		Scan(ctx)
	if err == nil {
		return setting.Value, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	installID := uuid.NewV7().String()
	created := &desktopmodels.AppSetting{
		Key:       deviceInstallIDSettingKey,
		Value:     installID,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	// 并发写入时保留先落库的标识，本机安装标识始终唯一。
	if _, err := s.db.NewInsert().
		Model(created).
		Column("key", "value", "updated_at").
		On("CONFLICT (key) DO NOTHING").
		Exec(ctx); err != nil {
		return "", err
	}
	stored := &desktopmodels.AppSetting{}
	if err := s.db.NewSelect().
		Model(stored).
		Where("key = ?", deviceInstallIDSettingKey).
		Scan(ctx); err != nil {
		return "", err
	}
	return stored.Value, nil
}

// LoadDeviceRegistration 读取本机在指定企业服务器上的设备编号。
func (s *Store) LoadDeviceRegistration(ctx context.Context, serverURL, organizationID string) (string, bool, error) {
	registration := &desktopmodels.DeviceRegistration{}
	err := s.db.NewSelect().
		Model(registration).
		Where("server_url = ?", serverURL).
		Where("organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return registration.DeviceID, true, nil
}

// SaveDeviceRegistration 保存本机在指定企业服务器上的设备编号。
func (s *Store) SaveDeviceRegistration(ctx context.Context, serverURL, organizationID, deviceID string) error {
	registration := &desktopmodels.DeviceRegistration{
		ServerURL:      serverURL,
		OrganizationID: organizationID,
		DeviceID:       deviceID,
		RegisteredAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.db.NewInsert().
		Model(registration).
		Column("server_url", "organization_id", "device_id", "registered_at").
		On("CONFLICT (server_url, organization_id) DO UPDATE").
		Set("device_id = EXCLUDED.device_id").
		Set("registered_at = EXCLUDED.registered_at").
		Exec(ctx)
	return err
}
