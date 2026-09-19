/** 登录会话级同步协调器：把实时通知与兜底探针结果转成资源 key 失效，并合并短窗口内的重复失效。 */
import type { SyncHeads } from "@/api"
import type { RealtimeServerFrame } from "@/api/realtime/protocol"
// 前端单元测试由 node 直接加载，运行时依赖使用相对路径。
import { resourceKeys } from "../../hooks/resource-keys.ts"

type ResourceKey = readonly unknown[]

/** 协调器依赖的缓存失效、失败重试、探针读取与错误处理入口。 */
export type SyncCoordinatorPorts = {
  invalidate: (key: ResourceKey) => void
  retry: () => void
  readHeads: () => Promise<SyncHeads>
  failed: (error: unknown) => void
}

/** 合并失效的窗口与兜底探针周期。 */
export type SyncCoordinatorTiming = {
  invalidationWindowMs: number
  probeIntervalMs: number
}

const defaultTiming: SyncCoordinatorTiming = {
  invalidationWindowMs: 300,
  probeIntervalMs: 30_000,
}

/** 返回收件箱列表、列表行、提醒总数、最近会话摘要与搜索结果的失效前缀。 */
function inboxKeys(): ResourceKey[] {
  return [
    resourceKeys.inbox(),
    resourceKeys.inboxConversations(),
    resourceKeys.inboxAttention(),
    resourceKeys.recentConversations(),
    resourceKeys.inboxSearch(),
  ]
}

/** 返回单个会话内容变化时需要重读的资源 key，省略会话编号时返回全部会话的前缀。 */
function conversationKeys(conversationId?: string): ResourceKey[] {
  return [
    ...inboxKeys(),
    resourceKeys.conversationSummary(conversationId),
    resourceKeys.conversationMessages(conversationId),
    resourceKeys.conversationMessagePage(conversationId),
    resourceKeys.conversationNavigation(conversationId),
    resourceKeys.conversationMentions(conversationId),
    resourceKeys.groupConversation(conversationId),
    resourceKeys.customerDeliveries(conversationId),
    resourceKeys.customerCopilotThreads(conversationId),
    resourceKeys.conversationMessageReferences(conversationId),
    resourceKeys.directConversation(),
  ]
}

/** 登录会话内唯一的同步协调器，随登录外壳创建与销毁。 */
export class SyncCoordinator {
  private readonly ports: SyncCoordinatorPorts
  private readonly timing: SyncCoordinatorTiming
  private readonly pending = new Map<string, ResourceKey>()
  private flushTimer: ReturnType<typeof setTimeout> | undefined
  private probeTimer: ReturnType<typeof setInterval> | undefined
  private heads: SyncHeads | null = null
  private headsRevision = 0
  private appliedRevision = 0
  private probing = false
  private probeAgain = false
  private disposed = false

  /** 创建协调器，start 之后开始周期探针。 */
  constructor(ports: SyncCoordinatorPorts, timing: Partial<SyncCoordinatorTiming> = {}) {
    this.ports = ports
    this.timing = { ...defaultTiming, ...timing }
  }

  /** 立即执行一次兜底校验，之后按固定周期无条件执行。 */
  start() {
    if (this.disposed || this.probeTimer !== undefined) {
      return
    }
    this.probeTimer = setInterval(() => void this.probe(), this.timing.probeIntervalMs)
    void this.probe()
  }

  /** 停止周期探针与待合并的失效，之后到达的通知与探针结果一律丢弃。 */
  dispose() {
    this.disposed = true
    clearInterval(this.probeTimer)
    clearTimeout(this.flushTimer)
    this.probeTimer = undefined
    this.flushTimer = undefined
    this.pending.clear()
  }

