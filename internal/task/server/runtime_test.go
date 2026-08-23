//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/task"
)

// TestConfigNamespaceNames 验证一个命名空间同时隔离所有 JetStream 资源。
func TestConfigNamespaceNames(t *testing.T) {
	config := runtimeConfig{Namespace: "feature_one"}
	if config.streamName() != "CERVI_FEATURE_ONE_TASKS" {
		t.Fatalf("stream name = %q", config.streamName())
	}
	if config.consumerName() != "CERVI_FEATURE_ONE_WORKERS" {
		t.Fatalf("consumer name = %q", config.consumerName())
	}
	if config.subjectPrefix() != "cervi.feature_one.tasks" {
		t.Fatalf("subject prefix = %q", config.subjectPrefix())
	}
	if config.streamName() == (runtimeConfig{Namespace: "feature-one"}).streamName() {
		t.Fatal("different namespaces share a stream name")
	}
}

// TestRegisterJSONMarksInvalidPayloadPermanent 验证损坏的任务输入不会无效重试。
func TestRegisterJSONMarksInvalidPayloadPermanent(t *testing.T) {
	type input struct {
		Value string `json:"value"`
	}
	registry := NewRegistry()
	if err := RegisterJSON(registry, "test.action", func(context.Context, input) error { return nil }); err != nil {
		t.Fatal(err)
	}
	handler, exists := registry.lookup("test.action")
	if !exists {
		t.Fatal("registered handler not found")
	}
	err := handler(context.Background(), []byte(`{"value":`))
	if !task.IsPermanent(err) {
		t.Fatalf("invalid payload error = %v, want permanent", err)
	}
}

// TestPermanentPreservesCause 验证永久错误仍支持错误链判断。
func TestPermanentPreservesCause(t *testing.T) {
	cause := errors.New("invalid input")
	if err := task.Permanent(cause); !errors.Is(err, cause) {
		t.Fatalf("permanent error does not preserve cause: %v", err)
	}
}

// TestExecuteHandlerRecoversPanic 验证单个 Action panic 不会终止 Worker 进程。
func TestExecuteHandlerRecoversPanic(t *testing.T) {
	err := executeHandler(context.Background(), func(context.Context, json.RawMessage) error {
		panic("broken action")
	}, json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "broken action") {
		t.Fatalf("panic error = %v", err)
	}
}

// TestTaskFinalizationContextIgnoresCancellation 验证停机后仍能提交任务终态。
func TestTaskFinalizationContextIgnoresCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	cancelParent()
	ctx, cancel := taskFinalizationContext(parent)
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Fatalf("finalization context is already cancelled: %v", err)
	}
}

// TestMessageRecoveryAgeKeepsSafetyWindow 验证消息会在保留期结束前进入补偿窗口。
func TestMessageRecoveryAgeKeepsSafetyWindow(t *testing.T) {
	maxAge := 30 * 24 * time.Hour
	if got, want := messageRecoveryAge(maxAge), maxAge-taskMessageDuplicateWindow; got != want {
		t.Fatalf("message recovery age = %s, want %s", got, want)
	}
	shortMaxAge := 30 * time.Minute
	if got, want := messageRecoveryAge(shortMaxAge), 27*time.Minute; got != want {
		t.Fatalf("short message recovery age = %s, want %s", got, want)
	}
}

// TestResolveExecutionErrorPreservesHandlerResult 验证心跳错误不会覆盖已经完成的 Action 结果。
func TestResolveExecutionErrorPreservesHandlerResult(t *testing.T) {
	heartbeatErr := errors.New("lease lost")
	if err := resolveExecutionError(nil, heartbeatErr); err != nil {
		t.Fatalf("successful handler result was replaced: %v", err)
	}
	permanentErr := task.Permanent(errors.New("invalid input"))
	if err := resolveExecutionError(permanentErr, heartbeatErr); !errors.Is(err, permanentErr) {
		t.Fatalf("permanent handler result was replaced: %v", err)
	}
	permanentCancel := task.Permanent(context.Canceled)
	if err := resolveExecutionError(permanentCancel, heartbeatErr); !errors.Is(err, permanentCancel) {
		t.Fatalf("permanent cancellation was replaced: %v", err)
	}
	if err := resolveExecutionError(context.Canceled, heartbeatErr); !errors.Is(err, heartbeatErr) {
		t.Fatalf("cancelled handler result = %v, want heartbeat error", err)
	}
}
