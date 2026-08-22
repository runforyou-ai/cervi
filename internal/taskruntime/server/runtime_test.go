//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/taskruntime"
)

// TestConfigNamespaceNames 验证一个命名空间同时隔离所有 JetStream 资源。
func TestConfigNamespaceNames(t *testing.T) {
	config := Config{Namespace: "feature_one"}
	if config.streamName() != "CERVI_FEATURE_ONE_TASKS" {
		t.Fatalf("stream name = %q", config.streamName())
	}
	if config.consumerName() != "CERVI_FEATURE_ONE_WORKERS" {
		t.Fatalf("consumer name = %q", config.consumerName())
	}
	if config.subjectPrefix() != "cervi.feature_one.tasks" {
		t.Fatalf("subject prefix = %q", config.subjectPrefix())
	}
	if config.streamName() == (Config{Namespace: "feature-one"}).streamName() {
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
	if !taskruntime.IsPermanent(err) {
		t.Fatalf("invalid payload error = %v, want permanent", err)
	}
}

// TestPermanentPreservesCause 验证永久错误仍支持错误链判断。
func TestPermanentPreservesCause(t *testing.T) {
	cause := errors.New("invalid input")
	if err := taskruntime.Permanent(cause); !errors.Is(err, cause) {
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

// TestPayloadsEqualIgnoresJSONFormatting 验证计划输入比较不受 JSONB 格式影响。
func TestPayloadsEqualIgnoresJSONFormatting(t *testing.T) {
	if !payloadsEqual([]byte(`{"enabled":true,"count":1}`), []byte(`{ "count": 1, "enabled": true }`)) {
		t.Fatal("equivalent JSON payloads are different")
	}
}
