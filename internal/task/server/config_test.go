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
		config.Replicas != taskReplicas ||
		config.Workers != taskWorkers ||
		config.MaxAckPending != taskMaxAckPending {
		t.Fatalf("NATS 任务配置 = %#v", config)
	}
}
