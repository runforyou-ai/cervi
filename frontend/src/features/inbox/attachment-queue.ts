/** 保存当前工作台内的附件队列，聊天切换不影响正在上传的文件。 */
import {
  AttachmentUploadStatus,
  FilePurpose,
  cancelFileUpload,
  completeFileMultipartUpload,
  completeFileUpload,
  createFilePartUpload,
  createFileUpload,
  sendAttachmentBatch,
  updateAttachmentUploads,
  uploadFileSlice,
  type FileUpload,
  type InboxConversation,
} from "@/api"
import type { OutgoingConversationMessage } from "./use-outgoing-conversation-messages"

export type SelectedAttachment = {
  id: string
  file: File
  previewURL: string
  imageWidth: number
  imageHeight: number
}

export type AttachmentJob = {
  id: string
  batchID: string
  conversationID: string
  targetIdentityID: string
  selected: SelectedAttachment | null
  previewURL: string
  upload: FileUpload | null
  message: OutgoingConversationMessage
  stage: "preparing" | "queued" | "uploading" | "ready" | "failed" | "cancelled"
  bytes: number
  nextPart: number
  contentUploaded: boolean
  controller: AbortController
}

type Batch = {
  id: string
  conversationID: string
  targetIdentityID: string
  body: string
  jobs: AttachmentJob[]
  saved: boolean
  preparing: boolean
  onCreated: (conversation: InboxConversation | null) => void
}

/** 管理有序消息的本地展示、并发上传和当前页面重试。 */
export class AttachmentQueue {
  private jobs: AttachmentJob[] = []
  private batches = new Map<string, Batch>()
  private listeners = new Set<() => void>()
  private active = 0
  private preparingBatch = false
  private disposed = false
  private heartbeat: ReturnType<typeof setInterval> | undefined
  private refresh: (conversationID: string) => void
  private reportError: (error: unknown) => void

  /** 保存查询刷新和发送错误提示的回调。 */
  constructor(
    refresh: (conversationID: string) => void,
    reportError: (error: unknown) => void,
  ) {
    this.refresh = refresh
    this.reportError = reportError
  }

