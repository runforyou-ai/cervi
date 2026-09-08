//go:build server

package integrationconnection

import (
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeInput 验证连接器输入完成裁剪并保留有效配置。
func TestNormalizeInput(t *testing.T) {
	input, fields := normalizeInput(Input{
		Type: domain.IntegrationConnectionTypeDify,
		Name: "  客服应用  ", Description: "  处理售前咨询  ",
		Configuration: Configuration{APIURL: "  https://api.dify.ai/v1  ", APIKey: "  app-key  "},
	})
	if len(fields) != 0 {
		t.Fatalf("unexpected validation fields: %v", fields)
	}
	if input.Name != "客服应用" || input.Description != "处理售前咨询" {
		t.Fatalf("unexpected normalized text: %#v", input)
	}
	if input.Configuration.APIURL != "https://api.dify.ai/v1" || input.Configuration.APIKey != "app-key" {
		t.Fatalf("unexpected normalized configuration: %#v", input.Configuration)
	}
}

// TestNormalizeRAGFlowInput 验证 RAGFlow 保存和草稿测试接受相同的实例配置。
func TestNormalizeRAGFlowInput(t *testing.T) {
	configuration := Configuration{APIURL: " http://ragflow.local/instance/ ", APIKey: " ragflow-test-key "}
	input, fields := normalizeInput(Input{
		Type: domain.IntegrationConnectionTypeRAGFlow, Name: "RAGFlow", Configuration: configuration,
	})
	if len(fields) != 0 || input.Configuration.APIURL != "http://ragflow.local/instance/" || input.Configuration.APIKey != "ragflow-test-key" {
		t.Fatalf("unexpected saved configuration: %#v, fields: %v", input, fields)
	}
	draft, fields := normalizeConnectionInput(ConnectionInput{
		Type: domain.IntegrationConnectionTypeRAGFlow, Configuration: configuration,
	})
	if len(fields) != 0 || draft.Type != input.Type || draft.Configuration != input.Configuration {
		t.Fatalf("unexpected draft configuration: %#v, fields: %v", draft, fields)
	}
}
