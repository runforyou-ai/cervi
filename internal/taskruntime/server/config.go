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
	natsStartupTimeout = 15 * time.Second
	taskStreamMaxBytes = int64(1 << 30)
	taskStreamMaxAge   = 30 * 24 * time.Hour
	taskReplicas       = 1
	taskWorkers        = 4
	taskMaxAckPending  = 1024
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

// ConfigFromEnv 读取 NATS 地址和任务命名空间。
func ConfigFromEnv() (Config, error) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" {
		return Config{}, fmt.Errorf("NATS_URL is required")
	}
	namespace := strings.TrimSpace(os.Getenv("NATS_NAMESPACE"))
	if !namespacePattern.MatchString(namespace) {
		return Config{}, fmt.Errorf("NATS_NAMESPACE must match %s", namespacePattern.String())
	}

	return Config{
		URL:            natsURL,
		Namespace:      namespace,
		StartupTimeout: natsStartupTimeout,
		MaxBytes:       taskStreamMaxBytes,
		MaxAge:         taskStreamMaxAge,
		Replicas:       taskReplicas,
		Workers:        taskWorkers,
		MaxAckPending:  taskMaxAckPending,
	}, nil
}

// streamName 生成任务 Stream 名称。
func (c Config) streamName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_TASKS"
}

// consumerName 生成任务 Consumer 名称。
func (c Config) consumerName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_WORKERS"
}

// subjectPrefix 生成任务 Subject 前缀。
func (c Config) subjectPrefix() string {
	return "cervi." + c.Namespace + ".tasks"
}
