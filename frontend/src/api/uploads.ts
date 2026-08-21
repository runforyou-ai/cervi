/** 创建文件记录并将内容上传到最终存储位置。 */
import type {
  FilePurpose,
  FileUploadRequest,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

import { completeFileUpload, createFileUpload } from "@/api/service"

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
  if (!Object.keys(headers).some((name) => name.toLowerCase() === "content-type")) {
    headers["Content-Type"] = file.type
  }
  const response = await fetch(request.url, {
    method: request.method,
    headers,
    body: file,
  })
  if (!response.ok) {
    throw new Error(`File upload failed with status ${response.status}`)
  }
}
