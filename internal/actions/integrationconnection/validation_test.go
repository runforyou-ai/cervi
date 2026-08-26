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
