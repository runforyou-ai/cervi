/** 对象存储设置表单校验规则。 */
import { isHTTPEndpoint } from "@/lib/http-url"
import { z } from "zod"

import { StorageProvider } from "@/api"
import { requiredWailsEnum } from "@/lib/wails-enum"

/** 创建对象存储设置校验。 */
export function createStorageSettingsSchema(messages: {
  providerInvalid: string
  endpointRequired: string
  endpointInvalid: string
  publicBaseUrlRequired: string
  publicBaseUrlInvalid: string
  regionRequired: string
  bucketRequired: string
  accessKeyIdRequired: string
  secretAccessKeyRequired: string
}) {
  return z.object({
    enabled: z.boolean(),
    provider: requiredWailsEnum(
      StorageProvider,
      messages.providerInvalid,
    ),
    endpoint: z
      .string()
      .trim()
      .min(1, messages.endpointRequired)
      .refine(isHTTPEndpoint, messages.endpointInvalid),
    publicBaseUrl: z
      .string()
      .trim()
      .min(1, messages.publicBaseUrlRequired)
      .refine(isHTTPEndpoint, messages.publicBaseUrlInvalid),
    region: z.string().trim().min(1, messages.regionRequired),
    bucket: z.string().trim().min(1, messages.bucketRequired),
    accessKeyId: z.string().trim().min(1, messages.accessKeyIdRequired),
    secretAccessKey: z
      .string()
      .trim()
      .min(1, messages.secretAccessKeyRequired),
    forcePathStyle: z.boolean(),
  })
}

export type StorageSettingsFormValues = z.infer<
  ReturnType<typeof createStorageSettingsSchema>
>
