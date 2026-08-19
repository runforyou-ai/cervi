import {
  GetS3Setting,
  SaveS3Setting,
  TestS3Setting,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  StorageProvider,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/domain/models"
import type { S3Setting } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"

export { StorageProvider }
export type { S3Setting }
export type StorageProviderId = Exclude<
  StorageProvider,
  StorageProvider.$zero
>

export function getS3Setting() {
  return call((meta) => GetS3Setting(meta))
}

export function saveS3Setting(input: S3Setting) {
  return call((meta) => SaveS3Setting(meta, input))
}

export function testS3Setting(input: S3Setting) {
  return call((meta) => TestS3Setting(meta, input))
}
