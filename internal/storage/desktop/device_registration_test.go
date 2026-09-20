//go:build !server && !ios && !android

package desktop

import (
	"context"
	"path/filepath"
	"testing"
)

// TestDeviceInstallIDStaysStable 验证本机安装标识生成一次后跨连接保持不变。
func TestDeviceInstallIDStaysStable(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "cervi.db")
	store, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	installID, err := store.DeviceInstallID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if installID == "" {
		t.Fatal("install ID is empty")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	reopened, err := store.DeviceInstallID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reopened != installID {
		t.Fatalf("install ID = %q, want %q", reopened, installID)
	}
}

// TestDeviceRegistrationPersistsPerOrganization 验证设备注册结果按企业服务器保存且可覆盖。
func TestDeviceRegistrationPersistsPerOrganization(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "cervi.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	const serverURL = "https://cervi.example.com"
	if _, found, err := store.LoadDeviceRegistration(ctx, serverURL, "org-1"); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("found a registration before saving one")
	}

	if err := store.SaveDeviceRegistration(ctx, serverURL, "org-1", "device-1"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDeviceRegistration(ctx, serverURL, "org-2", "device-2"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDeviceRegistration(ctx, serverURL, "org-1", "device-3"); err != nil {
		t.Fatal(err)
	}

	deviceID, found, err := store.LoadDeviceRegistration(ctx, serverURL, "org-1")
	if err != nil {
		t.Fatal(err)
	}
	if !found || deviceID != "device-3" {
		t.Fatalf("org-1 device = %q, found = %v", deviceID, found)
	}
	deviceID, found, err = store.LoadDeviceRegistration(ctx, serverURL, "org-2")
	if err != nil {
		t.Fatal(err)
	}
	if !found || deviceID != "device-2" {
		t.Fatalf("org-2 device = %q, found = %v", deviceID, found)
	}
}
