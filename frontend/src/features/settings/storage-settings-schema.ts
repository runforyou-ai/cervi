import { z } from "zod"

import { storageProviderIds } from "@/api/settings"

function validEndpoint(value: string) {
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

export function createStorageSettingsSchema(messages: {
  providerInvalid: string
  endpointRequired: string
  endpointInvalid: string
  regionRequired: string
  bucketRequired: string
  accessKeyIdRequired: string
  secretAccessKeyRequired: string
}) {
  return z.object({
    enabled: z.boolean(),
    provider: z.enum(storageProviderIds, {
      errorMap: () => ({ message: messages.providerInvalid }),
    }),
    endpoint: z
      .string()
      .trim()
      .min(1, messages.endpointRequired)
      .refine(validEndpoint, messages.endpointInvalid),
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
