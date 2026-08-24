/** 统一管理应用启动和显式会话转换。 */
import { SessionState, type Session } from "@/api"
import { clearToken } from "@/api/client"

export type SessionReloadReason =
  | "startup"
  | "retry"
  | "revalidate"
  | "login"
  | "logout"
  | "setup"
  | "connect"

export type SessionSnapshot = {
  status: "loading" | "loaded" | "failed"
  session: Session | null
  error: unknown
}

type SessionListener = () => void

/** 保存会话快照，并确保过期请求结果不会覆盖较新的转换。 */
export class SessionController {
  private snapshot: SessionSnapshot = {
    status: "loading",
    session: null,
    error: null,
  }
  private readonly listeners = new Set<SessionListener>()
  private requestId = 0
  private started = false

  public constructor(private readonly loadSession: () => Promise<Session>) {}

  /** 启动时只发起一次会话读取。 */
  public start = () => {
    if (this.started) {
      return
    }
    this.started = true
    void this.reload("startup")
  }

  /** 返回 React 订阅使用的当前快照。 */
  public getSnapshot = () => this.snapshot

  /** 订阅会话快照变化。 */
  public subscribe = (listener: SessionListener) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  /** 重新读取权威会话状态。 */
  public reload = async (reason: SessionReloadReason) => {
    const currentRequestId = ++this.requestId
    const preserveReady =
      reason === "revalidate" &&
      this.snapshot.status === "loaded" &&
      this.snapshot.session?.state === SessionState.SessionStateReady

    if (!preserveReady) {
      this.commit({ status: "loading", session: null, error: null })
    }

    try {
      const session = await this.loadSession()
      if (currentRequestId !== this.requestId) {
        return
      }
      this.commit({ status: "loaded", session, error: null })
    } catch (error) {
      if (currentRequestId !== this.requestId) {
        return
      }
      console.warn("读取会话失败", { reason, error })
      if (preserveReady) {
        this.commit({ ...this.snapshot, error })
        return
      }
      this.commit({ status: "failed", session: null, error })
    }
  }

  /** 提交接口明确返回的会话入口。 */
  public commitClassified = (state: string): boolean => {
    if (
      state !== SessionState.SessionStateLogin &&
      state !== SessionState.SessionStateSetup &&
      state !== SessionState.SessionStateConnect
    ) {
      return false
    }
    this.requestId += 1
    if (state === SessionState.SessionStateLogin) {
      clearToken()
    }
    const organizationName =
      this.snapshot.session?.organizationName ??
      this.snapshot.session?.identity?.organization.name
    if (
      state === SessionState.SessionStateLogin &&
      !organizationName?.trim()
    ) {
      void this.reload("retry")
      return true
    }
    this.commit({
      status: "loaded",
      session: {
        state,
        organizationName:
          state === SessionState.SessionStateLogin
            ? organizationName
            : undefined,
      },
      error: null,
    })
    return true
  }

  /** 原子更新快照并通知所有订阅者。 */
  private commit(snapshot: SessionSnapshot) {
    this.snapshot = snapshot
    this.listeners.forEach((listener) => listener())
  }
}
