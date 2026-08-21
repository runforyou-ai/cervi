/** 前端业务 API 入口，统一导出认证、绑定调用和契约类型。 */
export {
  ApiError,
  isApiError,
  isNotFoundApiError,
  isUnavailableApiError,
} from "@/api/client"
export {
  connectServer,
  getServerURL,
  install,
  login,
  logout,
  probeServer,
} from "@/api/auth"
export { loadSession, sessionPath } from "@/api/session"
export * from "@/api/service"
