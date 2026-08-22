/** AI 供应商表单校验规则。 */
import { z } from "zod"

import { AIProviderBrand } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 判断 AI API 地址是否完整有效。 */
function validAPIURL(value: string) {
  try {
    const endpoint = new URL(value)
    return (
      (endpoint.protocol === "http:" || endpoint.protocol === "https:") &&
      endpoint.username === "" &&
      endpoint.password === "" &&
      endpoint.search === "" &&
      endpoint.hash === ""
    )
  } catch {
    return false
  }
}

/** 创建 AI 供应商表单校验。 */
export function createAIProviderSchema(messages: {
  brandInvalid: string
  nameRequired: string
  nameTooLong: string
  apiKeyRequired: string
  apiKeyTooLong: string
  apiUrlRequired: string
  apiUrlInvalid: string
  modelIdentifierRequired: string
  modelIdentifierTooLong: string
  modelNameRequired: string
  modelNameTooLong: string
  contextWindowInvalid: string
  maxOutputTokensInvalid: string
  modelIdentifierDuplicate: string
}) {
  return z.object({
    brand: requiredWailsEnum(AIProviderBrand, messages.brandInvalid),
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(100, messages.nameTooLong),
    apiKey: z
      .string()
      .trim()
      .min(1, messages.apiKeyRequired)
      .max(2048, messages.apiKeyTooLong),
    apiUrl: z
      .string()
      .trim()
      .min(1, messages.apiUrlRequired)
      .refine(validAPIURL, messages.apiUrlInvalid),
    models: z
      .array(
        z.object({
          identifier: z
            .string()
            .trim()
            .min(1, messages.modelIdentifierRequired)
            .max(200, messages.modelIdentifierTooLong),
          name: z
            .string()
            .trim()
            .min(1, messages.modelNameRequired)
            .max(200, messages.modelNameTooLong),
          contextWindow: z
            .string()
            .trim()
            .refine(
              (value) => parseTokenCount(value) !== null,
              messages.contextWindowInvalid,
            ),
          maxOutputTokens: z
            .string()
            .trim()
            .refine(
              (value) => parseTokenCount(value) !== null,
              messages.maxOutputTokensInvalid,
            ),
        }),
      )
      .superRefine((models, context) => {
        const identifiers = new Set<string>()
        models.forEach((model, index) => {
          if (identifiers.has(model.identifier)) {
            context.addIssue({
              code: z.ZodIssueCode.custom,
              message: messages.modelIdentifierDuplicate,
              path: [index, "identifier"],
            })
          }
          identifiers.add(model.identifier)
        })
      }),
  })
}

/** 把正整数或 K、M 紧凑值转换为 Token 数。 */
export function parseTokenCount(value: string) {
  const matched = value
    .trim()
    .toUpperCase()
    .match(/^(\d+(?:\.\d+)?)([KM]?)$/)
  if (!matched) return null
  const amount = Number(matched[1])
  const multiplier =
    matched[2] === "M" ? 1_048_576 : matched[2] === "K" ? 1024 : 1
  const tokens = amount * multiplier
  return Number.isSafeInteger(tokens) && tokens > 0 ? tokens : null
}

export type AIProviderFormValues = z.infer<
  ReturnType<typeof createAIProviderSchema>
>
