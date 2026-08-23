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
	postgresMaxOpenConns    = 8
	postgresMaxIdleConns    = 2
	postgresConnMaxLifetime = 30 * time.Minute
	postgresConnMaxIdleTime = 5 * time.Minute
	postgresStartupTimeout  = time.Minute
)

// Config 定义 PostgreSQL 连接及连接池配置。
type Config struct {
	DSN             string        // PostgreSQL 连接字符串。
	MaxOpenConns    int           // 最大连接数。
	MaxIdleConns    int           // 最大空闲连接数。
	ConnMaxLifetime time.Duration // 单个连接可复用的最长时间。
	ConnMaxIdleTime time.Duration // 空闲连接在池中的最长保留时间。
	StartupTimeout  time.Duration // 连接检查和迁移共用的超时时间。
}

// ConfigFromEnv 读取 PostgreSQL 连接参数并设置连接池。
func ConfigFromEnv() (Config, error) {
	host, err := requiredEnvironment("POSTGRES_HOST")
	if err != nil {
		return Config{}, err
	}
	portValue, err := requiredEnvironment("POSTGRES_PORT")
	if err != nil {
		return Config{}, err
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("POSTGRES_PORT must be an integer between 1 and 65535")
	}
	user, err := requiredEnvironment("POSTGRES_USER")
	if err != nil {
		return Config{}, err
	}
	password, err := requiredEnvironment("POSTGRES_PASSWORD")
	if err != nil {
		return Config{}, err
	}
	databaseName, err := requiredEnvironment("POSTGRES_DB")
	if err != nil {
		return Config{}, err
	}
	sslMode, err := requiredEnvironment("POSTGRES_SSLMODE")
	if err != nil {
		return Config{}, err
	}

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
		MaxOpenConns:    postgresMaxOpenConns,
		MaxIdleConns:    postgresMaxIdleConns,
		ConnMaxLifetime: postgresConnMaxLifetime,
		ConnMaxIdleTime: postgresConnMaxIdleTime,
		StartupTimeout:  postgresStartupTimeout,
	}, nil
}

// requiredEnvironment 读取必填环境变量。
func requiredEnvironment(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}
