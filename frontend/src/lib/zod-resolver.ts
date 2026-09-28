/** 带嵌套字段原生校验提示的 Zod 表单解析器。 */
import { zodResolver as baseZodResolver } from "@hookform/resolvers/zod"
import { get, type FieldErrors, type FieldValues, type Resolver } from "react-hook-form"
import type { z } from "zod"

type NativeValidityRef = { setCustomValidity: (message: string) => void; reportValidity: () => boolean }

/** 判断引用是否支持原生校验提示。 */
function hasNativeValidity(ref: unknown): ref is NativeValidityRef {
  return typeof ref === "object" && ref !== null && "reportValidity" in ref
}

/** 按字段路径遍历参与校验的字段，为带原生校验能力的控件写入对应错误文案并提示，无错误时清空。 */
function reportNativeValidity(fields: object, errors: FieldErrors) {
  for (const value of Object.values(fields)) {
    if (typeof value !== "object" || value === null) continue
    if ("name" in value && typeof value.name === "string" && "ref" in value) {
      const refs: unknown[] = "refs" in value && Array.isArray(value.refs) ? value.refs : [value.ref]
      const message = get(errors, value.name)?.message
      for (const ref of refs) {
        if (!hasNativeValidity(ref)) continue
        ref.setCustomValidity(typeof message === "string" ? message : "")
        ref.reportValidity()
      }
      continue
    }
    reportNativeValidity(value, errors)
  }
}

/** 按 Zod schema 校验表单，启用原生校验时同时为数组与嵌套对象中的字段写入浏览器提示。 */
export function zodResolver<Input extends FieldValues, Output, T extends z.ZodType<Output, Input>>(
  schema: T,
): Resolver<z.input<T>, unknown, z.output<T>> {
  const resolve = baseZodResolver<Input, unknown, Output, T>(schema)
  return async (values, context, options) => {
    const result = await resolve(values, context, { ...options, shouldUseNativeValidation: false })
    if (options.shouldUseNativeValidation) reportNativeValidity(options.fields, result.errors)
    return result
  }
}
