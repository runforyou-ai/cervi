/** 在工作台维护有序附件消息和持续上传任务。 */
import {
  AttachmentUploadStatus,
  FileTransfer,
  prepareFileUpload,
  completeAttachmentUpload,
  sendAttachmentBatch,
  updateAttachmentUploads,
  type InboxConversation,
  type MessageAttachment,
} from "@/api"
import type { OutgoingMessageStore } from "./outgoing-message-store"

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
  body: string
  attachment: MessageAttachment
  fileID: string
  messageID: string
  stage: "preparing" | "queued" | "uploading" | "ready" | "failed" | "cancelled"
  selected: SelectedAttachment | null
  previewURL: string
  transfer: FileTransfer | null
  bytes: number
  controller: AbortController
  cancelRequested: boolean
}

type Batch = {
  id: string
  conversationID: string
  targetIdentityID: string
  jobs: AttachmentJob[]
  saved: boolean
  preparing: boolean
  onCreated: (conversation: InboxConversation | null) => void
}

/** 调度消息入库和文件上传，文件传输由共享执行器处理。 */
export class AttachmentQueue {
  private jobs: AttachmentJob[] = []
  private batches = new Map<string, Batch>()
  private listeners = new Set<() => void>()
  private active = 0
  private preparingBatch = false
  private disposed = false
  private heartbeat: ReturnType<typeof setInterval> | undefined
  private outgoing: OutgoingMessageStore
  private refresh: (conversationID: string) => void
  private reportError: (error: unknown) => void

