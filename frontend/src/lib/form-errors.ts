/** 从结构化业务错误中取出字段或总览文案。 */
import type { ApiError } from "@/api"

/** 按字段顺序读取错误文案，没有字段错误时返回总览消息。 */
export function apiErrorMessage(
  error: ApiError,
  fieldNames: readonly string[] = [],
) {
  for (const name of fieldNames) {
    const message = error.fields[name]
    if (message) {
      return message
    }
  }

  return Object.values(error.fields).find(Boolean) ?? error.message
}
