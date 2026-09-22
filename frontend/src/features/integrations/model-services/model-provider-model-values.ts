/** 模型目录条目与表单值之间的转换。 */
import type { AIProviderModelData } from "@/api"

/** 把模型 Token 数转换为紧凑显示值。 */
function formatTokenCount(value: number) {
  if (value % 1_048_576 === 0) return `${value / 1_048_576}M`
  if (value % 1024 === 0) return `${value / 1024}K`
  return String(value)
}

/** 把模型契约转换为表单值。 */
export function modelFormValue(model: AIProviderModelData) {
  return {
    identifier: model.identifier,
    name: model.name,
    type: model.type,
    inputModalities: model.inputModalities,
    contextWindow:
      model.contextWindow > 0 ? formatTokenCount(model.contextWindow) : "",
    maxOutputTokens:
      model.maxOutputTokens > 0 ? formatTokenCount(model.maxOutputTokens) : "",
  }
}
