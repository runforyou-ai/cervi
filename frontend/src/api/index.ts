/** 前端业务 API 入口，统一导出认证、绑定调用和契约类型。 */
export { ApiError, hasSession } from "@/api/client"
export {
  connectServer,
  getServerURL,
  install,
  loadSession,
  login,
  logout,
} from "@/api/auth"
export { resolveNativeEntry, type NativeEntry } from "@/api/native-entry"
export * from "@/api/service"
