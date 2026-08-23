//go:build server

// Package server 实现基于 PostgreSQL 和 NATS JetStream 的服务端任务运行时。
package server

import (
	"fmt"
	"os"
	"regexp"
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

// ConfigFromEnv 从环境变量读取 NATS 地址和命名空间并应用固定任务配置。
func ConfigFromEnv() (Config, error) {
	config := Config{
		URL:            strings.TrimSpace(os.Getenv("NATS_URL")),
		Namespace:      strings.TrimSpace(os.Getenv("NATS_NAMESPACE")),
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
		return Config{}, fmt.Errorf("NATS_NAMESPACE must match %s", namespacePattern.String())
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
