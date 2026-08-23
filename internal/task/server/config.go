//go:build server

// Package server 实现基于 PostgreSQL 和 NATS JetStream 的服务端任务运行时。
package server

import (
	"strings"
	"time"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
)

const (
	natsStartupTimeout = 15 * time.Second
	taskStreamMaxBytes = int64(1 << 30)
	taskStreamMaxAge   = 30 * 24 * time.Hour
	taskReplicas       = 1
	taskWorkers        = 4
	taskMaxAckPending  = 1024
)

// runtimeConfig 定义服务端任务运行时配置。
type runtimeConfig struct {
	URL            string
	Namespace      string
	StartupTimeout time.Duration
	MaxBytes       int64
	MaxAge         time.Duration
	Replicas       int
	Workers        int
	MaxAckPending  int
}

// newConfig 补充任务运行时的固定配置。
func newConfig(nats serverconfig.NATSConfig) runtimeConfig {
	return runtimeConfig{
		URL:            nats.URL,
		Namespace:      nats.Namespace,
		StartupTimeout: natsStartupTimeout,
		MaxBytes:       taskStreamMaxBytes,
		MaxAge:         taskStreamMaxAge,
		Replicas:       taskReplicas,
		Workers:        taskWorkers,
		MaxAckPending:  taskMaxAckPending,
	}
}

// streamName 生成任务 Stream 名称。
func (c runtimeConfig) streamName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_TASKS"
}

// consumerName 生成任务 Consumer 名称。
func (c runtimeConfig) consumerName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_WORKERS"
}

// subjectPrefix 生成任务 Subject 前缀。
func (c runtimeConfig) subjectPrefix() string {
	return "cervi." + c.Namespace + ".tasks"
}
