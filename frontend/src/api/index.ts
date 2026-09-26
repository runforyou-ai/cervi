/** 前端业务 API 入口，聚合各业务域模块并导出契约类型。 */
export * from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
export { ApiError, isApiError, isNotFoundApiError } from "@/api/client"
export {
  completeOfficialLogin,
  connectServer,
  getServerURL,
  install,
  login,
  logout,
  OfficialLoginStateError,
  probeServer,
  startOfficialLogin,
} from "@/api/auth"
export { getSyncHeads, loadIdentity, loadStartup, sessionPath } from "@/api/session"
export {
  createRunStreamClient,
  realtimeClient,
  type RealtimeClientEvent,
  type RealtimeState,
  type RunStreamBlock,
  type RunStreamEvent,
  type RunStreamPlanTask,
  type RunStreamState,
  type RunStreamToolCall,
} from "@/api/realtime"
export {
  completeFileUpload,
  createFilePartUpload,
  prepareFileUpload,
  FileTransfer,
  cancelFileUpload,
  uploadFileSlice,
  createFileUpload,
  uploadFile,
} from "@/api/uploads"
export * from "@/api/agents"
export * from "@/api/assistants"
export * from "@/api/ai-providers"
export * from "@/api/mcp-servers"
export * from "@/api/web-search"
export * from "@/api/channels"
export * from "@/api/contacts"
export * from "@/api/devices"
export * from "@/api/conversation-windows"
export * from "@/api/inbox"
export * from "@/api/knowledge-bases"
export * from "@/api/knowledge-gaps"
export * from "@/api/notifications"
export * from "@/api/reports"
export * from "@/api/roles"
export * from "@/api/settings"
export * from "@/api/translations"
export * from "@/api/teams"
export * from "@/api/users"
