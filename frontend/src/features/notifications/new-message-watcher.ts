/** 按成员事件流确认的新消息决定本地通知范围：维护各会话已知末条基线，逐条执行提醒策略。 */
import type { ConversationMessage, InboxConversation } from "@/api"
import type { RealtimeServerFrame } from "@/api/realtime/protocol"

/** 观察器依赖的权威读取、通知投递与错误处理入口。 */
export type NewMessageWatcherPorts = {
  readConversations: () => Promise<InboxConversation[]>
  readConversation: (conversationId: string) => Promise<InboxConversation | null>
  readMessages: (conversationId: string) => Promise<ConversationMessage[]>
  readPendingMentions: (conversationId: string) => Promise<string[]>
  deliver: (conversation: InboxConversation, message: ConversationMessage) => Promise<void>
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
  private seeded = false
  private seeding = false
  private disposed = false

  /** 创建观察器，事件流建立后由问候事件取得首个基线。 */
  constructor(
    identityId: string,
    ports: NewMessageWatcherPorts,
    timing: Partial<NewMessageWatcherTiming> = {},
  ) {
    this.identityId = identityId
    this.ports = ports
    this.timing = { ...defaultTiming, ...timing }
  }

  /** 停止处理，之后到达的事件与在途结果一律丢弃。 */
  dispose() {
    this.disposed = true
    clearTimeout(this.flushTimer)
    this.flushTimer = undefined
    this.pending.clear()
  }

  /** 处理一条服务端事件：连接问候重建基线，会话变化进入本轮合并窗口。 */
  receive(frame: RealtimeServerFrame) {
    if (this.disposed) {
      return
    }
    switch (frame.type) {
      case "server_hello":
        // 冷启动与重连按 catchup 处理：先取当前基线，历史消息只更新未读。
        this.pending.clear()
        this.seeded = false
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

  /** 尚未取得基线且没有在途读取时，登记一次基线读取。 */
  private startSeed() {
    if (this.seeded || this.seeding) {
      return
    }
    this.seeding = true
    this.enqueue(() => this.seed())
  }

  /** 读取当前会话行作为基线，读取失败时按无基线处理。 */
  private async seed() {
    try {
      const conversations = await this.ports.readConversations()
      if (this.disposed) {
        return
      }
      this.baselines.clear()
      for (const conversation of conversations) {
        this.remember(conversation)
      }
    } catch (error) {
      this.baselines.clear()
      this.ports.failed(error)
    } finally {
      this.seeded = true
      this.seeding = false
    }
  }

  /** 处理本轮合并窗口内变化的会话，按到达顺序串行读取。 */
  private flush() {
    this.flushTimer = undefined
    const conversationIds = [...this.pending]
    this.pending.clear()
    this.enqueue(async () => {
      for (const conversationId of conversationIds) {
        if (this.disposed) {
          return
        }
        try {
          await this.process(conversationId)
        } catch (error) {
          this.ports.failed(error)
        }
      }
    })
  }

  /** 读取变化会话的权威行与实际新增范围，逐条执行提醒策略。 */
  private async process(conversationId: string) {
    const baseline = this.baselines.get(conversationId)
    const conversation = await this.ports.readConversation(conversationId)
    if (this.disposed) {
      return
    }
    if (!conversation) {
      this.baselines.delete(conversationId)
      return
    }
    this.remember(conversation)
    const lastMessageId = conversation.lastMessageId
    // 基线尚未建立时只登记当前位置；末条未变或没有未读时只更新状态。
    if (!this.seeded || !lastMessageId || lastMessageId === baseline || conversation.unreadCount <= 0) {
      return
    }
    // 静音单聊、AI 聊天与客户会话不提醒，静音群聊只提醒 @ 本人的消息。
    if (conversation.muted && !conversation.group) {
      return
    }
    const messages = await this.ports.readMessages(conversationId)
    if (this.disposed) {
      return
    }
    const index = baseline ? messages.findIndex((message) => message.id === baseline) : -1
    // 基线缺失或已被翻页移出时只按最新一条判断，不把历史未读当成本次新增。
    const arrived = (index >= 0 ? messages.slice(index + 1) : messages.slice(-1)).slice(
      -conversation.unreadCount,
    )
    const notifiable = arrived.filter(
      (message) =>
        !this.notified.has(message.id) &&
        !message.systemEvent &&
        message.sender !== null &&
        message.sender.sourceId !== this.identityId &&
        (message.body.trim() !== "" || message.attachment !== null),
    )
    if (!notifiable.length) {
      return
    }
    const mentioned = conversation.muted
      ? new Set(await this.ports.readPendingMentions(conversationId))
      : null
    if (this.disposed) {
      return
    }
    for (const message of notifiable) {
      if (mentioned && !mentioned.has(message.id)) {
        continue
      }
      this.markNotified(message.id)
      await this.ports.deliver(conversation, message)
      if (this.disposed) {
        return
      }
    }
  }

  /** 记录会话当前的末条消息位置。 */
  private remember(conversation: InboxConversation) {
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

  /** 按登记顺序串行执行基线读取与会话处理。 */
  private enqueue(task: () => Promise<void>) {
    this.queue = this.queue.then(() => (this.disposed ? undefined : task()))
  }
}
