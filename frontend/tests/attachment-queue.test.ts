/** 验证真实上传执行器和消息队列的重试、取消及页面退出行为。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { runInNewContext } from "node:vm"
import { stripTypeScriptTypes } from "node:module"

const queueSource = readFileSync(
  new URL("../src/features/inbox/attachment-queue.ts", import.meta.url),
  "utf8",
)
const queueCode =
  stripTypeScriptTypes(queueSource)
    .replace(
      /import\s*\{([\s\S]*?)\}\s*from\s*["']@\/api["'];?/,
      "const {$1} = api;",
    )
    .replace("export class AttachmentQueue", "class AttachmentQueue") +
  "\nexports.AttachmentQueue = AttachmentQueue;"
const uploadSource = readFileSync(
  new URL("../src/api/uploads.ts", import.meta.url),
  "utf8",
)
const transferCode =
  stripTypeScriptTypes(
    uploadSource.slice(
      uploadSource.indexOf("export class FileTransfer"),
      uploadSource.indexOf("/** 上传一个文件片段"),
    ),
  ).replace("export class FileTransfer", "class FileTransfer") +
  "\nexports.FileTransfer = FileTransfer;"

/** 用可控制的请求运行实际上传执行器和队列。 */
function host(overrides: Record<string, (...args: any[]) => any> = {}) {
  const updates: { fileIds: string[]; status: string }[] = []
  const errors: unknown[] = []
  let prepared = 0
  let transfers = 0
  let batches = 0
  const api: Record<string, any> = {
    AttachmentUploadStatus: {
      AttachmentUploading: "uploading",
      AttachmentReady: "ready",
      AttachmentFailed: "failed",
      AttachmentCancelled: "cancelled",
    },
    prepareFileUpload: async (id: string) => {
      prepared++
      return { file: { id }, partSize: 0, request: {} }
    },
    updateAttachmentUploads: async (input: any) => {
      updates.push(input)
    },
    createFilePartUpload: async () => ({}),
    uploadFileSlice: async () => {
      transfers++
    },
    completeAttachmentUpload: async () => {},
    sendAttachmentBatch: async (input: any) => {
      batches++
      return {
        conversationId: "conversation",
        conversation: null,
        messages: [
          ...input.attachments.map((item: any) => ({
            id: item.clientMessageId,
            body: item.body,
            originatedAt: new Date().toISOString(),
            attachment: {
              id: `file-${item.clientMessageId}`,
              uploadStatus: "uploading",
            },
          })),

        ],
      }
    },
    ...overrides,
  }
  const exports: Record<string, any> = {}
  runInNewContext(transferCode, {
    exports,
    createFilePartUpload: api.createFilePartUpload,
    uploadFileSlice: api.uploadFileSlice,
  })
  api.FileTransfer = exports.FileTransfer
  runInNewContext(queueCode, {
    exports,
    api,
    crypto,
    AbortController,
    URL,
    setInterval,
    clearInterval,
    console: { warn() {} },
  })
  const queue = new exports.AttachmentQueue(
    () => {},
    (error: unknown) => errors.push(error),
  )
  const files = [1, 2].map((index) => ({
    id: `message-${index}`,
    body: index === 2 ? "说明" : "",
    file: new File(["contents"], `${index}.csv`),
    previewURL: "",
    imageWidth: 0,
    imageHeight: 0,
  }))
  return {
    queue,
    files,
    updates,
    errors,
    api,
    counts: () => ({ prepared, transfers, batches }),
  }
}

/** 等待异步队列达到目标状态。 */
async function settled(predicate: () => boolean) {
  for (let index = 0; index < 100; index++) {
    if (predicate()) return
    await new Promise((resolve) => setImmediate(resolve))
  }
  assert.fail("队列未达到预期状态")
}

test("消息尚未入库时不准备上传，会在返回后继续执行单个文件取消", async () => {
  const gate = Promise.withResolvers<void>()
  const h = host()
  const save = h.api.sendAttachmentBatch
  h.api.sendAttachmentBatch = async (input: any) => {
    await gate.promise
    return save(input)
  }
  // 绑定函数在队列加载时已捕获，使用第二个宿主注入阻塞请求。
  const q = host({ sendAttachmentBatch: h.api.sendAttachmentBatch })
  q.queue.enqueue(q.files, "conversation", "", () => {})
  await q.queue.cancel("message-1")
  assert.equal(q.counts().prepared, 0)
  gate.resolve()
  await settled(() => q.queue.snapshot()[1].stage === "ready")
  assert.equal(q.queue.snapshot().length, 2)
  assert.equal(q.queue.snapshot()[0].message.body, "")
  assert.equal(q.queue.snapshot()[1].message.body, "说明")
  assert.equal(q.queue.snapshot()[1].message.saved.body, "说明")
  assert.equal(q.queue.snapshot()[0].stage, "cancelled")
  assert.equal(q.counts().prepared, 1)
  assert.equal(q.counts().transfers, 1)
  assert.ok(q.updates.some((item) => item.status === "cancelled"))
})

