/** 创建文件记录并将内容上传到最终存储位置。 */
import {
  CompleteFileUpload,
  CreateFilePartUpload,
  CompleteFileMultipartUpload,
  CancelFileUpload,
  CreateFileUpload,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  FilePurpose,
  FileUploadRequest,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"

/** 创建分片直传请求。 */
export const createFilePartUpload = bind(CreateFilePartUpload)

/** 合并分片并确认上传完成。 */
export const completeFileMultipartUpload = bind(CompleteFileMultipartUpload)

/** 取消未发送的临时文件。 */
export const cancelFileUpload = bind(CancelFileUpload)

/** 创建文件上传请求。 */
export const createFileUpload = bind(CreateFileUpload)

/** 核验并完成文件上传。 */
export const completeFileUpload = bind(CompleteFileUpload)

/** 创建、上传并确认一个临时文件。 */
export async function uploadFile(file: globalThis.File, purpose: FilePurpose) {
  const upload = await createFileUpload({
    purpose,
    fileName: file.name,
    contentType: file.type,
    byteSize: file.size,
  })
  await uploadFileContent(upload.request, file)
  return completeFileUpload(upload.file.id)
}

/** 将浏览器文件直接上传到请求指定的最终存储位置。 */
export async function uploadFileContent(
  request: FileUploadRequest,
  file: globalThis.File,
) {
  const headers = Object.fromEntries(
    Object.entries(request.headers ?? {}).filter(
      (entry): entry is [string, string] => entry[1] !== undefined,
    ),
  )
  const response = await fetch(request.url, {
    method: request.method,
    headers,
    body: file,
  })
  if (!response.ok) {
    throw new Error(`File upload failed with status ${response.status}`)
  }
}

/** 上传一个文件片段并报告字节进度。 */
export function uploadFileSlice(
  request: FileUploadRequest,
  content: Blob,
  signal: AbortSignal,
  onProgress: (bytes: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    xhr.open(request.method, request.url)
    for (const [name, value] of Object.entries(request.headers ?? {})) {
      if (value !== undefined) xhr.setRequestHeader(name, value)
    }
    xhr.upload.onprogress = (event) => onProgress(event.loaded)
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve()
      else reject(new Error(`File upload failed with status ${xhr.status}`))
    }
    xhr.onerror = () => reject(new Error("File upload network error"))
    xhr.onabort = () => reject(new DOMException("Upload cancelled", "AbortError"))
    xhr.onloadend = () => signal.removeEventListener("abort", abort)
    if (signal.aborted) {
      reject(new DOMException("Upload cancelled", "AbortError"))
      return
    }
    signal.addEventListener("abort", abort, { once: true })
    xhr.send(content)
  })
}
