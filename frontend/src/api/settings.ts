import { request } from "@/api/client"

export const storageProviderIds = [
  "generic",
  "aws",
  "r2",
  "aliyun",
  "tencent",
  "baidu",
  "qiniu",
  "huawei",
  "ucloud",
  "minio",
  "rustfs",
] as const

export type StorageProviderId = (typeof storageProviderIds)[number]

export type S3Setting = {
  enabled: boolean
  provider: StorageProviderId
  endpoint: string
  region: string
  bucket: string
  accessKeyId: string
  secretAccessKey: string
  forcePathStyle: boolean
}

export function getS3Setting() {
  return request<S3Setting>("/settings/storage/s3")
}

export function saveS3Setting(input: S3Setting) {
  return request<S3Setting>("/settings/storage/s3", {
    method: "PUT",
    body: JSON.stringify(input),
  })
}

export function testS3Setting(input: S3Setting) {
  return request<void>("/settings/storage/s3/test", {
    method: "POST",
    body: JSON.stringify(input),
  })
}