test("单个附件的说明在本地和保存后始终属于同一条消息", async () => {
  const h = host()
  const file = { ...h.files[0], body: "  单个附件说明\n第二行  " }
  h.queue.enqueue([file], "conversation", "", () => {})
  assert.equal(h.queue.snapshot().length, 1)
  assert.equal(h.queue.snapshot()[0].message.body, "单个附件说明\n第二行")
  await settled(() => h.queue.snapshot()[0].stage === "ready")
  assert.equal(h.queue.snapshot().length, 1)
  assert.equal(h.queue.snapshot()[0].message.saved.body, "单个附件说明\n第二行")
})

test("完成请求期间取消会释放文件，迟到的完成响应不能恢复本地消息", async () => {
  const gate = Promise.withResolvers<void>()
  let entered = false
  const h = host({
    completeAttachmentUpload: async () => {
      entered = true
      await gate.promise
    },
  })
  h.queue.enqueue(h.files.slice(0, 1), "conversation", "", () => {})
  await settled(() => entered)
  await h.queue.cancel("message-1")
  gate.resolve()
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(h.queue.snapshot()[0].stage, "cancelled")
  assert.equal(h.queue.snapshot()[0].selected, null)
  assert.equal(h.queue.snapshot()[0].transfer, null)
})

test("离开页面后入库响应才返回，附件标记失败且不会开始上传", async () => {
  const gate = Promise.withResolvers<void>()
  const h = host()
  const q = host({
    sendAttachmentBatch: async (input: any) => {
      await gate.promise
      return h.api.sendAttachmentBatch(input)
    },
  })
  q.queue.enqueue(q.files, "conversation", "", () => {})
  q.queue.dispose()
  gate.resolve()
  await settled(() =>
    q.updates.some(
      (item) => item.status === "failed" && item.fileIds.length === 2,
    ),
  )
  assert.equal(q.counts().prepared, 0)
  assert.equal(q.counts().transfers, 0)
})

test("完成失败后重试复用成功分片，仅重新确认完成", async () => {
  let completed = 0
  const h = host({
    prepareFileUpload: async () => ({
      file: { id: "file" },
      partSize: 2,
      request: {},
    }),
    completeAttachmentUpload: async () => {
      if (++completed === 1) throw new Error("完成暂时失败")
    },
  })
  h.queue.enqueue(h.files.slice(0, 1), "conversation", "", () => {})
  await settled(() => h.queue.snapshot()[0].stage === "failed")
  assert.equal(h.counts().transfers, 4)
  h.queue.retry("message-1")
  await settled(() => h.queue.snapshot()[0].stage === "ready")
  assert.equal(h.counts().transfers, 4)
  assert.equal(h.queue.snapshot()[0].transfer, null)
})

test("入库响应丢失后重试沿用带说明附件的消息编号", async () => {
  const requests: any[] = []
  const h = host()
  const q = host({
    sendAttachmentBatch: async (input: any) => {
      requests.push(input)
      if (requests.length === 1) throw new Error("响应丢失")
      return h.api.sendAttachmentBatch(input)
    },
  })
  q.queue.enqueue(q.files, "conversation", "", () => {})
  await settled(() => q.queue.snapshot()[0].stage === "failed")
  q.queue.retry(q.queue.snapshot()[1].id)
  await settled(() => q.queue.snapshot()[1].stage === "ready")
  assert.equal(JSON.stringify(requests[0]), JSON.stringify(requests[1]))
  assert.equal(q.queue.snapshot()[1].stage, "ready")
  assert.equal(q.queue.snapshot()[1].message.saved.body, "说明")
  assert.equal(requests[0].attachments.length, 2)
  assert.equal(requests[0].attachments[0].body, "")
  assert.equal(requests[0].attachments[1].body, "说明")
})

test("失败分片重试跳过已成功的片", async () => {
  const parts: number[] = []
  let fail = true
  let current = 0
  const h = host({
    prepareFileUpload: async () => ({
      file: { id: "file" },
      partSize: 2,
      request: {},
    }),
    createFilePartUpload: async (
      _id: string,
      input: { partNumber: number },
    ) => {
      current = input.partNumber
      return {}
    },
    uploadFileSlice: async () => {
      parts.push(current)
      if (current === 2 && fail) {
        fail = false
        throw new Error("分片失败")
      }
    },
  })
  h.queue.enqueue(h.files.slice(0, 1), "conversation", "", () => {})
  await settled(() => h.queue.snapshot()[0].stage === "failed")
  h.queue.retry("message-1")
  await settled(() => h.queue.snapshot()[0].stage === "ready")
  assert.deepEqual(parts, [1, 2, 2, 3, 4])
})

test("失权时清除排队附件，迟到的保存结果不能恢复队列或打开详情", async () => {
  const gate = Promise.withResolvers<void>()
  const h = host()
  const q = host({ sendAttachmentBatch: async (input: any) => { await gate.promise; return h.api.sendAttachmentBatch(input) } })
  let opened = 0
  q.queue.enqueue(q.files, "removed", "", () => { opened++ })
  q.queue.forgetConversation("removed")
  assert.equal(q.queue.snapshot().length, 0)
  gate.resolve()
  await settled(() => h.counts().batches === 1)
  await new Promise((resolve) => setImmediate(resolve))
  assert.equal(q.queue.snapshot().length, 0)
  assert.equal(opened, 0)
  assert.equal(q.counts().transfers, 0)
  assert.equal(q.errors.length, 0)
})
