/** MCP 服务表单校验规则。 */
import { z } from "zod"

import { MCPServerType } from "@/api"

/** 判断地址是否为不含认证信息的完整 HTTP 或 HTTPS 地址。 */
function isMCPServerURL(value: string) {
  try {
    const parsed = new URL(value)
    return (
      (parsed.protocol === "http:" || parsed.protocol === "https:") &&
      parsed.host !== "" &&
      parsed.username === "" &&
      parsed.password === ""
    )
  } catch {
    return false
  }
}

/** 创建 MCP 服务表单校验。 */
export function createMCPServerSchema(messages: {
  nameRequired: string
  nameTooLong: string
  serverTypeInvalid: string
  urlRequired: string
  urlTooLong: string
  urlInvalid: string
}) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, messages.nameRequired)
      .max(100, messages.nameTooLong),
    url: z
      .string()
      .trim()
      .min(1, messages.urlRequired)
      .max(2048, messages.urlTooLong)
      .refine(isMCPServerURL, messages.urlInvalid),
    serverType: z.enum(MCPServerType, { error: messages.serverTypeInvalid }).refine(
      (value): boolean =>
        value === MCPServerType.MCPServerTypeSSE ||
        value === MCPServerType.MCPServerTypeStreamableHTTP,
      messages.serverTypeInvalid,
    ),
    authorizationToken: z.string(),
  })
}

export type MCPServerFormValues = z.infer<
  ReturnType<typeof createMCPServerSchema>
>
