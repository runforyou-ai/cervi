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
			Identifier: " custom-model ", Name: " 自定义模型 ", Type: domain.AIModelTypeChat,
			InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText},
			ContextWindow:   128_000, MaxOutputTokens: 8_000,
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
			{Identifier: "model", Name: "模型一", Type: domain.AIModelTypeChat, InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText}, ContextWindow: 1, MaxOutputTokens: 1},
			{Identifier: "model", Name: "模型二", Type: domain.AIModelTypeChat, InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText}, ContextWindow: 0, MaxOutputTokens: 1},
		},
	})
	if fields["models"] != ValidationModelsInvalid {
		t.Fatalf("normalizeInput() fields = %#v", fields)
	}
}

// TestNormalizeInputRejectsEmptyModels 验证模型目录为空时不允许保存供应商。
func TestNormalizeInputRejectsEmptyModels(t *testing.T) {
	_, fields := normalizeInput(Input{
		Brand: domain.AIProviderBrandDeepSeek, Name: "供应商", APIKey: "secret", APIURL: "https://api.deepseek.com",
	})
	if fields["models"] != ValidationModelsInvalid {
		t.Fatalf("normalizeInput() fields = %#v", fields)
	}
}

// TestNormalizeInputAcceptsEmbeddingModalities 验证嵌入模型可声明输入模态且不保存输出 Token 数。
func TestNormalizeInputAcceptsEmbeddingModalities(t *testing.T) {
	input, fields := normalizeInput(Input{
		Brand: domain.AIProviderBrandOpenAI, Name: "OpenAI", APIKey: "secret", APIURL: "https://api.openai.com/v1",
		Models: []Model{{
			Identifier: "text-embedding-3-large", Name: "Text Embedding 3 Large", Type: domain.AIModelTypeEmbedding,
			InputModalities: []domain.AIModelInputModality{domain.AIModelInputModalityText},
			ContextWindow:   8_192, MaxOutputTokens: 1,
		}},
	})
	if len(fields) != 0 {
		t.Fatalf("normalizeInput() fields = %#v", fields)
	}
	if input.Models[0].MaxOutputTokens != 0 || len(input.Models[0].InputModalities) != 1 {
		t.Fatalf("normalizeInput() model = %#v", input.Models[0])
	}
}

// TestAvailableModelsReturnsOpenAICatalog 验证 OpenAI 预设只包含当前主流生成和向量模型。
func TestAvailableModelsReturnsOpenAICatalog(t *testing.T) {
	models := AvailableModels(domain.AIProviderBrandOpenAI)
	if len(models) != 5 || models[0].Identifier != "gpt-5.6-sol" || models[3].Identifier != "text-embedding-3-large" {
		t.Fatalf("AvailableModels() = %#v", models)
	}
	if models[3].Type != domain.AIModelTypeEmbedding || len(models[3].InputModalities) != 1 {
		t.Fatalf("AvailableModels() embedding = %#v", models[3])
	}
}

// TestAvailableModelsReturnsAlibabaCatalog 验证阿里云百炼预设覆盖三类模型。
func TestAvailableModelsReturnsAlibabaCatalog(t *testing.T) {
	models := AvailableModels(domain.AIProviderBrandAlibaba)
	if len(models) != 7 || models[0].Identifier != "qwen3.8-max" || models[3].Identifier != "qwen3.7-text-embedding" {
		t.Fatalf("AvailableModels() = %#v", models)
	}
	if models[5].Type != domain.AIModelTypeRerank || len(models[4].InputModalities) != 3 {
		t.Fatalf("AvailableModels() capabilities = %#v", models)
	}
}

// TestAvailableModelsReturnsDeepSeekCatalog 验证 DeepSeek 预设模型目录。
func TestAvailableModelsReturnsDeepSeekCatalog(t *testing.T) {
	models := AvailableModels(domain.AIProviderBrandDeepSeek)
	if len(models) != 2 || models[0].Identifier != "deepseek-v4-flash" || models[1].Identifier != "deepseek-v4-pro" {
		t.Fatalf("AvailableModels() = %#v", models)
	}
}
