/** 模型服务供应商调用与归一化。 */
import {
  CreateAIProvider,
  DeleteAIProvider,
  GetAIProvider,
  ListAIProviders,
  ListAvailableAIModels,
  TestAIProviderConnection,
  UpdateAIProvider,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  AIModelInputModality,
  AIModelType,
  AIProviderBrand,
  type AIProvider,
  type AIProviderInput,
  type AIProviderList,
  type AIProviderModel,
  type AIProviderModelList,
  type AIProviderModelSummary,
  type AIProviderSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type AIProviderBrandId = Exclude<AIProviderBrand, AIProviderBrand.$zero>

export type AIModelTypeId = Exclude<AIModelType, AIModelType.$zero>

export type AIModelInputModalityId = Exclude<
  AIModelInputModality,
  AIModelInputModality.$zero
>

export type AIProviderModelData = Omit<
  AIProviderModel,
  "type" | "inputModalities"
> & {
  type: AIModelTypeId
  inputModalities: AIModelInputModalityId[]
}

export type AIProviderData = Omit<AIProvider, "brand" | "models"> & {
  brand: AIProviderBrandId
  models: AIProviderModelData[]
}

export type AIProviderModelSummaryData = Omit<AIProviderModelSummary, "type"> & {
  type: AIModelTypeId
}

export type AIProviderSummaryData = Omit<
  AIProviderSummary,
  "brand" | "models"
> & {
  brand: AIProviderBrandId
  models: AIProviderModelSummaryData[]
}

export type AIProviderListData = Omit<AIProviderList, "providers"> & {
  providers: AIProviderSummaryData[]
}

const listAIProvidersBound = bind(ListAIProviders)
const getAIProviderBound = bind(GetAIProvider)
const listAvailableAIModelsBound = bind(ListAvailableAIModels)
const testAIProviderConnectionBound = bind(TestAIProviderConnection)
const createAIProviderBound = bind(CreateAIProvider)
const updateAIProviderBound = bind(UpdateAIProvider)

/** 读取当前企业的模型服务供应商列表。 */
export function listAIProviders() {
  return listAIProvidersBound().then(
    (output): AIProviderListData => ({
      ...output,
      providers: asList(output.providers).map(normalizeAIProviderSummary),
    }),
  )
}

/** 读取模型服务供应商详情。 */
export function getAIProvider(providerId: string) {
  return getAIProviderBound(providerId).then(normalizeAIProvider)
}

/** 读取指定品牌的预设模型目录。 */
export function listAvailableAIModels(brand: AIProviderBrand) {
  return listAvailableAIModelsBound(brand).then((output: AIProviderModelList) =>
    asList(output.models).map(normalizeAIProviderModel),
  )
}

/** 测试模型服务供应商草稿配置。 */
export const testAIProviderConnection = testAIProviderConnectionBound

/** 创建模型服务供应商。 */
export function createAIProvider(input: AIProviderInput) {
  return createAIProviderBound(input).then(normalizeAIProvider)
}

/** 修改模型服务供应商。 */
export function updateAIProvider(providerId: string, input: AIProviderInput) {
  return updateAIProviderBound(providerId, input).then(normalizeAIProvider)
}

/** 删除模型服务供应商。 */
export const deleteAIProvider = bind(DeleteAIProvider)

/** 归一化模型服务供应商详情。 */
function normalizeAIProvider(provider: AIProvider): AIProviderData {
  return {
    ...provider,
    brand: provider.brand as AIProviderBrandId,
    models: asList(provider.models).map(normalizeAIProviderModel),
  }
}

/** 归一化模型服务供应商列表项。 */
function normalizeAIProviderSummary(
  provider: AIProviderSummary,
): AIProviderSummaryData {
  return {
    ...provider,
    brand: provider.brand as AIProviderBrandId,
    models: asList(provider.models).map(normalizeAIProviderModelSummary),
  }
}

/** 归一化供应商列表中的模型目录摘要。 */
function normalizeAIProviderModelSummary(
  model: AIProviderModelSummary,
): AIProviderModelSummaryData {
  return { ...model, type: model.type as AIModelTypeId }
}

/** 归一化模型目录项。 */
function normalizeAIProviderModel(model: AIProviderModel): AIProviderModelData {
  return {
    ...model,
    type: model.type as AIModelTypeId,
    inputModalities: asList(model.inputModalities) as AIModelInputModalityId[],
  }
}
