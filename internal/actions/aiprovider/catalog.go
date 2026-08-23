//go:build server

package aiprovider

import "github.com/runforyou-ai/cervi/internal/domain"

// AvailableModels 返回指定品牌的预设模型目录。
func AvailableModels(brand domain.AIProviderBrand) []Model {
	text := []domain.AIModelInputModality{domain.AIModelInputModalityText}
	textAndImage := []domain.AIModelInputModality{domain.AIModelInputModalityText, domain.AIModelInputModalityImage}
	textImageVideo := []domain.AIModelInputModality{
		domain.AIModelInputModalityText,
		domain.AIModelInputModalityImage,
		domain.AIModelInputModalityVideo,
	}
	switch brand {
	case domain.AIProviderBrandDeepSeek:
		return []Model{
			{Identifier: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Type: domain.AIModelTypeChat, InputModalities: text, ContextWindow: 1_048_576, MaxOutputTokens: 393_216},
			{Identifier: "deepseek-v4-pro", Name: "DeepSeek V4 Pro", Type: domain.AIModelTypeChat, InputModalities: text, ContextWindow: 1_048_576, MaxOutputTokens: 393_216},
		}
	case domain.AIProviderBrandAlibaba:
		return []Model{
			{Identifier: "qwen3.8-max", Name: "Qwen 3.8 Max", Type: domain.AIModelTypeChat, InputModalities: textImageVideo, ContextWindow: 1_000_000, MaxOutputTokens: 131_072},
			{Identifier: "qwen3.7-plus", Name: "Qwen 3.7 Plus", Type: domain.AIModelTypeChat, InputModalities: textImageVideo, ContextWindow: 1_000_000, MaxOutputTokens: 131_072},
			{Identifier: "qwen3.7-flash", Name: "Qwen 3.7 Flash", Type: domain.AIModelTypeChat, InputModalities: textImageVideo, ContextWindow: 1_000_000, MaxOutputTokens: 131_072},
			{Identifier: "qwen3.7-text-embedding", Name: "Qwen 3.7 Text Embedding", Type: domain.AIModelTypeEmbedding, InputModalities: text, ContextWindow: 131_072},
			{Identifier: "qwen3-vl-embedding", Name: "Qwen 3 VL Embedding", Type: domain.AIModelTypeEmbedding, InputModalities: textImageVideo, ContextWindow: 32_000},
			{Identifier: "qwen3-rerank", Name: "Qwen 3 Rerank", Type: domain.AIModelTypeRerank, InputModalities: text, ContextWindow: 4_000},
			{Identifier: "qwen3-vl-rerank", Name: "Qwen 3 VL Rerank", Type: domain.AIModelTypeRerank, InputModalities: textImageVideo, ContextWindow: 8_000},
		}
	case domain.AIProviderBrandOpenAI:
		return []Model{
			{Identifier: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Type: domain.AIModelTypeChat, InputModalities: textAndImage, ContextWindow: 1_050_000, MaxOutputTokens: 128_000},
			{Identifier: "gpt-5.6-terra", Name: "GPT-5.6 Terra", Type: domain.AIModelTypeChat, InputModalities: textAndImage, ContextWindow: 1_050_000, MaxOutputTokens: 128_000},
			{Identifier: "gpt-5.6-luna", Name: "GPT-5.6 Luna", Type: domain.AIModelTypeChat, InputModalities: textAndImage, ContextWindow: 1_050_000, MaxOutputTokens: 128_000},
			{Identifier: "text-embedding-3-large", Name: "Text Embedding 3 Large", Type: domain.AIModelTypeEmbedding, InputModalities: text, ContextWindow: 8_192},
			{Identifier: "text-embedding-3-small", Name: "Text Embedding 3 Small", Type: domain.AIModelTypeEmbedding, InputModalities: text, ContextWindow: 8_192},
		}
	default:
		return nil
	}
}
