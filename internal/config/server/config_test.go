//go:build server

// 本文件验证企业服务端配置的加载、覆盖与生产约束。
package serverconfig

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
environment: development
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
	t.Setenv("DATABASE_URL", "postgres://environment")
	t.Setenv("WAILS_SERVER_PORT", "28080")

	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.URL != "postgres://environment" || config.Server.Port != 28080 {
		t.Fatalf("环境变量未覆盖文件配置: %#v", config)
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

// clearServerEnvironment 清除可能影响配置测试的服务端环境变量。
func clearServerEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CERVI_ENV", "WAILS_SERVER_HOST", "WAILS_SERVER_PORT", "DATABASE_URL",
		"POSTGRES_MIGRATION_TIMEOUT",
		"CERVI_HTTPS_MODE", "CERVI_TLS_DATA_DIR",
		"CERVI_ACME_EMAIL", "FILE_STORAGE_PATH",
	} {
		t.Setenv(name, "")
	}
}

// TestProductionValidationRejectsDevelopmentDefaults 验证生产配置不能继承开发数据库和相对目录。
func TestProductionValidationRejectsDevelopmentDefaults(t *testing.T) {
	config := defaultConfig()
	config.Environment = "production"
	config.normalize()
	if err := config.validate(false); err == nil {
		t.Fatal("生产环境接受了开发默认配置")
	}
}
