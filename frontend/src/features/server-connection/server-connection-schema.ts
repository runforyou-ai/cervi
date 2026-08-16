import { z } from "zod"

type ServerConnectionTranslator = (
  key: "serverUrlRequired" | "serverUrlInvalid",
) => string

function isValidServerUrl(value: string) {
  try {
    const url = new URL(value)
    return Boolean(url.hostname) && ["http:", "https:"].includes(url.protocol)
  } catch {
    return false
  }
}

export function createServerConnectionSchema(t: ServerConnectionTranslator) {
  return z.object({
    serverUrl: z
      .string()
      .trim()
      .min(1, t("serverUrlRequired"))
      .refine(isValidServerUrl, t("serverUrlInvalid")),
  })
}

export type ServerConnectionFormValues = z.infer<
  ReturnType<typeof createServerConnectionSchema>
>
