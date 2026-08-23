//go:build server

package server

import (
	"net/url"
	"testing"
)

// TestConfigFromEnvBuildsPostgreSQLURL 验证连接参数会被安全编码为 PostgreSQL URL。
func TestConfigFromEnvBuildsPostgreSQLURL(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "database.internal")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "cervi")
	t.Setenv("POSTGRES_PASSWORD", "secret@value")
	t.Setenv("POSTGRES_DB", "cervi_test")
	t.Setenv("POSTGRES_SSLMODE", "require")

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(config.DSN)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if parsed.Hostname() != "database.internal" ||
		parsed.Port() != "5433" ||
		parsed.User.Username() != "cervi" ||
		password != "secret@value" ||
		parsed.Path != "/cervi_test" ||
		parsed.Query().Get("sslmode") != "require" {
		t.Fatalf("PostgreSQL URL = %q", config.DSN)
	}
	if config.MaxOpenConns != postgresMaxOpenConns || config.MaxIdleConns != postgresMaxIdleConns {
		t.Fatalf("PostgreSQL 连接池 = %d/%d", config.MaxOpenConns, config.MaxIdleConns)
	}
}

// TestConfigFromEnvRequiresPostgreSQLHost 验证缺少连接参数时直接失败。
func TestConfigFromEnvRequiresPostgreSQLHost(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("缺少 POSTGRES_HOST 时未返回错误")
	}
}
