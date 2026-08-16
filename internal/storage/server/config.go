//go:build server

package server

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDSN             = "postgres://cervi:cervi_local_dev@localhost:5432/cervi?sslmode=disable"
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
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

// ConfigFromEnv 从环境变量读取并校验 PostgreSQL 配置。
func ConfigFromEnv() (Config, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		slog.Warn("未配置 DATABASE_URL，使用本地开发数据库")
		dsn = defaultDSN
	}

	config := Config{
		DSN:             dsn,
		MaxOpenConns:    defaultMaxOpenConns,
		MaxIdleConns:    defaultMaxIdleConns,
		ConnMaxLifetime: defaultConnMaxLifetime,
		ConnMaxIdleTime: defaultConnMaxIdleTime,
		StartupTimeout:  defaultStartupTimeout,
	}

	var err error
	if config.MaxOpenConns, err = intFromEnv("POSTGRES_MAX_OPEN_CONNS", config.MaxOpenConns); err != nil {
		return Config{}, err
	}
	if config.MaxIdleConns, err = intFromEnv("POSTGRES_MAX_IDLE_CONNS", config.MaxIdleConns); err != nil {
		return Config{}, err
	}
	if config.ConnMaxLifetime, err = durationFromEnv("POSTGRES_CONN_MAX_LIFETIME", config.ConnMaxLifetime); err != nil {
		return Config{}, err
	}
	if config.ConnMaxIdleTime, err = durationFromEnv("POSTGRES_CONN_MAX_IDLE_TIME", config.ConnMaxIdleTime); err != nil {
		return Config{}, err
	}
	if config.StartupTimeout, err = durationFromEnv("POSTGRES_STARTUP_TIMEOUT", config.StartupTimeout); err != nil {
		return Config{}, err
	}
	if config.MaxOpenConns > 0 && config.MaxIdleConns > config.MaxOpenConns {
		return Config{}, fmt.Errorf("POSTGRES_MAX_IDLE_CONNS must not exceed POSTGRES_MAX_OPEN_CONNS")
	}

	return config, nil
}

// intFromEnv 读取非负整数环境变量。
func intFromEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

// durationFromEnv 读取正数时长环境变量。
func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration (for example 30s or 5m)", name)
	}
	return parsed, nil
}
