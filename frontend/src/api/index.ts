/** 前端业务 API 入口，统一导出认证、绑定调用和契约类型。 */
export { ApiError, hasToken } from "@/api/client"
export {
  connectServer,
  getInstallationStatus,
  getServerURL,
  install,
  loadIdentity,
  login,
  logout,
  probeServer,
} from "@/api/auth"
export { resolveNativeEntry, type NativeEntry } from "@/api/native-entry"
export * from "@/api/service"
