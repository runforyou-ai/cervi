/** 验证附件队列在取消、离开页面和重试并发下保持发送意图。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { runInNewContext } from "node:vm"
import { stripTypeScriptTypes } from "node:module"

const source = readFileSync(new URL("../src/features/inbox/attachment-queue.ts", import.meta.url), "utf8")
const code = stripTypeScriptTypes(source)
  .replace(/import\s*\{([\s\S]*?)\}\s*from\s*["']@\/api["'];?/, "const {$1} = api;")
  .replace("export class AttachmentQueue", "class AttachmentQueue") + "\nexports.AttachmentQueue = AttachmentQueue;"

/** 用可控制的 API 请求运行实际队列代码。 */
function host(overrides: Record<string, (...args: any[]) => any> = {}) {
  const updates: string[] = []
  const cleaned: string[] = []
  const errors: unknown[] = []
  let created = 0
  let transfers = 0
  let batches = 0
  const api = {
    AttachmentUploadStatus: { AttachmentUploading: "uploading", AttachmentReady: "ready", AttachmentFailed: "failed", AttachmentCancelled: "cancelled" },
    FilePurpose: { FilePurposeMessageAttachment: "message_attachment" },
    createFileUpload: async () => ({ file: { id: `file-${++created}` }, partSize: 0, request: {} }),
    cancelFileUpload: async (id: string) => { cleaned.push(id) },
    updateAttachmentUploads: async (input: { status: string }) => { updates.push(input.status) },
    createFilePartUpload: async () => ({}),
    uploadFileSlice: async () => { transfers++ },
    completeFileUpload: async () => ({}),
    completeFileMultipartUpload: async () => ({}),
    sendAttachmentBatch: async (input: any) => {
      batches++
      return {
        conversationId: "conversation", conversation: null,
        messages: input.attachments.map((item: any) => ({ id: item.clientMessageId, originatedAt: new Date().toISOString(), attachment: { id: item.fileId, uploadStatus: "uploading" } })),
      }
    },
    ...overrides,
  }
  const exports: Record<string, any> = {}
  runInNewContext(code, { exports, api, crypto, AbortController, URL, setInterval, clearInterval, console: { warn() {} } })
  const queue = new exports.AttachmentQueue(() => {}, (error: unknown) => errors.push(error))
  const files = [1, 2].map((index) => ({ id: `message-${index}`, file: new File(["contents"], `${index}.csv`), previewURL: "", imageWidth: 0, imageHeight: 0 }))
  return { queue, files, updates, cleaned, errors, counts: () => ({ created, transfers, batches }) }
}

/** 等待队列完成异步状态转移。 */
async function settled(check: () => boolean) {
  const deadline = Date.now() + 2000
  while (!check()) {
    if (Date.now() > deadline) assert.fail("队列状态没有按预期收敛")
    await new Promise((resolve) => setTimeout(resolve, 1))
  }
}

test("完成请求未返回时取消，不能再发送 ready，且释放文件引用", async () => {
  const entered = Promise.withResolvers<void>()
  const complete = Promise.withResolvers<void>()
  const h = host({ completeFileUpload: async () => { entered.resolve(); await complete.promise } })
  h.queue.enqueue(h.files.slice(0, 1), "", "conversation", "", () => {})
  await entered.promise
  await h.queue.cancel("message-1")
  complete.resolve()
  await new Promise((resolve) => setTimeout(resolve, 10))
  assert.equal(h.queue.snapshot()[0].stage, "cancelled")
  assert.equal(h.queue.snapshot()[0].selected, null)
  assert.ok(h.updates.includes("cancelled"))
  assert.ok(!h.updates.includes("ready"))
})

test("准备批次期间取消单个文件，不破坏其他文件的入库和上传", async () => {
  const gate = Promise.withResolvers<void>()
  let count = 0
  const h = host({ createFileUpload: async () => { const id = `file-${++count}`; await gate.promise; return { file: { id }, partSize: 0, request: {} } } })
  h.queue.enqueue(h.files, "", "conversation", "", () => {})
  await h.queue.cancel("message-1")
  gate.resolve()
  await settled(() => h.queue.snapshot()[1].stage === "ready")
  assert.equal(h.queue.snapshot()[0].stage, "cancelled")
  assert.equal(h.queue.snapshot()[0].selected, null)
  assert.equal(h.counts().transfers, 1)
  assert.equal(h.errors.length, 0)
})

test("离开页面时清理此前已创建及随后才返回的临时文件", async () => {
  const entered = Promise.withResolvers<void>()
  const gate = Promise.withResolvers<void>()
  let count = 0
  const h = host({ createFileUpload: async () => {
    const id = `file-${++count}`
    if (count === 2) { entered.resolve(); await gate.promise }
    return { file: { id }, partSize: 0, request: {} }
  } })
  h.queue.enqueue(h.files, "", "conversation", "", () => {})
  await entered.promise
  h.queue.dispose()
  gate.resolve()
  await settled(() => h.cleaned.includes("file-1") && h.cleaned.includes("file-2"))
  assert.equal(h.counts().batches, 0)
  assert.equal(h.counts().transfers, 0)
})

test("分片合并失败后重试不会重复上传成功的分片", async () => {
  let completed = 0
  const h = host({
    createFileUpload: async () => ({ file: { id: "file" }, partSize: 2, request: {} }),
    completeFileMultipartUpload: async () => { if (++completed === 1) throw new Error("合并暂时失败") },
  })
  h.queue.enqueue(h.files.slice(0, 1), "", "conversation", "", () => {})
  await settled(() => h.queue.snapshot()[0].stage === "failed")
  assert.equal(h.counts().transfers, 4)
  assert.ok(h.queue.snapshot()[0].selected)
  h.queue.retry("message-1")
  await settled(() => h.queue.snapshot()[0].stage === "ready")
  assert.equal(h.counts().transfers, 4)
  assert.equal(h.queue.snapshot()[0].selected, null)
})
