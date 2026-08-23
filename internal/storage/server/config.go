//go:build server

package server

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHost            = "127.0.0.1"
	defaultPort            = 5432
	defaultUser            = "cervi"
	defaultPassword        = "cervi_local_dev"
	defaultDatabaseName    = "cervi"
	defaultSSLMode         = "disable"
	defaultMaxOpenConns    = 8
	defaultMaxIdleConns    = 2
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultStartupTimeout  = time.Minute
)

// Config 定义 PostgreSQL 连接及连接池配置。
type Config struct {
	DSN             string        // PostgreSQL 连接字符串。
	MaxOpenConns    int           // 最大连接数；0 表示不限制。
	MaxIdleConns    int           // 最大空闲连接数；0 表示不保留空闲连接。
	ConnMaxLifetime time.Duration // 单个连接可复用的最长时间。
	ConnMaxIdleTime time.Duration // 空闲连接在池中的最长保留时间。
	StartupTimeout  time.Duration // 连接检查和迁移共用的超时时间。
}

// ConfigFromEnv 读取 PostgreSQL 连接参数并应用固定连接配置。
func ConfigFromEnv() (Config, error) {
	host := stringFromEnv("POSTGRES_HOST", defaultHost)
	portValue := stringFromEnv("POSTGRES_PORT", strconv.Itoa(defaultPort))
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("POSTGRES_PORT must be an integer between 1 and 65535")
	}
	user := stringFromEnv("POSTGRES_USER", defaultUser)
	password := stringFromEnv("POSTGRES_PASSWORD", defaultPassword)
	databaseName := stringFromEnv("POSTGRES_DB", defaultDatabaseName)
	sslMode := stringFromEnv("POSTGRES_SSLMODE", defaultSSLMode)

	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, portValue),
		Path:   databaseName,
	}
	query := databaseURL.Query()
	query.Set("sslmode", sslMode)
	databaseURL.RawQuery = query.Encode()

	return Config{
		DSN:             databaseURL.String(),
		MaxOpenConns:    defaultMaxOpenConns,
		MaxIdleConns:    defaultMaxIdleConns,
		ConnMaxLifetime: defaultConnMaxLifetime,
		ConnMaxIdleTime: defaultConnMaxIdleTime,
		StartupTimeout:  defaultStartupTimeout,
	}, nil
}

// stringFromEnv 读取非空字符串环境变量。
func stringFromEnv(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
