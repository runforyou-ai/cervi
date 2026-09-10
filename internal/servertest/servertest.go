//go:build server

// Package servertest 为服务端集成测试提供测试数据库连接配置。
package servertest

import (
	"os"
	"strconv"
	"strings"
	"testing"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
)

// DatabaseConfig 从测试专用的 PostgreSQL 分项环境变量读取连接配置。
//
// 未设置 TEST_POSTGRES_HOST 时跳过当前测试；数据库名必须以 _test 结尾，
// 防止集成测试误连开发库。
func DatabaseConfig(t *testing.T) serverconfig.DatabaseConfig {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	port, err := strconv.Atoi(os.Getenv("TEST_POSTGRES_PORT"))
	if err != nil {
		t.Fatalf("TEST_POSTGRES_PORT is invalid: %v", err)
	}
	databaseName := os.Getenv("TEST_POSTGRES_DB")
	if !strings.HasSuffix(databaseName, "_test") {
		t.Fatalf("TEST_POSTGRES_DB must end with _test")
	}
	return serverconfig.DatabaseConfig{
		Host:     host,
		Port:     port,
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
		Name:     databaseName,
		SSLMode:  os.Getenv("TEST_POSTGRES_SSLMODE"),
	}
}
