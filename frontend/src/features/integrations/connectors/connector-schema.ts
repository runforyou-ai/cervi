/** 连接器表单校验规则。 */
import { z } from "zod"

import { IntegrationConnectionType } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 判断连接器服务地址是否完整有效。 */
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

/** 创建连接器表单校验。 */
export function createConnectorSchema(messages: {
  typeInvalid: string
  nameRequired: string
  nameTooLong: string
  descriptionTooLong: string
  apiURLRequired: string
  apiURLInvalid: string
  apiKeyRequired: string
  apiKeyTooLong: string
}) {
  return z.object({
    type: requiredWailsEnum(IntegrationConnectionType, messages.typeInvalid),
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(100, messages.nameTooLong),
    description: z.string().trim().max(2000, messages.descriptionTooLong),
    apiURL: z
      .string()
      .trim()
      .min(1, messages.apiURLRequired)
      .refine(validAPIURL, messages.apiURLInvalid),
    apiKey: z
      .string()
      .trim()
      .min(1, messages.apiKeyRequired)
      .max(2048, messages.apiKeyTooLong),
  })
}

/** 连接器表单值。 */
export type ConnectorFormValues = z.infer<
  ReturnType<typeof createConnectorSchema>
>
