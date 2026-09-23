/** 按成员事件流确认的新消息决定本地通知范围：维护各会话已知末条基线，逐条执行提醒策略。 */
import type { ConversationMessage, InboxConversationData } from "@/api"
import type { RealtimeServerFrame } from "@/api/realtime/protocol"

/** 观察器依赖的权威读取、通知投递与错误处理入口。 */
export type NewMessageWatcherPorts = {
  readConversations: () => Promise<InboxConversationData[]>
  readConversation: (conversationId: string) => Promise<InboxConversationData | null>
  readMessages: (conversationId: string) => Promise<ConversationMessage[]>
  readPendingMentions: (conversationId: string) => Promise<string[]>
  deliver: (conversation: InboxConversationData, message: ConversationMessage) => Promise<void>
  failed: (error: unknown) => void
}

/** 合并同一批变更通知的窗口。 */
export type NewMessageWatcherTiming = {
  mergeWindowMs: number
}

const defaultTiming: NewMessageWatcherTiming = {
  mergeWindowMs: 300,
}

const notifiedMessageLimit = 500

/** 登录会话内唯一的新消息观察器，随登录外壳创建与销毁。 */
export class NewMessageWatcher {
  private readonly identityId: string
  private readonly ports: NewMessageWatcherPorts
  private readonly timing: NewMessageWatcherTiming
  private readonly baselines = new Map<string, string>()
  private readonly notified = new Set<string>()
  private readonly pending = new Set<string>()
  private queue: Promise<void> = Promise.resolve()
  private flushTimer: ReturnType<typeof setTimeout> | undefined
  private connection = 0
  private seededConnection = -1
  private seedingConnection = -1
  private disposed = false

  /** 创建观察器，事件流建立后由问候事件或 start 取得首个基线。 */
  constructor(
    identityId: string,
    ports: NewMessageWatcherPorts,
    timing: Partial<NewMessageWatcherTiming> = {},
  ) {
    this.identityId = identityId
    this.ports = ports
    this.timing = { ...defaultTiming, ...timing }
  }

  /** 订阅时事件流已经建立的情况下立即取得首个基线。 */
  start() {
    if (this.disposed) {
      return
    }
    this.startSeed()
  }

  /** 停止处理，之后到达的事件与在途结果一律丢弃。 */
  dispose() {
    this.disposed = true
    clearTimeout(this.flushTimer)
    this.flushTimer = undefined
    this.pending.clear()
  }

  /** 处理一条服务端事件：连接问候开启新的连接代次并重建基线，会话变化进入本轮合并窗口。 */
  receive(frame: RealtimeServerFrame) {
    if (this.disposed) {
      return
    }
    switch (frame.type) {
      case "server_hello":
        // 冷启动与重连按 catchup 处理：新代次先取当前基线，历史消息只更新未读，上一代次的在途结果与待处理会话一律丢弃。
        this.connection += 1
        this.pending.clear()
        clearTimeout(this.flushTimer)
        this.flushTimer = undefined
        this.startSeed()
        return
      case "conversation_changed":
        // 订阅时事件流已建立而没有问候事件时，同样先取基线再处理本次变化。
        this.startSeed()
        this.pending.add(frame.conversationId)
        this.flushTimer ??= setTimeout(() => this.flush(), this.timing.mergeWindowMs)
        return
      case "conversation_removed":
        this.pending.delete(frame.conversationId)
        this.baselines.delete(frame.conversationId)
        return
    }
  }

  /** 当前连接代次尚未取得基线且没有在途读取时，登记一次基线读取。 */
  private startSeed() {
    if (this.seededConnection === this.connection || this.seedingConnection === this.connection) {
      return
    }
    this.seedingConnection = this.connection
    const connection = this.connection
    this.enqueue(() => this.seed(connection))
  }

  /** 读取当前会话行作为本代次基线，读取失败时按无基线处理。 */
  private async seed(connection: number) {
    try {
      const conversations = await this.ports.readConversations()
      if (this.stale(connection)) {
        return
      }
      this.baselines.clear()
      for (const conversation of conversations) {
        this.remember(conversation)
      }
    } catch (error) {
      if (this.stale(connection)) {
        return
      }
      this.baselines.clear()
      this.ports.failed(error)
    } finally {
      if (connection === this.connection) {
        this.seededConnection = connection
        this.seedingConnection = -1
      }
    }
  }

