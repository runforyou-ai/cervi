//go:build server

package server

import (
	"testing"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
)

// TestNewConfigAddsRuntimeDefaults 验证 NATS 身份和固定任务配置。
func TestNewConfigAddsRuntimeDefaults(t *testing.T) {
	config := newConfig(serverconfig.NATSConfig{URL: "nats://127.0.0.1:4222", Namespace: "test_runtime"})
	if config.URL != "nats://127.0.0.1:4222" || config.Namespace != "test_runtime" {
		t.Fatalf("NATS 配置 = %#v", config)
	}
	if config.MaxBytes != taskStreamMaxBytes ||
		config.MaxAge != taskStreamMaxAge ||
		config.Replicas != taskReplicas {
		t.Fatalf("NATS 任务配置 = %#v", config)
	}
	if len(config.WorkerPools) != 2 {
		t.Fatalf("Worker Pool 数量 = %d，期望 2", len(config.WorkerPools))
	}
	if standard, agent := config.WorkerPools[0], config.WorkerPools[1]; standard.Name != workerPoolStandard || standard.Workers != standardTaskWorkers || standard.MaxAckPending != taskPoolMaxAckPending ||
		agent.Name != workerPoolAgent || agent.Workers != agentTaskWorkers || agent.MaxAckPending != taskPoolMaxAckPending {
		t.Fatalf("Worker Pool 配置 = %#v", config.WorkerPools)
	}
}

// TestConfigBuildsIsolatedConsumerNames 验证一个命名空间内的 Worker Pool 使用独立 Consumer 和 Subject。
func TestConfigBuildsIsolatedConsumerNames(t *testing.T) {
	config := runtimeConfig{Namespace: "feature_one"}
	if config.streamName() != "CERVI_FEATURE_ONE_TASKS" {
		t.Fatalf("stream name = %q", config.streamName())
	}
	if config.consumerName(workerPoolStandard) != "CERVI_FEATURE_ONE_STANDARD_WORKERS" {
		t.Fatalf("standard consumer name = %q", config.consumerName(workerPoolStandard))
	}
	if config.consumerName(workerPoolAgent) != "CERVI_FEATURE_ONE_AGENT_WORKERS" {
		t.Fatalf("agent consumer name = %q", config.consumerName(workerPoolAgent))
	}
	if config.filterSubject(workerPoolStandard) != "cervi.feature_one.tasks.standard.>" {
		t.Fatalf("standard filter subject = %q", config.filterSubject(workerPoolStandard))
	}
	if config.filterSubject(workerPoolAgent) != "cervi.feature_one.tasks.agent.>" {
		t.Fatalf("agent filter subject = %q", config.filterSubject(workerPoolAgent))
	}
}

// TestTaskSubjectRoutesOnlyAgentQueueToAgentPool 验证未知队列仍由标准 Worker Pool 消费。
func TestTaskSubjectRoutesOnlyAgentQueueToAgentPool(t *testing.T) {
	config := runtimeConfig{Namespace: "test_runtime"}
	if subject := config.taskSubject(QueueAgent); subject != "cervi.test_runtime.tasks.agent.agent" {
		t.Fatalf("Agent Subject = %q", subject)
	}
	for _, queue := range []string{defaultQueue, "files", "maintenance", "future_queue"} {
		want := "cervi.test_runtime.tasks.standard." + queue
		if subject := config.taskSubject(queue); subject != want {
			t.Fatalf("队列 %q Subject = %q，期望 %q", queue, subject, want)
		}
	}
}
