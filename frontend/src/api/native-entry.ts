import { getServerURL, loadSession } from "@/api/auth"
import { ApiError, hasSession } from "@/api/client"
import type { Principal } from "@/api/service"

export type NativeEntry =
  | { status: "ready"; principal: Principal }
  | { status: "connect" }
  | { status: "login" }

/** 判断原生端进入工作台、登录页还是连接页。 */
export async function resolveNativeEntry(): Promise<NativeEntry> {
  if ((await getServerURL()) === "") {
    return { status: "connect" }
  }
  if (!hasSession()) {
    return { status: "login" }
  }
  try {
    return { status: "ready", principal: await loadSession() }
  } catch (error) {
    if (error instanceof ApiError) {
      if (
        error.code === "SERVER_CONNECTION_REQUIRED" ||
        error.code === "INSTALLATION_REQUIRED"
      ) {
        return { status: "connect" }
      }
      if (error.code === "AUTH_REQUIRED") {
        return { status: "login" }
      }
    }
    throw error
  }
}
