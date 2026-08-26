package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// AIProviderBrand 表示模型服务供应商品牌。
type AIProviderBrand string

const (
	AIProviderBrandDeepSeek AIProviderBrand = AIProviderBrand(domain.AIProviderBrandDeepSeek)
	AIProviderBrandAlibaba  AIProviderBrand = AIProviderBrand(domain.AIProviderBrandAlibaba)
	AIProviderBrandOpenAI   AIProviderBrand = AIProviderBrand(domain.AIProviderBrandOpenAI)
)

// AIModelType 表示 AI 模型用途。
type AIModelType string

const (
	AIModelTypeChat      AIModelType = AIModelType(domain.AIModelTypeChat)
	AIModelTypeEmbedding AIModelType = AIModelType(domain.AIModelTypeEmbedding)
	AIModelTypeRerank    AIModelType = AIModelType(domain.AIModelTypeRerank)
)

// AIModelInputModality 表示模型支持的输入模态。
type AIModelInputModality string

const (
	AIModelInputModalityText  AIModelInputModality = AIModelInputModality(domain.AIModelInputModalityText)
	AIModelInputModalityImage AIModelInputModality = AIModelInputModality(domain.AIModelInputModalityImage)
	AIModelInputModalityAudio AIModelInputModality = AIModelInputModality(domain.AIModelInputModalityAudio)
	AIModelInputModalityVideo AIModelInputModality = AIModelInputModality(domain.AIModelInputModalityVideo)
)

// AIProviderInput 定义模型服务供应商可编辑字段。
type AIProviderInput struct {
	Brand  AIProviderBrand   `json:"brand"`
	Name   string            `json:"name"`
	APIKey string            `json:"apiKey"`
	APIURL string            `json:"apiUrl"`
	Models []AIProviderModel `json:"models"`
}

// AIProviderConnectionInput 定义测试模型服务连接需要的草稿配置。
type AIProviderConnectionInput struct {
	Brand  AIProviderBrand `json:"brand"`
	APIKey string          `json:"apiKey"`
	APIURL string          `json:"apiUrl"`
}

// AIProviderModel 定义模型服务供应商的模型目录项。
type AIProviderModel struct {
	Identifier      string                 `json:"identifier"`
	Name            string                 `json:"name"`
	Type            AIModelType            `json:"type"`
	InputModalities []AIModelInputModality `json:"inputModalities"`
	ContextWindow   int64                  `json:"contextWindow"`
	MaxOutputTokens int64                  `json:"maxOutputTokens"`
}

// AIProvider 定义企业模型服务供应商及其模型目录。
type AIProvider struct {
	ID     string            `json:"id"`
	Brand  AIProviderBrand   `json:"brand"`
	Name   string            `json:"name"`
	APIKey string            `json:"apiKey"`
	APIURL string            `json:"apiUrl"`
	Models []AIProviderModel `json:"models"`
}

// AIProviderModelSummary 定义供应商列表中的模型目录摘要。
type AIProviderModelSummary struct {
	Identifier string      `json:"identifier"`
	Name       string      `json:"name"`
	Type       AIModelType `json:"type"`
}

// AIProviderSummary 定义模型服务供应商列表项。
type AIProviderSummary struct {
	ID     string                   `json:"id"`
	Brand  AIProviderBrand          `json:"brand"`
	Name   string                   `json:"name"`
	APIURL string                   `json:"apiUrl"`
	Models []AIProviderModelSummary `json:"models"`
}

// AIProviderList 定义模型服务供应商列表。
type AIProviderList struct {
	Providers []AIProviderSummary `json:"providers"`
}

// AIProviderModelList 定义指定品牌的预设模型目录。
type AIProviderModelList struct {
	Models []AIProviderModel `json:"models"`
}
