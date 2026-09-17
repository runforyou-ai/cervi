/** 模型服务供应商品牌的界面配置。 */
import { AIProviderBrand, type AIProviderBrandId } from "@/api"

type AIProviderBrandConfig = {
  nameKey: `modelServices.brands.${AIProviderBrandId}`
  defaultAPIURL: string
  /** 模型目录由服务实例提供，添加模型时读取服务实例。 */
  discoversModels?: boolean
  /** 自建或本机部署的服务可以不配置凭据。 */
  supportsNoCredential?: boolean
}

export const aiProviderBrandOrder: AIProviderBrandId[] = [
  AIProviderBrand.AIProviderBrandDeepSeek,
  AIProviderBrand.AIProviderBrandAlibaba,
  AIProviderBrand.AIProviderBrandOpenAI,
  AIProviderBrand.AIProviderBrandAnthropic,
  AIProviderBrand.AIProviderBrandGoogle,
  AIProviderBrand.AIProviderBrandMoonshot,
  AIProviderBrand.AIProviderBrandZhipu,
  AIProviderBrand.AIProviderBrandVolcengine,
  AIProviderBrand.AIProviderBrandMiniMax,
  AIProviderBrand.AIProviderBrandXAI,
  AIProviderBrand.AIProviderBrandMistral,
  AIProviderBrand.AIProviderBrandOllama,
  AIProviderBrand.AIProviderBrandOpenAICompatible,
]

export const aiProviderBrandConfigs: Record<
  AIProviderBrandId,
  AIProviderBrandConfig
> = {
  [AIProviderBrand.AIProviderBrandDeepSeek]: {
    nameKey: "modelServices.brands.deepseek",
    defaultAPIURL: "https://api.deepseek.com",
  },
  [AIProviderBrand.AIProviderBrandAlibaba]: {
    nameKey: "modelServices.brands.alibaba",
    defaultAPIURL: "https://dashscope.aliyuncs.com",
  },
  [AIProviderBrand.AIProviderBrandOpenAI]: {
    nameKey: "modelServices.brands.openai",
    defaultAPIURL: "https://api.openai.com/v1",
  },
  [AIProviderBrand.AIProviderBrandAnthropic]: {
    nameKey: "modelServices.brands.anthropic",
    defaultAPIURL: "https://api.anthropic.com",
  },
  [AIProviderBrand.AIProviderBrandGoogle]: {
    nameKey: "modelServices.brands.google",
    defaultAPIURL: "https://generativelanguage.googleapis.com",
  },
  [AIProviderBrand.AIProviderBrandMoonshot]: {
    nameKey: "modelServices.brands.moonshot",
    defaultAPIURL: "https://api.moonshot.cn/v1",
  },
  [AIProviderBrand.AIProviderBrandZhipu]: {
    nameKey: "modelServices.brands.zhipu",
    defaultAPIURL: "https://open.bigmodel.cn/api/paas/v4",
  },
  [AIProviderBrand.AIProviderBrandVolcengine]: {
    nameKey: "modelServices.brands.volcengine",
    defaultAPIURL: "https://ark.cn-beijing.volces.com/api/v3",
  },
  [AIProviderBrand.AIProviderBrandMiniMax]: {
    nameKey: "modelServices.brands.minimax",
    defaultAPIURL: "https://api.minimaxi.com/v1",
  },
  [AIProviderBrand.AIProviderBrandXAI]: {
    nameKey: "modelServices.brands.xai",
    defaultAPIURL: "https://api.x.ai/v1",
  },
  [AIProviderBrand.AIProviderBrandMistral]: {
    nameKey: "modelServices.brands.mistral",
    defaultAPIURL: "https://api.mistral.ai/v1",
  },
  [AIProviderBrand.AIProviderBrandOllama]: {
    nameKey: "modelServices.brands.ollama",
    defaultAPIURL: "http://localhost:11434",
    discoversModels: true,
    supportsNoCredential: true,
  },
  [AIProviderBrand.AIProviderBrandOpenAICompatible]: {
    nameKey: "modelServices.brands.openai_compatible",
    defaultAPIURL: "",
    discoversModels: true,
    supportsNoCredential: true,
  },
}
