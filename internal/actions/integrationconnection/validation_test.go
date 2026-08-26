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

// TestValidAPIURL 验证连接器地址只接受不含认证信息、查询参数和片段的 HTTP 地址。
func TestValidAPIURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "https", value: "https://example.com/api/v1", valid: true},
		{name: "http", value: "http://localhost:5678", valid: true},
		{name: "credentials", value: "https://user:password@example.com", valid: false},
		{name: "query", value: "https://example.com?tenant=1", valid: false},
		{name: "fragment", value: "https://example.com#api", valid: false},
		{name: "unsupported scheme", value: "ftp://example.com", valid: false},
		{name: "relative", value: "/api/v1", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := validAPIURL(test.value); actual != test.valid {
				t.Fatalf("validAPIURL(%q) = %t, want %t", test.value, actual, test.valid)
			}
		})
	}
}