  /** 保存发送状态存储、查询刷新回调和发送错误回调。 */
  constructor(
    outgoing: OutgoingMessageStore,
    refresh: (conversationID: string) => void,
    reportError: (error: unknown) => void,
  ) {
    this.outgoing = outgoing
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
  /** 返回当前队列快照。 */
  snapshot = () => this.jobs
  /** 发布新的队列快照。 */
  private emit() {
    this.jobs = [...this.jobs]
    for (const listener of this.listeners) listener()
  }

  /** 定期确认排队或上传中的附件仍由当前页面持有。 */
  start() {
    this.disposed = false
    this.heartbeat = setInterval(() => {
      const ids = this.jobs
        .filter((job) => job.stage === "queued" || job.stage === "uploading")
        .map((job) => job.fileID)
      for (let index = 0; index < ids.length; index += 100) {
        void updateAttachmentUploads({
          fileIds: ids.slice(index, index + 100),
          status: AttachmentUploadStatus.AttachmentUploading,
        }).catch(() => {})
      }
    }, 30000)
  }

  /** 立即展示附件消息，以固定消息编号提交一次有序发送。 */
  enqueue(
    files: (SelectedAttachment & { body: string })[],
    conversationID: string,
    targetIdentityID: string,
    onCreated: Batch["onCreated"],
  ) {
    const batchID = crypto.randomUUID()
    const now = Date.now()
    // 草稿尚无会话编号，发送项按对端身份分组。
    const scopeID =
      conversationID || (targetIdentityID ? `draft:${targetIdentityID}` : "")
    const jobs: AttachmentJob[] = files.map((selected, index) => {
      const attachment: MessageAttachment = {
        id: selected.id,
        name: selected.file.name,
        contentType: selected.file.type,
        byteSize: selected.file.size,
        contentUrl: "",
        uploadStatus: AttachmentUploadStatus.AttachmentUploading,
        imageWidth: selected.imageWidth,
        imageHeight: selected.imageHeight,
      }
      this.outgoing.start(scopeID, {
        clientMessageID: selected.id,
        attachment,
        body: selected.body.trim(),
        originatedAt: new Date(now + index).toISOString(),
        replyTo: null,
        mentionSubjectIDs: [],
        mentionAll: false,
        mentionAllToken: null,
      })
      return {
        id: selected.id,
        batchID,
        conversationID,
        targetIdentityID,
        body: selected.body.trim(),
        attachment,
        fileID: "",
        messageID: "",
        selected,
        previewURL: selected.previewURL,
        transfer: null,
        bytes: 0,
        controller: new AbortController(),
        cancelRequested: false,
        stage: "preparing",
      }
    })
    const batch: Batch = {
      id: batchID,
      conversationID,
      targetIdentityID,
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

  /** 先原子保存文件记录和消息，收到结果后才开始传输文件内容。 */
  private async prepare(batch: Batch) {
    if (batch.preparing || this.preparingBatch || this.disposed) return
    this.preparingBatch = true
    batch.preparing = true
    for (const item of batch.jobs) {
      if (item.stage !== "cancelled") item.stage = "preparing"
    }
    this.emit()
    try {
      const result = await sendAttachmentBatch({
        conversationId: batch.targetIdentityID ? "" : batch.conversationID,
        targetIdentityId: batch.targetIdentityID,
        attachments: batch.jobs.map((job) => ({
          clientMessageId: job.id,
          body: job.body,
          fileName: job.attachment.name,
          contentType: job.attachment.contentType,
          byteSize: job.attachment.byteSize,
          imageWidth: job.attachment.imageWidth,
          imageHeight: job.attachment.imageHeight,
        })),
      })
      if (!this.batches.has(batch.id)) return
      batch.saved = true
      batch.conversationID = result.conversationId
      for (let index = 0; index < batch.jobs.length; index++) {
        const item = batch.jobs[index]
        const saved = result.messages[index]
        item.conversationID = result.conversationId
        item.fileID = saved.attachment!.id
        item.messageID = saved.id
        item.attachment = saved.attachment!
        if (this.disposed) this.outgoing.fail(item.id)
        else this.outgoing.succeed(item.id, saved)
      }
      for (const job of batch.jobs) {
        const status = job.attachment.uploadStatus
        if (
          job.cancelRequested ||
          status === AttachmentUploadStatus.AttachmentCancelled
        ) {
          job.cancelRequested = true
          job.stage = "cancelled"
          void this.cancel(job.id).catch(this.reportError)
        } else if (status === AttachmentUploadStatus.AttachmentReady) {
          job.stage = "ready"
          job.selected = null
          job.transfer = null
        } else job.stage = "queued"
      }
      this.refresh(result.conversationId)
      if (this.disposed) {
        await this.abandon()
        return
      }
      batch.onCreated(result.conversation)
      this.pump()
    } catch (error) {
      if (!this.batches.has(batch.id)) return
      console.warn("附件消息保存失败", error)
      if (!this.disposed) this.reportError(error)
      for (const item of batch.jobs) {
        if (item.stage !== "cancelled") {
          item.stage = "failed"
          this.outgoing.fail(item.id)
        }
      }
    } finally {
      batch.preparing = false
      this.preparingBatch = false
      this.emit()
      const next = [...this.batches.values()].find(
        (item) =>
          !item.saved && item.jobs.some((job) => job.stage === "preparing"),
      )
      if (next) void this.prepare(next)
    }
  }

  /** 同时上传最多三个文件。 */
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

  /** 上传文件后由服务端完成原附件消息，失败时保留执行器供手动重试。 */
  private async upload(job: AttachmentJob) {
    const fileID = job.fileID
    try {
      await updateAttachmentUploads({
        fileIds: [fileID],
        status: AttachmentUploadStatus.AttachmentUploading,
      })
      job.controller.signal.throwIfAborted()
      job.transfer ??= new FileTransfer(job.selected!.file, () =>
        prepareFileUpload(fileID),
      )
      await job.transfer.run(
        job.controller.signal,
        (bytes) => {
          job.bytes = bytes
          this.emit()
        },
        completeAttachmentUpload,
      )
      if (job.cancelRequested || this.disposed || !this.batches.has(job.batchID)) return
      job.stage = "ready"
      job.selected = null
      job.transfer = null
      this.refresh(job.conversationID)
    } catch (error) {
      if (job.cancelRequested || this.disposed || !this.batches.has(job.batchID)) return
      console.warn("附件上传失败", error)
      job.stage = "failed"
      await updateAttachmentUploads({
        fileIds: [fileID],
        status: AttachmentUploadStatus.AttachmentFailed,
      }).catch(() => {})
      this.refresh(job.conversationID)
    } finally {
      this.emit()
    }
  }

  /** 按当前失败阶段重试消息入库、取消或剩余文件内容。 */
  retry(id: string) {
    const job = this.jobs.find((job) => job.id === id)
    if (!job || job.stage !== "failed") return
    const batch = this.batches.get(job.batchID)!
    if (!batch.saved) {
      void this.prepare(batch)
      return
    }
    if (job.cancelRequested) {
      void this.cancel(id).catch(this.reportError)
      return
    }
    job.controller = new AbortController()
    job.stage = "queued"
    this.pump()
  }

  /** 撤去未完成的附件，取消意图在入库请求返回后继续生效。 */
  async cancel(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (!job || (job.stage === "ready" && !job.cancelRequested)) return
    job.cancelRequested = true
    job.stage = "cancelled"
    job.controller.abort()
    // 尚未入库的附件直接丢弃发送项，已入库的等取消请求成功后再丢弃。
    if (!job.fileID) this.outgoing.discard(job.id)
    this.emit()
    try {
      if (job.fileID) {
        await updateAttachmentUploads({
          fileIds: [job.fileID],
          status: AttachmentUploadStatus.AttachmentCancelled,
        })
        this.outgoing.discard(job.id)
        this.refresh(job.conversationID)
      }
    } catch (error) {
      job.stage = "failed"
      this.outgoing.fail(job.id)
      throw error
    } finally {
      if (job.previewURL) URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      job.selected = null
      job.transfer = null
      this.emit()
    }
  }

  /** 远端图片可用后释放本地预览。 */
  releasePreview(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (job?.stage === "ready" && job.previewURL) {
      URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      this.emit()
    }
  }

  /** 失权后释放本地文件与队列，未完成上传由服务端活跃期限收敛。 */
  forgetConversation(conversationID: string) {
    for (const job of this.jobs.filter((item) => item.conversationID === conversationID)) {
      job.controller.abort()
      if (job.previewURL) URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      job.selected = null
      job.transfer = null
      this.outgoing.discard(job.id)
      this.batches.delete(job.batchID)
    }
    this.jobs = this.jobs.filter((job) => job.conversationID !== conversationID)
    this.emit()
  }

  /** 页面关闭时释放本轮上传的文件内容。 */
  dispose() {
    this.disposed = true
    clearInterval(this.heartbeat)
    for (const job of this.jobs) {
      job.controller.abort()
      if (job.previewURL) URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      job.selected = null
      job.transfer = null
      // 未完成的附件在服务端标记失败，发送状态同步为失败。
      if (job.stage !== "ready" && !job.cancelRequested)
        this.outgoing.fail(job.id)
    }
    void this.abandon()
  }

  /** 已入库的未完成附件保留失败消息，异常退出由服务端活跃期限收敛。 */
  private async abandon() {
    const ids = this.jobs
      .filter(
        (job) => job.fileID && job.stage !== "ready" && !job.cancelRequested,
      )
      .map((job) => job.fileID)
    for (let index = 0; index < ids.length; index += 100) {
      await updateAttachmentUploads({
        fileIds: ids.slice(index, index + 100),
        status: AttachmentUploadStatus.AttachmentFailed,
      }).catch(() => {})
    }
  }
}
