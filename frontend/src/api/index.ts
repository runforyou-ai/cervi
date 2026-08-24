/** 前端业务 API 入口，统一导出认证、绑定调用和契约类型。 */
export {
  ApiError,
  isApiError,
  isNotFoundApiError,
} from "@/api/client"
export {
  connectServer,
  getServerURL,
  install,
  login,
  logout,
  probeServer,
} from "@/api/auth"
export { loadStartup, sessionPath } from "@/api/session"
export { uploadFile, uploadFileContent } from "@/api/uploads"
export * from "@/api/service"
