/** 模型服务供应商品牌的界面元数据。 */
import { AIProviderBrand, type AIProviderBrandId } from "@/api"

export type AIProviderBrandConfig = {
  id: AIProviderBrandId
  nameKey: `modelServices.brands.${"deepseek" | "alibaba" | "openai"}`
  defaultAPIURL: string
}

export const aiProviderBrands: AIProviderBrandConfig[] = [
  {
    id: AIProviderBrand.AIProviderBrandDeepSeek,
    nameKey: "modelServices.brands.deepseek",
    defaultAPIURL: "https://api.deepseek.com",
  },
  {
    id: AIProviderBrand.AIProviderBrandAlibaba,
    nameKey: "modelServices.brands.alibaba",
    defaultAPIURL: "https://dashscope.aliyuncs.com",
  },
  {
    id: AIProviderBrand.AIProviderBrandOpenAI,
    nameKey: "modelServices.brands.openai",
    defaultAPIURL: "https://api.openai.com/v1",
  },
]

/** 返回指定 AI 供应商品牌的界面配置。 */
export function getAIProviderBrand(id: AIProviderBrand) {
  const brand = aiProviderBrands.find((item) => item.id === id)
  if (!brand) throw new Error(`Unsupported AI provider brand: ${id}`)
  return brand
}
