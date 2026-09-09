//go:build server

package server

import "testing"

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

// TestTaskSubjectRoutesDedicatedQueues 验证专用队列独立消费，未知队列仍使用标准 Worker Pool。
func TestTaskSubjectRoutesDedicatedQueues(t *testing.T) {
	config := runtimeConfig{Namespace: "test_runtime"}
	if subject := config.taskSubject(QueueAgent); subject != "cervi.test_runtime.tasks.agent.agent" {
		t.Fatalf("Agent Subject = %q", subject)
	}
	if subject := config.taskSubject(QueueKnowledge); subject != "cervi.test_runtime.tasks.knowledge.knowledge" {
		t.Fatalf("knowledge subject=%s", subject)
	}
	for _, queue := range []string{defaultQueue, "files", "maintenance", "future_queue"} {
		want := "cervi.test_runtime.tasks.standard." + queue
		if subject := config.taskSubject(queue); subject != want {
			t.Fatalf("队列 %q Subject = %q，期望 %q", queue, subject, want)
		}
	}
}
