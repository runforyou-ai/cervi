/** 在工作台内上传附件，传完后按选择顺序逐个发送为消息。 */
import {
  FilePurpose,
  FileTransfer,
  cancelFileUpload,
  completeFileUpload,
  createFileUpload,
  MessageVisibility,
  sendAttachmentMessage,
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
  body: string
  attachment: MessageAttachment
  fileID: string
  messageID: string
  stage: "queued" | "uploading" | "uploaded" | "sending" | "sent" | "failed" | "cancelled"
  selected: SelectedAttachment | null
  previewURL: string
  transfer: FileTransfer | null
  bytes: number
  controller: AbortController
}

type Batch = {
  id: string
  conversationID: string
  targetIdentityID: string
  agentIdentityID: string
  customerConversationID: string
  jobs: AttachmentJob[]
  sending: boolean
  onCreated: (conversation: InboxConversation | null, conversationID: string) => void
}

/** 同时上传最多三个文件，并按每批的选择顺序串行发送已上传的附件。 */
export class AttachmentQueue {
  private jobs: AttachmentJob[] = []
  private batches = new Map<string, Batch>()
  private listeners = new Set<() => void>()
  private active = 0
  private disposed = false
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

  /** 页面挂载后开始接收附件任务。 */
  start() {
    this.disposed = false
  }

