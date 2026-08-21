//go:build server

package aiprovider

import "github.com/runforyou-ai/cervi/internal/domain"

// AvailableModels 返回指定品牌可选的模型目录。
func AvailableModels(brand domain.AIProviderBrand) []Model {
	switch brand {
	case domain.AIProviderBrandDeepSeek:
		return []Model{
			{Identifier: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: 1_048_576, MaxOutputTokens: 393_216},
			{Identifier: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", ContextWindow: 1_048_576, MaxOutputTokens: 393_216},
		}
	default:
		return nil
	}
}
