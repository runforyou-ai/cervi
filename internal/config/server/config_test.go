//go:build server

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadMergesFileAndEnvironment 验证环境变量覆盖显式配置文件。
func TestLoadMergesFileAndEnvironment(t *testing.T) {
	clearServerEnvironment(t)
	path := filepath.Join(t.TempDir(), "cervi.yaml")
	data := []byte(`
server:
  host: 127.0.0.1
  port: 18080
database:
  url: postgres://file
  migrationTimeout: 15m
https:
  mode: off
storage:
  localDirectory: data/files
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POSTGRES_HOST", "database.internal")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "environment")
	t.Setenv("POSTGRES_PASSWORD", "secret@value")
	t.Setenv("POSTGRES_DB", "cervi")
	t.Setenv("POSTGRES_SSLMODE", "require")
	t.Setenv("WAILS_SERVER_PORT", "28080")
	t.Setenv("TLS_MODE", "external")
	t.Setenv("TLS_DATA_DIR", "/var/lib/cervi/tls")
	t.Setenv("TLS_ACME_EMAIL", "admin@example.com")
	t.Setenv("FILE_STORAGE_PATH", t.TempDir())

	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.URL != "postgres://environment:secret%40value@database.internal:5433/cervi?sslmode=require" || config.Server.Port != 28080 {
		t.Fatalf("环境变量未覆盖文件配置: %#v", config)
	}
	if config.HTTPS.Mode != "external" || config.HTTPS.TLSDataDirectory != "/var/lib/cervi/tls" || config.HTTPS.ACMEEmail != "admin@example.com" {
		t.Fatalf("TLS 环境变量未覆盖文件配置: %#v", config.HTTPS)
	}
	if config.Database.MigrationTimeout.Value() != 15*time.Minute {
		t.Fatalf("迁移超时 = %s", config.Database.MigrationTimeout.Value())
	}
}

// TestLoadRejectsUnknownFileField 验证配置文件会拒绝未知字段。
func TestLoadRejectsUnknownFileField(t *testing.T) {
	clearServerEnvironment(t)
	path := filepath.Join(t.TempDir(), "cervi.yaml")
	if err := os.WriteFile(path, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("未知配置字段未被拒绝")
	}
}

// TestValidationRequiresDatabaseName 验证连接地址必须指定数据库名称。
func TestValidationRequiresDatabaseName(t *testing.T) {
	config := defaultConfig()
	config.Database.URL = "postgres://cervi@localhost"
	config.normalize()
	if err := config.validate(); err == nil {
		t.Fatal("接受了未指定数据库名称的连接地址")
	}
}

// TestDefaultTLSModeIsOff 验证 TLS 默认关闭。
func TestDefaultTLSModeIsOff(t *testing.T) {
	if mode := defaultConfig().HTTPS.Mode; mode != "off" {
		t.Fatalf("TLS 默认模式 = %q", mode)
	}
}

// clearServerEnvironment 清除可能影响配置测试的服务端环境变量。
func clearServerEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"WAILS_SERVER_HOST", "WAILS_SERVER_PORT",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSLMODE",
		"TLS_MODE", "TLS_DATA_DIR", "TLS_ACME_EMAIL", "FILE_STORAGE_PATH",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// TestValidationRejectsAutoHTTPSPortConflict 验证自动 HTTPS 端口不会与服务监听器冲突。
func TestValidationRejectsAutoHTTPSPortConflict(t *testing.T) {
	config := defaultConfig()
	config.Server.Port = 443
	config.Database.URL = "postgres://cervi@localhost/cervi"
	config.HTTPS.Mode = "auto"
	config.HTTPS.TLSDataDirectory = t.TempDir()
	config.Storage.LocalDirectory = t.TempDir()
	config.normalize()
	if err := config.validate(); err == nil {
		t.Fatal("自动 HTTPS 接受了 443 服务监听端口")
	}
}
