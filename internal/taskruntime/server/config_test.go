//go:build server

package server

import "testing"

// TestConfigFromEnvLoadsNATSIdentity 验证 NATS 地址、命名空间和固定任务配置。
func TestConfigFromEnvLoadsNATSIdentity(t *testing.T) {
	t.Setenv("NATS_URL", "nats://127.0.0.1:4222")
	t.Setenv("NATS_NAMESPACE", "test_runtime")

	config, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
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

// TestConfigFromEnvRequiresNATSURL 验证缺少 NATS 地址时直接失败。
func TestConfigFromEnvRequiresNATSURL(t *testing.T) {
	t.Setenv("NATS_URL", "")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("缺少 NATS_URL 时未返回错误")
	}
}