  /** 处理本轮合并窗口内变化的会话，按到达顺序串行读取。 */
  private flush() {
    this.flushTimer = undefined
    const conversationIds = [...this.pending]
    this.pending.clear()
    const connection = this.connection
    this.enqueue(async () => {
      for (const conversationId of conversationIds) {
        if (this.stale(connection)) {
          return
        }
        try {
          await this.process(conversationId, connection)
        } catch (error) {
          this.ports.failed(error)
        }
      }
    })
  }

  /** 读取变化会话的权威行与实际新增范围，逐条执行提醒策略。 */
  private async process(conversationId: string, connection: number) {
    const baseline = this.baselines.get(conversationId)
    const conversation = await this.ports.readConversation(conversationId)
    if (this.stale(connection)) {
      return
    }
    if (!conversation) {
      this.baselines.delete(conversationId)
      return
    }
    const lastMessageId = conversation.lastMessageId
    // 基线尚未建立、末条未变、没有未读，以及静音的单聊、AI 聊天与客户会话，只登记当前位置。
    if (
      this.seededConnection !== connection ||
      !lastMessageId ||
      lastMessageId === baseline ||
      conversation.unreadCount <= 0 ||
      (conversation.muted && !conversation.group)
    ) {
      this.remember(conversation)
      return
    }
    const messages = await this.ports.readMessages(conversationId)
    if (this.stale(connection)) {
      return
    }
    const baselineIndex = baseline ? messages.findIndex((message) => message.id === baseline) : -1
    const readIndex = conversation.lastReadMessageId
      ? messages.findIndex((message) => message.id === conversation.lastReadMessageId)
      : -1
    // 新增范围从已知末条与已读水位中靠后的一端开始；基线缺失或已被翻页移出时只按最新一条判断。
    const arrived =
      baselineIndex >= 0 ? messages.slice(Math.max(baselineIndex, readIndex) + 1) : messages.slice(-1)
    const notifiable = arrived.filter(
      (message) =>
        !this.notified.has(message.id) &&
        !message.systemEvent &&
        message.sender !== null &&
        message.sender.sourceId !== this.identityId &&
        (message.body.trim() !== "" || message.attachment !== null),
    )
    // 静音群聊只提醒 @ 本人的消息。
    const mentioned =
      conversation.muted && notifiable.length
        ? new Set(await this.ports.readPendingMentions(conversationId))
        : null
    if (this.stale(connection)) {
      return
    }
    for (const message of notifiable) {
      if (mentioned && !mentioned.has(message.id)) {
        continue
      }
      this.markNotified(message.id)
      await this.ports.deliver(conversation, message)
      if (this.stale(connection)) {
        return
      }
    }
    // 权威读取与投递全部完成后才推进基线，读取失败时保留原位置，下一次事件重新处理该范围。
    this.remember(conversation)
  }

  /** 判断观察器已销毁或该结果属于过期的连接代次。 */
  private stale(connection: number) {
    return this.disposed || connection !== this.connection
  }

  /** 记录会话当前的末条消息位置。 */
  private remember(conversation: InboxConversationData) {
    if (conversation.lastMessageId) {
      this.baselines.set(conversation.id, conversation.lastMessageId)
      return
    }
    this.baselines.delete(conversation.id)
  }

  /** 登记已投递的消息编号，超出上限时淘汰最早的记录。 */
  private markNotified(messageId: string) {
    this.notified.add(messageId)
    if (this.notified.size > notifiedMessageLimit) {
      const oldest = this.notified.values().next()
      if (!oldest.done) {
        this.notified.delete(oldest.value)
      }
    }
  }

  /** 按登记顺序串行执行基线读取与会话处理，单个任务失败后队列继续接受后续任务。 */
  private enqueue(task: () => Promise<void>) {
    this.queue = this.queue
      .then(() => (this.disposed ? undefined : task()))
      .catch((error) => {
        this.ports.failed(error)
      })
  }
}
