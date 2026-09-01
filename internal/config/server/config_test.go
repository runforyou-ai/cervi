//go:build server

package server

import (
	"os"
	"path/filepath"
	"testing"
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
  host: file.internal
  port: 5432
  user: file
  password: file-secret
  name: file
  sslMode: disable
nats:
  url: nats://file:4222
  namespace: file
tls:
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
	t.Setenv("NATS_URL", "nats://environment:4222")
	t.Setenv("NATS_NAMESPACE", "environment")
	t.Setenv("WAILS_SERVER_PORT", "28080")
	t.Setenv("TLS_MODE", "external")
	t.Setenv("TLS_ACME_EMAIL", "admin@example.com")
	t.Setenv("FILE_STORAGE_PATH", t.TempDir())

	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.Host != "database.internal" || config.Database.Port != 5433 || config.Database.User != "environment" || config.Database.Password != "secret@value" || config.Database.Name != "cervi" || config.Database.SSLMode != "require" || config.Server.Port != 28080 {
		t.Fatalf("环境变量未覆盖文件配置: %#v", config)
	}
	if config.TLS.Mode != "external" || config.TLS.ACMEEmail != "admin@example.com" {
		t.Fatalf("TLS 环境变量未覆盖文件配置: %#v", config.TLS)
	}
	if config.NATS.URL != "nats://environment:4222" || config.NATS.Namespace != "environment" {
		t.Fatalf("NATS 环境变量未覆盖文件配置: %#v", config.NATS)
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

// TestValidationRequiresDatabaseName 验证必须指定数据库名称。
func TestValidationRequiresDatabaseName(t *testing.T) {
	config := validTestConfig()
	config.Database.Name = ""
	config.normalize()
	if err := config.validate(); err == nil {
		t.Fatal("接受了未指定数据库名称的配置")
	}
}

// TestValidationRejectsInvalidNATSConfig 验证 NATS 地址和命名空间。
func TestValidationRejectsInvalidNATSConfig(t *testing.T) {
	for _, nats := range []NATSConfig{
		{Namespace: "cervi"},
		{URL: "nats://127.0.0.1:4222", Namespace: "INVALID"},
	} {
		config := validTestConfig()
		config.NATS = nats
		config.normalize()
		if err := config.validate(); err == nil {
			t.Fatalf("接受了无效 NATS 配置: %#v", nats)
		}
	}
}

// clearServerEnvironment 清除可能影响配置测试的服务端环境变量。
func clearServerEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"WAILS_SERVER_HOST", "WAILS_SERVER_PORT",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSLMODE",
		"NATS_URL", "NATS_NAMESPACE",
		"TLS_MODE", "TLS_ACME_EMAIL", "FILE_STORAGE_PATH",
	} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// TestValidationRejectsAutoTLSPortConflict 验证 TLS 自动模式端口不会与服务监听器冲突。
func TestValidationRejectsAutoTLSPortConflict(t *testing.T) {
	config := validTestConfig()
	config.Server.Port = 443
	config.TLS.Mode = "auto"
	config.Storage.LocalDirectory = t.TempDir()
	config.normalize()
	if err := config.validate(); err == nil {
		t.Fatal("TLS 自动模式接受了 443 服务监听端口")
	}
}

// validTestConfig 返回满足基础校验的服务端测试配置。
func validTestConfig() Config {
	config := defaultConfig()
	config.Database.Host = "127.0.0.1"
	config.Database.Port = 5432
	config.Database.User = "cervi"
	config.Database.Password = "secret"
	config.Database.Name = "cervi"
	config.Database.SSLMode = "disable"
	config.NATS.URL = "nats://127.0.0.1:4222"
	config.NATS.Namespace = "cervi"
	return config
}
