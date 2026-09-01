//go:build server

// Package server 实现基于 PostgreSQL 和 NATS JetStream 的服务端任务运行时。
package server

import (
	"strings"
	"time"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
)

const (
	natsStartupTimeout    = 15 * time.Second
	taskStreamMaxBytes    = int64(1 << 30)
	taskStreamMaxAge      = 30 * 24 * time.Hour
	taskReplicas          = 1
	standardTaskWorkers   = 4
	agentTaskWorkers      = 2
	taskPoolMaxAckPending = 1024
	workerPoolStandard    = "standard"
	workerPoolAgent       = "agent"
)

// workerPoolConfig 定义一组相互隔离的任务 Worker。
type workerPoolConfig struct {
	Name          string
	Workers       int
	MaxAckPending int
}

// runtimeConfig 定义服务端任务运行时配置。
type runtimeConfig struct {
	URL            string
	Namespace      string
	StartupTimeout time.Duration
	MaxBytes       int64
	MaxAge         time.Duration
	Replicas       int
	WorkerPools    []workerPoolConfig
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
		WorkerPools: []workerPoolConfig{
			{Name: workerPoolStandard, Workers: standardTaskWorkers, MaxAckPending: taskPoolMaxAckPending},
			{Name: workerPoolAgent, Workers: agentTaskWorkers, MaxAckPending: taskPoolMaxAckPending},
		},
	}
}

// streamName 生成任务 Stream 名称。
func (c runtimeConfig) streamName() string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_TASKS"
}

// consumerName 生成指定 Worker Pool 的 Consumer 名称。
func (c runtimeConfig) consumerName(pool string) string {
	return "CERVI_" + strings.ToUpper(c.Namespace) + "_" + strings.ToUpper(pool) + "_WORKERS"
}

// subjectPrefix 生成任务 Subject 前缀。
func (c runtimeConfig) subjectPrefix() string {
	return "cervi." + c.Namespace + ".tasks"
}

// filterSubject 生成指定 Worker Pool 的订阅过滤条件。
func (c runtimeConfig) filterSubject(pool string) string {
	return c.subjectPrefix() + "." + pool + ".>"
}

// taskSubject 生成指定逻辑队列的发布 Subject。
func (c runtimeConfig) taskSubject(queue string) string {
	return c.subjectPrefix() + "." + workerPoolForQueue(queue) + "." + queue
}

// workerPoolForQueue 返回逻辑队列所属的 Worker Pool。
func workerPoolForQueue(queue string) string {
	if queue == QueueAgent {
		return workerPoolAgent
	}
	return workerPoolStandard
}
