/** 管理当前页面的单文件上传和失败分片重传。 */
import { useEffect, useRef, useState } from "react"
import {
  FilePurpose,
  cancelFileUpload,
  completeFileUpload,
  createFileUpload,
  FileTransfer,
  type File as UploadedFile,
} from "@/api"

type UploadSession = {
  transfer: FileTransfer
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
  const [status, setStatus] = useState<"uploading" | "uploaded" | "failed">(
    "uploading",
  )

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
      const session = sessionRef.current
      session?.controller.abort()
      // 已经发出的消息命令等待其完成，由发送结果决定是否清理临时文件。
      if (session?.transfer.upload && !session.sending) {
        void cancelFileUpload(session.transfer.upload.file.id).catch(() => {})
      }
    }
  }, [])

  /** 从第一个尚未成功的分片继续上传。 */
  async function run(session: UploadSession, fileSize: number) {
    setStatus("uploading")
    try {
      const result = await session.transfer.run(
        session.controller.signal,
        (bytes) => {
          if (aliveRef.current && sessionRef.current === session)
            setProgress(fileSize ? bytes / fileSize : 1)
        },
        completeFileUpload,
      )
      if (!aliveRef.current || sessionRef.current !== session) return
      setUploaded(result)
      setProgress(1)
      setStatus("uploaded")
    } catch (error) {
      if (
        session.controller.signal.aborted &&
        session.transfer.upload &&
        !session.sending
      ) {
        await cancelFileUpload(session.transfer.upload.file.id).catch(() => {})
      }
      if (
        !aliveRef.current ||
        sessionRef.current !== session ||
        session.controller.signal.aborted
      )
        return
      setProgress(fileSize ? session.transfer.bytes / fileSize : 0)
      setStatus("failed")
      throw error
    }
  }

  /** 开始上传用户刚选择的文件。 */
  function select(selected: File) {
    const session: UploadSession = {
      transfer: new FileTransfer(selected, () =>
        createFileUpload({
          purpose: FilePurpose.FilePurposeMessageAttachment,
          fileName: selected.name,
          contentType: selected.type,
          byteSize: selected.size,
        }),
      ),
      controller: new AbortController(),
      sending: false,
    }
    sessionRef.current = session
    setFile(selected)
    setUploaded(null)
    setProgress(0)
    return run(session, selected.size)
  }

  /** 重新上传当前未完成的分片或确认上传结果。 */
  function retry() {
    const session = sessionRef.current
    if (session && file) return run(session, file.size)
    return Promise.resolve()
  }

  /** 移除当前选择并取消未发送的上传。 */
  function clear(sent = false) {
    const session = sessionRef.current
    sessionRef.current = null
    session?.controller.abort()
    if (!sent && session?.transfer.upload)
      void cancelFileUpload(session.transfer.upload.file.id).catch(() => {})
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
