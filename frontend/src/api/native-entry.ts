/** 判断原生端进入工作台、登录页还是连接页。 */
import { getInstallationStatus, getServerURL, loadIdentity } from "@/api/auth"
import { ApiError, hasToken } from "@/api/client"
import type { Identity, InstallationStatus } from "@/api/service"

export type NativeEntry =
  | { status: "ready"; identity: Identity }
  | { status: "connect" }
  | { status: "login" }

/** 判断是否已连接到已初始化的企业。 */
function isConnectedEnterprise(status: InstallationStatus) {
  return status.installed && status.organizationName.trim() !== ""
}

/** 判断原生端进入工作台、登录页还是连接页。 */
export async function resolveNativeEntry(): Promise<NativeEntry> {
  if ((await getServerURL()) === "") {
    return { status: "connect" }
  }
  if (!hasToken()) {
    try {
      const status = await getInstallationStatus()
      if (!isConnectedEnterprise(status)) {
        return { status: "connect" }
      }
      return { status: "login" }
    } catch {
      return { status: "connect" }
    }
  }
  try {
    return { status: "ready", identity: await loadIdentity() }
  } catch (error) {
    if (error instanceof ApiError) {
      if (
        error.code === "SERVER_CONNECTION_REQUIRED" ||
        error.code === "INSTALLATION_REQUIRED" ||
        error.code === "SERVER_UNAVAILABLE"
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
