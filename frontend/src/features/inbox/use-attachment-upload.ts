/** 管理当前页面的单文件上传和失败分片重传。 */
import { useEffect, useRef, useState } from "react"
import {
  FilePurpose,
  cancelFileUpload,
  completeFileMultipartUpload,
  completeFileUpload,
  createFilePartUpload,
  createFileUpload,
  uploadFileSlice,
  type File as UploadedFile,
  type FileUpload,
} from "@/api"

type UploadSession = {
  file: File
  upload: FileUpload | null
  contentUploaded: boolean
  completedBytes: number
  nextPart: number
  controller: AbortController
  sending: boolean
}

/** 保留当前选择及已成功的分片，取消后交由服务端清理。 */
export function useAttachmentUpload() {
  const sessionRef = useRef<UploadSession | null>(null)
  const aliveRef = useRef(true)
  const [file, setFile] = useState<File | null>(null)
  const [uploaded, setUploaded] = useState<UploadedFile | null>(null)
  const [progress, setProgress] = useState(0)
  const [status, setStatus] = useState<"uploading" | "uploaded" | "failed">("uploading")

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
      const session = sessionRef.current
      session?.controller.abort()
      // 已经发出的消息命令等待其完成，由发送结果决定是否清理临时文件。
      if (session?.upload && !session.sending) {
        void cancelFileUpload(session.upload.file.id).catch(() => {})
      }
    }
  }, [])

  /** 从第一个尚未成功的分片继续上传。 */
  async function run(session: UploadSession) {
    setStatus("uploading")
    try {
      session.upload ??= await createFileUpload({
        purpose: FilePurpose.FilePurposeMessageAttachment,
        fileName: session.file.name,
        contentType: session.file.type,
        byteSize: session.file.size,
      })
      if (session.controller.signal.aborted) {
        await cancelFileUpload(session.upload.file.id)
        return
      }
      const { partSize } = session.upload
      if (partSize > 0) {
        while (session.completedBytes < session.file.size) {
          const request = await createFilePartUpload(session.upload.file.id, { partNumber: session.nextPart })
          const end = Math.min(session.completedBytes + partSize, session.file.size)
          await uploadFileSlice(request, session.file.slice(session.completedBytes, end), session.controller.signal, (bytes) => {
            if (aliveRef.current && sessionRef.current === session) setProgress((session.completedBytes + bytes) / session.file.size)
          })
          session.completedBytes = end
          session.nextPart += 1
        }
      } else if (!session.contentUploaded) {
        await uploadFileSlice(session.upload.request, session.file, session.controller.signal, (bytes) => {
          if (aliveRef.current && sessionRef.current === session) setProgress(session.file.size ? bytes / session.file.size : 1)
        })
      }
      session.contentUploaded = true
      const result = partSize > 0
        ? await completeFileMultipartUpload(session.upload.file.id)
        : await completeFileUpload(session.upload.file.id)
      if (!aliveRef.current || sessionRef.current !== session) return
      setUploaded(result)
      setProgress(1)
      setStatus("uploaded")
    } catch (error) {
      if (!aliveRef.current || sessionRef.current !== session || session.controller.signal.aborted) return
      setProgress(session.file.size ? session.completedBytes / session.file.size : 0)
      setStatus("failed")
      throw error
    }
  }

  /** 开始上传用户刚选择的文件。 */
  function select(selected: File) {
    const session: UploadSession = { file: selected, upload: null, contentUploaded: false, completedBytes: 0, nextPart: 1, controller: new AbortController(), sending: false }
    sessionRef.current = session
    setFile(selected)
    setUploaded(null)
    setProgress(0)
    return run(session)
  }

  /** 重新上传当前未完成的分片或确认上传结果。 */
  function retry() {
    const session = sessionRef.current
    if (session) return run(session)
    return Promise.resolve()
  }

  /** 移除当前选择并取消未发送的上传。 */
  function clear(sent = false) {
    const session = sessionRef.current
    sessionRef.current = null
    session?.controller.abort()
    if (!sent && session?.upload) void cancelFileUpload(session.upload.file.id).catch(() => {})
    if (aliveRef.current) {
      setFile(null)
      setUploaded(null)
    }
  }

  /** 标记消息命令已发出，避免页面离开时取消同一个文件。 */
  function markSending(sending: boolean) {
    if (sessionRef.current) sessionRef.current.sending = sending
  }

  return { file, uploaded, progress, status, select, retry, clear, markSending }
}
