package domain

// AIProviderBrand 定义模型服务供应商品牌。
type AIProviderBrand string

const (
	AIProviderBrandDeepSeek AIProviderBrand = "deepseek"
	AIProviderBrandAlibaba  AIProviderBrand = "alibaba"
	AIProviderBrandOpenAI   AIProviderBrand = "openai"
)

// AIModelType 定义 AI 模型用途。
type AIModelType string

const (
	AIModelTypeChat      AIModelType = "chat"
	AIModelTypeEmbedding AIModelType = "embedding"
	AIModelTypeRerank    AIModelType = "rerank"
)

// AIModelInputModality 定义模型支持的输入模态。
type AIModelInputModality string

const (
	AIModelInputModalityText  AIModelInputModality = "text"
	AIModelInputModalityImage AIModelInputModality = "image"
	AIModelInputModalityAudio AIModelInputModality = "audio"
	AIModelInputModalityVideo AIModelInputModality = "video"
)