  /** 立即展示本地附件气泡，并开始上传所选文件。 */
  enqueue(
    files: (SelectedAttachment & { body: string })[],
    {
      conversationID,
      targetIdentityID = "",
      agentIdentityID = "",
      customerConversationID = "",
    }: { conversationID: string; targetIdentityID?: string; agentIdentityID?: string; customerConversationID?: string },
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
        imageWidth: selected.imageWidth,
        imageHeight: selected.imageHeight,
      }
      this.outgoing.start(scopeID, {
        clientMessageID: selected.id,
        attachment,
        visibility: MessageVisibility.MessageVisibilityCustomerVisible,
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
        conversationID: targetIdentityID ? "" : conversationID,
        body: selected.body.trim(),
        attachment,
        fileID: "",
        messageID: "",
        stage: "queued",
        selected,
        previewURL: selected.previewURL,
        transfer: null,
        bytes: 0,
        controller: new AbortController(),
      }
    })
    this.batches.set(batchID, {
      id: batchID,
      conversationID,
      targetIdentityID,
      agentIdentityID,
      customerConversationID,
      jobs,
      sending: false,
      onCreated,
    })
    this.jobs = [...this.jobs, ...jobs]
    this.pump()
  }

  /** 为排队的附件分配上传名额。 */
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

  /** 上传文件内容并确认临时文件，失败时保留已成功的分片供重试。 */
  private async upload(job: AttachmentJob) {
    const file = job.selected!.file
    try {
      job.transfer ??= new FileTransfer(file, () =>
        createFileUpload({
          purpose: FilePurpose.FilePurposeMessageAttachment,
          fileName: file.name,
          contentType: file.type,
          byteSize: file.size,
        }),
      )
      const uploaded = await job.transfer.run(
        job.controller.signal,
        (bytes) => {
          job.bytes = bytes
          this.emit()
        },
        completeFileUpload,
      )
      if (job.stage !== "uploading") return
      job.fileID = uploaded.id
      job.stage = "uploaded"
    } catch (error) {
      if (job.stage !== "uploading") return
      console.warn("附件上传失败", error)
      job.stage = "failed"
      this.outgoing.fail(job.id)
    } finally {
      this.emit()
      void this.flush(job.batchID)
    }
  }

  /** 按选择顺序发送已上传的附件，前面的附件未结束时等待，失败或取消的附件不阻塞后续附件。 */
  private async flush(batchID: string) {
    const batch = this.batches.get(batchID)
    if (!batch || batch.sending) return
    batch.sending = true
    try {
      for (;;) {
        const job = batch.jobs.find(
          (item) =>
            item.stage !== "sent" &&
            item.stage !== "failed" &&
            item.stage !== "cancelled",
        )
        if (job?.stage !== "uploaded" || this.disposed || !this.batches.has(batchID)) break
        await this.send(batch, job)
      }
    } finally {
      batch.sending = false
    }
  }

  /** 以固定发送编号发送一个已上传的附件，草稿首发成功后后续附件改用正式会话。 */
  private async send(batch: Batch, job: AttachmentJob) {
    job.stage = "sending"
    this.emit()
    try {
      const result = await sendAttachmentMessage({
        conversationId: batch.targetIdentityID ? "" : batch.conversationID,
        targetIdentityId: batch.targetIdentityID,
        agentIdentityId: batch.agentIdentityID,
        customerConversationId: batch.customerConversationID,
        clientMessageId: job.id,
        fileId: job.fileID,
        body: job.body,
        imageWidth: job.attachment.imageWidth,
        imageHeight: job.attachment.imageHeight,
      })
      if (job.stage !== "sending" || !this.batches.has(batch.id)) return
      job.stage = "sent"
      job.messageID = result.message.id
      job.selected = null
      job.transfer = null
      this.outgoing.succeed(job.id, result.message)
      if (batch.targetIdentityID || batch.agentIdentityID) {
        batch.conversationID = result.conversationId
        batch.targetIdentityID = ""
        batch.agentIdentityID = ""
        batch.customerConversationID = ""
        for (const item of batch.jobs) item.conversationID = result.conversationId
        batch.onCreated(result.conversation, result.conversationId)
      }
      this.refresh(result.conversationId)
    } catch (error) {
      if (job.stage !== "sending" || !this.batches.has(batch.id)) return
      console.warn("附件消息发送失败", error)
      job.stage = "failed"
      this.outgoing.fail(job.id)
      if (!this.disposed) this.reportError(error)
    } finally {
      this.emit()
    }
  }

  /** 重试失败的附件：已上传的重新发送，未上传的从已成功的分片继续上传。 */
  retry(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (!job || job.stage !== "failed") return
    // 重试的附件移到本批末尾，成功后排在同批其余附件之后。
    const batch = this.batches.get(job.batchID)
    if (batch) batch.jobs = [...batch.jobs.filter((item) => item !== job), job]
    job.controller = new AbortController()
    if (job.fileID) {
      job.stage = "uploaded"
      this.emit()
      void this.flush(job.batchID)
    } else {
      job.stage = "queued"
      this.pump()
    }
  }

  /** 撤去尚未开始发送的附件，释放临时文件并放行同批后续附件。 */
  cancel(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (!job || job.stage === "sending" || job.stage === "sent" || job.stage === "cancelled") return
    this.release(job)
    job.stage = "cancelled"
    this.outgoing.discard(job.id)
    this.emit()
    void this.flush(job.batchID)
  }

  /** 远端图片可用后释放本地预览。 */
  releasePreview(id: string) {
    const job = this.jobs.find((item) => item.id === id)
    if (job?.stage === "sent" && job.previewURL) {
      URL.revokeObjectURL(job.previewURL)
      job.previewURL = ""
      this.emit()
    }
  }

  /** 失权后中止该会话的附件任务并清除本地气泡。 */
  forgetConversation(conversationID: string) {
    for (const job of this.jobs.filter((item) => item.conversationID === conversationID)) {
      if (job.stage !== "cancelled") this.release(job)
      job.stage = "cancelled"
      this.outgoing.discard(job.id)
      this.batches.delete(job.batchID)
    }
    this.jobs = this.jobs.filter((job) => job.conversationID !== conversationID)
    this.emit()
  }

  /** 页面关闭时中止未发送的附件并标记为发送失败，迟到的发送结果不再改写状态。 */
  dispose() {
    this.disposed = true
    for (const job of this.jobs) {
      if (job.stage === "sent" || job.stage === "cancelled") continue
      this.release(job)
      job.stage = "failed"
      this.outgoing.fail(job.id)
    }
  }

  /** 中止附件任务，释放本地预览和尚未发送的临时文件。 */
  private release(job: AttachmentJob) {
    job.controller.abort()
    const fileID = job.fileID || job.transfer?.upload?.file.id
    if (fileID && job.stage !== "sent") {
      void cancelFileUpload(fileID).catch((error) =>
        console.warn("附件临时文件释放失败", error),
      )
    }
    if (job.previewURL) URL.revokeObjectURL(job.previewURL)
    job.previewURL = ""
    job.selected = null
    job.transfer = null
  }
}
