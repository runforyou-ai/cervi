/** 模型服务页签、类型和输入模态的界面配置。 */
import {
  AIModelInputModality,
  AIModelType,
  AIProviderBrand,
  type AIModelInputModalityId,
  type AIModelTypeId,
  type AIProviderBrandId,
} from "@/api"

export type ModelServiceSection = "chat" | "embedding" | "rerank"

export const modelServiceSectionOrder: ModelServiceSection[] = [
  "chat",
  "embedding",
  "rerank",
]

export const modelServiceSectionConfigs: Record<
  ModelServiceSection,
  {
    modelType: AIModelTypeId
    defaultBrand: AIProviderBrandId
    nameKey: `modelServices.tabs.${ModelServiceSection}`
  }
> = {
  chat: {
    modelType: AIModelType.AIModelTypeChat,
    defaultBrand: AIProviderBrand.AIProviderBrandDeepSeek,
    nameKey: "modelServices.tabs.chat",
  },
  embedding: {
    modelType: AIModelType.AIModelTypeEmbedding,
    defaultBrand: AIProviderBrand.AIProviderBrandAlibaba,
    nameKey: "modelServices.tabs.embedding",
  },
  rerank: {
    modelType: AIModelType.AIModelTypeRerank,
    defaultBrand: AIProviderBrand.AIProviderBrandAlibaba,
    nameKey: "modelServices.tabs.rerank",
  },
}

export const modelTypeNameKeys: Record<
  AIModelTypeId,
  `modelServices.models.types.${ModelServiceSection}`
> = {
  [AIModelType.AIModelTypeChat]: "modelServices.models.types.chat",
  [AIModelType.AIModelTypeEmbedding]: "modelServices.models.types.embedding",
  [AIModelType.AIModelTypeRerank]: "modelServices.models.types.rerank",
}

export const modelInputModalityOrder: AIModelInputModalityId[] = [
  AIModelInputModality.AIModelInputModalityText,
  AIModelInputModality.AIModelInputModalityImage,
  AIModelInputModality.AIModelInputModalityAudio,
  AIModelInputModality.AIModelInputModalityVideo,
]

export const modelInputModalityNameKeys: Record<
  AIModelInputModalityId,
  `modelServices.models.modalities.${"text" | "image" | "audio" | "video"}`
> = {
  [AIModelInputModality.AIModelInputModalityText]:
    "modelServices.models.modalities.text",
  [AIModelInputModality.AIModelInputModalityImage]:
    "modelServices.models.modalities.image",
  [AIModelInputModality.AIModelInputModalityAudio]:
    "modelServices.models.modalities.audio",
  [AIModelInputModality.AIModelInputModalityVideo]:
    "modelServices.models.modalities.video",
}
