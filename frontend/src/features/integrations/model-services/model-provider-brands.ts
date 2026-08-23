/** 模型服务供应商品牌的界面配置。 */
import { AIProviderBrand, type AIProviderBrandId } from "@/api"

type AIProviderBrandConfig = {
  nameKey: `modelServices.brands.${"deepseek" | "alibaba" | "openai"}`
  defaultAPIURL: string
}

export const aiProviderBrandOrder: AIProviderBrandId[] = [
  AIProviderBrand.AIProviderBrandDeepSeek,
  AIProviderBrand.AIProviderBrandAlibaba,
  AIProviderBrand.AIProviderBrandOpenAI,
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
}
