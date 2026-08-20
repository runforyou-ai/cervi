/** 判断原生端进入工作台、登录页还是连接页。 */
import { getServerURL, loadIdentity } from "@/api/auth"
import { ApiError, hasToken } from "@/api/client"
import type { Identity } from "@/api/service"

export type NativeEntry =
  | { status: "ready"; identity: Identity }
  | { status: "connect" }
  | { status: "login" }

/** 判断原生端进入工作台、登录页还是连接页。 */
export async function resolveNativeEntry(): Promise<NativeEntry> {
  if ((await getServerURL()) === "") {
    return { status: "connect" }
  }
  if (!hasToken()) {
    return { status: "login" }
  }
  try {
    return { status: "ready", identity: await loadIdentity() }
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
