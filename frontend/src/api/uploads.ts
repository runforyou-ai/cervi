/** 执行服务端签发的文件内容上传请求。 */
import type { FileUploadRequest } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

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