  /** 订阅上传状态变化。 */
  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }
  /** 返回当前不可变队列快照。 */
  snapshot = () => this.jobs
  /** 发布新的队列快照。 */
  private emit() {
    this.jobs = [...this.jobs]
    for (const listener of this.listeners) listener()
  }

  /** 启动当前工作台的上传活跃确认。 */
  start() {
    this.disposed = false
    this.heartbeat = setInterval(() => {
      const ids = this.jobs
        .filter(
          (job) =>
            (job.stage === "queued" || job.stage === "uploading") && job.upload,
        )
        .map((job) => job.upload!.file.id)
      for (let index = 0; index < ids.length; index += 100) {
        void updateAttachmentUploads({
          fileIds: ids.slice(index, index + 100),
          status: AttachmentUploadStatus.AttachmentUploading,
        }).catch(() => {})
      }
    }, 30000)
  }

  /** 立即展示所选文件和说明，再请求服务端按顺序保存消息。 */
  enqueue(
    files: SelectedAttachment[],
    body: string,
    conversationID: string,
    targetIdentityID: string,
    onCreated: Batch["onCreated"],
  ) {
    const batchID = crypto.randomUUID()
    const now = Date.now()
    const jobs: AttachmentJob[] = files.map((selected, index) => ({
      id: selected.id,
      batchID,
      conversationID,
      targetIdentityID,
      selected,
      previewURL: selected.previewURL,
      upload: null,
      stage: "preparing",
      bytes: 0,
      nextPart: 1,
      contentUploaded: false,
      controller: new AbortController(),
      message: {
        clientMessageID: selected.id,
        body: "",
        originatedAt: new Date(now + index).toISOString(),
        replyTo: null,
        mentionSubjectIDs: [],
        mentionAll: false,
        mentionAllToken: null,
        status: "sending",
        showSending: true,
        saved: null,
        attachment: {
          id: selected.id,
          name: selected.file.name,
          contentType: selected.file.type,
          byteSize: selected.file.size,
          contentUrl: "",
          uploadStatus: AttachmentUploadStatus.AttachmentUploading,
          imageWidth: selected.imageWidth,
          imageHeight: selected.imageHeight,
        },
      },
    }))
    if (body.trim())
      jobs.push({
        id: batchID,
        batchID,
        conversationID,
        targetIdentityID,
        selected: null,
        previewURL: "",
        upload: null,
        stage: "preparing",
        bytes: 0,
        nextPart: 1,
        contentUploaded: false,
        controller: new AbortController(),
        message: {
          clientMessageID: batchID,
          body: body.trim(),
          originatedAt: new Date(now + files.length).toISOString(),
          replyTo: null,
          mentionSubjectIDs: [],
          mentionAll: false,
          mentionAllToken: null,
          status: "sending",
          showSending: true,
          saved: null,
        },
      })
    const batch: Batch = {
      id: batchID,
      conversationID,
      targetIdentityID,
      body: body.trim(),
      jobs,
      saved: false,
      preparing: false,
      onCreated,
    }
    this.batches.set(batchID, batch)
    this.jobs = [...this.jobs, ...jobs]
    this.emit()
    void this.prepare(batch)
  }

  /** 为未保存的批次创建临时文件记录并幂等保存全部消息。 */
  private async prepare(batch: Batch) {
    if (batch.preparing || this.preparingBatch || this.disposed) return
    this.preparingBatch = true
    batch.preparing = true
    for (const job of batch.jobs)
      if (job.stage !== "cancelled") {
        job.stage = "preparing"
        job.message.status = "sending"
      }
    this.emit()
    try {
      const files = batch.jobs.filter((job) => job.message.attachment)
      for (const job of files) {
        const attachment = job.message.attachment!
        job.upload ??= await createFileUpload({
          purpose: FilePurpose.FilePurposeMessageAttachment,
          fileName: attachment.name,
          contentType: attachment.contentType,
          byteSize: attachment.byteSize,
        })
        if (this.disposed) {
          await this.abandon()
          return
        }
      }
      const result = await sendAttachmentBatch({
        batchId: batch.id,
        conversationId: batch.targetIdentityID ? "" : batch.conversationID,
        targetIdentityId: batch.targetIdentityID,
        body: batch.body,
        attachments: files.map((job) => ({
          fileId: job.upload!.file.id,
          clientMessageId: job.id,
          imageWidth: job.message.attachment!.imageWidth,
          imageHeight: job.message.attachment!.imageHeight,
        })),
      })
      batch.saved = true
      batch.conversationID = result.conversationId
      for (let index = 0; index < batch.jobs.length; index++) {
        const job = batch.jobs[index]
        job.conversationID = result.conversationId
        job.message.saved = result.messages[index]
        job.message.originatedAt = result.messages[index].originatedAt
        job.message.status = "sent"
        if (job.stage === "cancelled") {
          if (job.upload)
            await updateAttachmentUploads({
              fileIds: [job.upload.file.id],
              status: AttachmentUploadStatus.AttachmentCancelled,
            })
          continue
        }
        job.stage = job.message.attachment ? "queued" : "ready"
      }
      this.refresh(result.conversationId)
      if (this.disposed) {
        await this.abandon()
        return
      }
      batch.onCreated(result.conversation)
      this.emit()
      this.pump()
    } catch (error) {
      console.warn("附件消息批次保存失败", error)
      if (!this.disposed) this.reportError(error)
      for (const job of batch.jobs)
        if (job.stage !== "cancelled") {
          job.stage = "failed"
          job.message.status = "failed"
        }
      this.emit()
      if (this.disposed) await this.abandon()
    } finally {
      batch.preparing = false
      this.preparingBatch = false
      const next = [...this.batches.values()].find(
        (item) =>
          !item.saved && item.jobs.some((job) => job.stage === "preparing"),
      )
      if (next) void this.prepare(next)
    }
  }

  /** 同时处理最多三个文件，每个文件顺序上传分片。 */
  private pump() {
    if (this.disposed) return
    for (const job of this.jobs) {
      if (this.active >= 3) break
      if (job.stage !== "queued") continue
      job.stage = "uploading"
      this.active++
      void this.upload(job).finally(() => {
        this.active--
        this.pump()
      })
    }
    this.emit()
  }

  /** 从第一个未成功的分片开始上传，完成后激活原消息中的文件。 */
  private async upload(job: AttachmentJob) {
    const session = job.upload!
    const file = job.selected!.file
    try {
      await updateAttachmentUploads({
        fileIds: [session.file.id],
        status: AttachmentUploadStatus.AttachmentUploading,
      })
      job.controller.signal.throwIfAborted()
      if (session.partSize > 0) {
        while ((job.nextPart - 1) * session.partSize < file.size) {
          const start = (job.nextPart - 1) * session.partSize
          const end = Math.min(start + session.partSize, file.size)
          const request = await createFilePartUpload(session.file.id, {
            partNumber: job.nextPart,
          })
          job.controller.signal.throwIfAborted()
          await uploadFileSlice(
            request,
            file.slice(start, end),
            job.controller.signal,
            (bytes) => {
              job.bytes = start + bytes
              this.emit()
            },
          )
          job.bytes = end
          job.nextPart++
        }
      } else if (!job.contentUploaded) {
        await uploadFileSlice(
          session.request,
          file,
          job.controller.signal,
          (bytes) => {
            job.bytes = bytes
            this.emit()
          },
        )
      }
      job.controller.signal.throwIfAborted()
      job.contentUploaded = true
      if (session.partSize > 0)
        await completeFileMultipartUpload(session.file.id)
      else await completeFileUpload(session.file.id)
      job.controller.signal.throwIfAborted()
      await updateAttachmentUploads({
        fileIds: [session.file.id],
        status: AttachmentUploadStatus.AttachmentReady,
      })
      if (job.stage === "cancelled" || this.disposed) return
      job.stage = "ready"
      job.selected = null
      if (job.message.saved?.attachment)
        job.message.saved.attachment.uploadStatus =
          AttachmentUploadStatus.AttachmentReady
      this.refresh(job.conversationID)
    } catch (error) {
      if (job.stage === "cancelled" || this.disposed) return
      console.warn("附件上传失败", error)
      job.stage = "failed"
      job.bytes = Math.min((job.nextPart - 1) * session.partSize, file.size)
      await updateAttachmentUploads({
        fileIds: [session.file.id],
        status: AttachmentUploadStatus.AttachmentFailed,
      }).catch(() => {})
      this.refresh(job.conversationID)
    } finally {
      this.emit()
    }
  }

  /** 重试当前页面仍持有内容的文件或尚未入库的完整批次。 */
  retry(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (!job || job.stage !== "failed") return
    const batch = this.batches.get(job.batchID)!
    if (!batch.saved) {
      void this.prepare(batch)
      return
    }
    job.controller = new AbortController()
    job.stage = "queued"
    this.pump()
  }

  /** 取消未完成文件并从双方时间线撤去其消息。 */
  async cancel(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (!job || job.stage === "ready" || job.stage === "cancelled") return
    job.stage = "cancelled"
    job.controller.abort()
    this.emit()
    try {
      if (job.upload && this.batches.get(job.batchID)?.saved) {
        await updateAttachmentUploads({
          fileIds: [job.upload.file.id],
          status: AttachmentUploadStatus.AttachmentCancelled,
        })
        this.refresh(job.conversationID)
      }
      if (job.previewURL) URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      job.selected = null
    } catch (error) {
      job.stage = "failed"
      throw error
    } finally {
      this.emit()
    }
  }

  /** 在远端图片可用后释放本地预览占用。 */
  releasePreview(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (job?.stage === "ready" && job.previewURL) {
      URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      this.emit()
    }
  }

  /** 结束本次页面持有的上传，不保留重启后续传信息。 */
  dispose() {
    this.disposed = true
    clearInterval(this.heartbeat)
    for (const job of this.jobs) {
      job.controller.abort()
      if (job.previewURL) URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      job.selected = null
    }
    void this.abandon()
  }

  /** 将已入库但尚未完成的附件标记为失败，异常退出由服务端活跃期限收敛。 */
  private async abandon() {
    // 未入库的临时文件直接交给清理；已入库文件保留失败消息。
    const unused = this.jobs.filter(
      (job) => job.upload && !this.batches.get(job.batchID)?.saved,
    )
    for (const job of unused) {
      await cancelFileUpload(job.upload!.file.id).catch((error) => {
        console.warn("清理未发送附件失败", error)
      })
      job.selected = null
    }
    const ids = this.jobs
      .filter(
        (job) =>
          job.upload &&
          job.stage !== "ready" &&
          job.stage !== "cancelled" &&
          this.batches.get(job.batchID)?.saved,
      )
      .map((job) => job.upload!.file.id)
    for (let index = 0; index < ids.length; index += 100)
      await updateAttachmentUploads({
        fileIds: ids.slice(index, index + 100),
        status: AttachmentUploadStatus.AttachmentFailed,
      }).catch(() => {})
  }
}
