//go:build server

package aiprovider

import (
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeInputAcceptsCustomModels 验证自定义模型可保存并规范化文本字段。
func TestNormalizeInputAcceptsCustomModels(t *testing.T) {
	input, fields := normalizeInput(Input{
		Brand:  domain.AIProviderBrandDeepSeek,
		Name:   " 自定义供应商 ",
		APIKey: " secret ",
		APIURL: " https://api.deepseek.com ",
		Models: []Model{{
			Identifier: " custom-model ", Name: " 自定义模型 ", ContextWindow: 128_000, MaxOutputTokens: 8_000,
		}},
	})
	if len(fields) != 0 {
		t.Fatalf("normalizeInput() fields = %#v", fields)
	}
	if input.Name != "自定义供应商" || input.APIKey != "secret" || input.APIURL != "https://api.deepseek.com" {
		t.Fatalf("normalizeInput() = %#v", input)
	}
	if len(input.Models) != 1 || input.Models[0].Identifier != "custom-model" || input.Models[0].Name != "自定义模型" {
		t.Fatalf("normalizeInput() models = %#v", input.Models)
	}
}

// TestNormalizeInputRejectsInvalidModels 验证重复标识和无效 Token 数会被拒绝。
func TestNormalizeInputRejectsInvalidModels(t *testing.T) {
	_, fields := normalizeInput(Input{
		Brand: domain.AIProviderBrandDeepSeek, Name: "供应商", APIKey: "secret", APIURL: "https://api.deepseek.com",
		Models: []Model{
			{Identifier: "model", Name: "模型一", ContextWindow: 1, MaxOutputTokens: 1},
			{Identifier: "model", Name: "模型二", ContextWindow: 0, MaxOutputTokens: 1},
		},
	})
	if fields["models"] != ValidationModelsInvalid {
		t.Fatalf("normalizeInput() fields = %#v", fields)
	}
}

// TestAvailableModelsReturnsDeepSeekCatalog 验证 DeepSeek 预设模型目录。
func TestAvailableModelsReturnsDeepSeekCatalog(t *testing.T) {
	models := AvailableModels(domain.AIProviderBrandDeepSeek)
	if len(models) != 2 || models[0].Identifier != "deepseek-v4-flash" || models[1].Identifier != "deepseek-v4-pro" {
		t.Fatalf("AvailableModels() = %#v", models)
	}
}
