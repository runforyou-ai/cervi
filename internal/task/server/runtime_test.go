//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/task"
)

// TestRegisterJSONMarksInvalidPayloadPermanent 验证损坏的任务输入不会无效重试。
func TestRegisterJSONMarksInvalidPayloadPermanent(t *testing.T) {
	type input struct {
		Value string `json:"value"`
	}
	registry := NewRegistry()
	if err := registry.RegisterJSON("test.action", func(context.Context, input) error { return nil }); err != nil {
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

// TestRegisterJSONWithTerminalFailure 解码并执行最终失败业务收尾。
func TestRegisterJSONWithTerminalFailure(t *testing.T) {
	type input struct {
		Value string `json:"value"`
	}
	registry := NewRegistry()
	var finalized string
	terminalErr := errors.New("attempts exhausted")
	if err := registry.RegisterJSONWithTerminalFailure(
		"test.finalized_action",
		func(context.Context, input) error { return nil },
		func(_ context.Context, value input, runErr error) error {
			if !errors.Is(runErr, terminalErr) {
				t.Fatalf("terminal error = %v", runErr)
			}
			finalized = value.Value
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	finalize, exists := registry.lookupTerminalFailure("test.finalized_action")
	if !exists {
		t.Fatal("terminal failure handler not found")
	}
	if err := finalize(context.Background(), json.RawMessage(`{"value":"done"}`), terminalErr); err != nil {
		t.Fatal(err)
	}
	if finalized != "done" {
		t.Fatalf("finalized input = %q", finalized)
	}
	if err := finalize(context.Background(), json.RawMessage(`{"value":`), terminalErr); err != nil {
		t.Fatalf("malformed terminal payload should not block task finalization: %v", err)
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
