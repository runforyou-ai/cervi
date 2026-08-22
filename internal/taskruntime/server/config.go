//go:build server

// Package server 实现基于 PostgreSQL 和 NATS JetStream 的服务端任务运行时。
package server

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultNATSURL        = "nats://127.0.0.1:4222"
	defaultStartupTimeout = 15 * time.Second
	defaultMaxBytes       = int64(1 << 30)
	defaultMaxAge         = 30 * 24 * time.Hour
	defaultWorkers        = 4
	defaultMaxAckPending  = 1024
)

var namespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Config 定义服务端任务运行时配置。
type Config struct {
	URL            string
	Namespace      string
	StartupTimeout time.Duration
	MaxBytes       int64
	MaxAge         time.Duration
	Replicas       int
	Workers        int
	MaxAckPending  int
}

// ConfigFromEnv 从环境变量读取服务端任务运行时配置。
func ConfigFromEnv() (Config, error) {
	config := Config{
		URL:            strings.TrimSpace(os.Getenv("CERVI_NATS_URL")),
		Namespace:      strings.TrimSpace(os.Getenv("CERVI_NATS_NAMESPACE")),
		StartupTimeout: defaultStartupTimeout,
		MaxBytes:       defaultMaxBytes,
		MaxAge:         defaultMaxAge,
		Replicas:       1,
		Workers:        defaultWorkers,
		MaxAckPending:  defaultMaxAckPending,
	}
	if config.URL == "" {
		config.URL = defaultNATSURL
	}
	if !namespacePattern.MatchString(config.Namespace) {
		return Config{}, fmt.Errorf("CERVI_NATS_NAMESPACE must match %s", namespacePattern.String())
	}
	var err error
	if config.StartupTimeout, err = durationEnv("NATS_STARTUP_TIMEOUT", config.StartupTimeout); err != nil {
		return Config{}, err
	}
	if config.MaxAge, err = durationEnv("NATS_TASK_MAX_AGE", config.MaxAge); err != nil {
		return Config{}, err
	}
	if config.MaxBytes, err = int64Env("NATS_TASK_MAX_BYTES", config.MaxBytes); err != nil {
		return Config{}, err
	}
	if config.Replicas, err = intEnv("NATS_TASK_REPLICAS", config.Replicas); err != nil {
		return Config{}, err
	}
	if config.Workers, err = intEnv("NATS_TASK_WORKERS", config.Workers); err != nil {
		return Config{}, err
	}
	if config.MaxAckPending, err = intEnv("NATS_TASK_MAX_ACK_PENDING", config.MaxAckPending); err != nil {
		return Config{}, err
	}
	if config.StartupTimeout <= 0 || config.MaxBytes <= 0 || config.MaxAge <= 0 || config.Replicas <= 0 || config.Workers <= 0 || config.MaxAckPending <= 0 {
		return Config{}, fmt.Errorf("NATS task limits and durations must be positive")
	}
	if config.MaxAckPending < config.Workers {
		return Config{}, fmt.Errorf("NATS_TASK_MAX_ACK_PENDING must not be less than NATS_TASK_WORKERS")
	}
	return config, nil
}

// streamName 返回当前命名空间独占的 Stream 名称。
func (c Config) streamName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_TASKS"
}

// consumerName 返回当前命名空间独占的 Consumer 名称。
func (c Config) consumerName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_WORKERS"
}

// subjectPrefix 返回当前命名空间独占的 Subject 前缀。
func (c Config) subjectPrefix() string {
	return "cervi." + c.Namespace + ".tasks"
}

// durationEnv 读取可选时长环境变量。
func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

// intEnv 读取可选正整数环境变量。
func intEnv(name string, fallback int) (int, error) {
	value, err := int64Env(name, int64(fallback))
	return int(value), err
}

// int64Env 读取可选正整数环境变量。
func int64Env(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