  /** 处理一条服务端事件：连接问候携带探针值，变更通知映射为对应资源 key 的失效。 */
  receive(frame: RealtimeServerFrame) {
    switch (frame.type) {
      case "server_hello":
        this.headsRevision += 1
        this.applyHeads(frame.syncHeads, this.headsRevision)
        return
      case "conversation_changed":
        this.enqueue(conversationKeys(frame.conversationId))
        return
      case "conversation_state_changed":
        // 群资料携带本人免打扰状态，个人会话状态变化时一并重读。
        this.enqueue([
          ...inboxKeys(),
          resourceKeys.conversationSummary(frame.conversationId),
          resourceKeys.conversationNavigation(frame.conversationId),
          resourceKeys.conversationMentions(frame.conversationId),
          resourceKeys.groupConversation(frame.conversationId),
        ])
        return
      case "conversation_removed":
        // 独立摘要与群资料重读确认阅读资格，失权后由其消费方清理会话资源。
        this.enqueue([
          ...inboxKeys(),
          resourceKeys.conversationSummary(frame.conversationId),
          resourceKeys.groupConversation(frame.conversationId),
        ])
        return
      case "identity_profile_changed":
        this.enqueue([resourceKeys.identity()])
        return
      case "pin_order_changed":
        // 个人置顶顺序变化使置顶区游标失效，两个分区一并整区重读。
        this.enqueue(inboxKeys())
        return
    }
  }

  /** 读取一次同步探针；已有探针在途时于其结束后再读取一次。 */
  probe = async () => {
    if (this.disposed) {
      return
    }
    if (this.probing) {
      this.probeAgain = true
      return
    }
    this.probing = true
    this.headsRevision += 1
    const revision = this.headsRevision
    try {
      this.applyHeads(await this.ports.readHeads(), revision)
    } catch (error) {
      if (!this.disposed) {
        this.ports.failed(error)
      }
    } finally {
      this.probing = false
      if (this.probeAgain && !this.disposed) {
        this.probeAgain = false
        void this.probe()
      }
    }
  }

  /** 与上次探针值比较，不一致的部分失效对应资源；早于已应用结果发起的读取直接丢弃。 */
  private applyHeads(heads: SyncHeads, revision: number) {
    if (this.disposed || revision < this.appliedRevision) {
      return
    }
    this.appliedRevision = revision
    const previous = this.heads
    this.heads = heads
    // 首个探针值无法确认已读取的数据是否早于它，按不一致处理。
    if (
      !previous ||
      previous.conversationCount !== heads.conversationCount ||
      previous.conversationChecksum !== heads.conversationChecksum
    ) {
      this.enqueue(conversationKeys())
    }
    if (!previous || previous.identityProfileVersion !== heads.identityProfileVersion) {
      this.enqueue([resourceKeys.identity()])
    }
    if (!previous || previous.pinOrderVersion !== heads.pinOrderVersion) {
      this.enqueue(inboxKeys())
    }
    // 探针值一致时同样开启合并窗口，窗口结束时重试上次失败的同步读取。
    this.enqueue([])
  }

  /** 登记待失效的 key，并在合并窗口结束时统一失效。 */
  private enqueue(keys: ResourceKey[]) {
    if (this.disposed) {
      return
    }
    for (const key of keys) {
      this.pending.set(JSON.stringify(key), key)
    }
    this.flushTimer ??= setTimeout(() => this.flush(), this.timing.invalidationWindowMs)
  }

  /** 失效本窗口登记的 key，已被同批较短前缀覆盖的 key 不重复失效，之后重试失败的同步读取。 */
  private flush() {
    this.flushTimer = undefined
    const keys = [...this.pending.values()].sort((left, right) => left.length - right.length)
    this.pending.clear()
    const prefixes: ResourceKey[] = []
    for (const key of keys) {
      const covered = prefixes.some((prefix) =>
        prefix.every((part, index) => JSON.stringify(part) === JSON.stringify(key[index])),
      )
      if (!covered) {
        prefixes.push(key)
        this.ports.invalidate(key)
      }
    }
    this.ports.retry()
  }
}
