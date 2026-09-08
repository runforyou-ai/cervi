/** 将附件上传队列保持在整个已登录工作台的生命周期内。 */
import {
  createContext,
  useContext,
  useEffect,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { isApiError } from "@/api"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { resourceKeys } from "@/hooks/resource-keys"
import { AttachmentQueue } from "./attachment-queue"

const AttachmentQueueContext = createContext<AttachmentQueue | null>(null)

/** 为所有聊天页面提供同一上传队列。 */
export function AttachmentQueueProvider({ children }: { children: ReactNode }) {
  const invalidate = useResourceInvalidator()
  const navigate = useNavigate()
  const { t } = useTranslation("inbox")
  const [queue] = useState(
    () =>
      new AttachmentQueue(
        (conversationID) => {
          void invalidate(resourceKeys.conversationMessages(conversationID))
          void invalidate(resourceKeys.inbox())
          void invalidate(resourceKeys.attachmentStates(conversationID))
        },
        (error) => {
          if (!recoverSession(error, navigate)) {
            toast.error(
              isApiError(error)
                ? apiErrorMessage(error, [])
                : t("messageSendError"),
            )
          }
        },
      ),
  )
  useEffect(() => {
    queue.start()
    const leave = () => queue.dispose()
    window.addEventListener("pagehide", leave)
    return () => {
      window.removeEventListener("pagehide", leave)
      queue.dispose()
    }
  }, [queue])
  return (
    <AttachmentQueueContext value={queue}>{children}</AttachmentQueueContext>
  )
}

/** 读取当前工作台队列及其上传快照。 */
export function useAttachmentQueue() {
  const queue = useContext(AttachmentQueueContext)
  const jobs = useSyncExternalStore(
    queue?.subscribe ?? emptySubscribe,
    queue?.snapshot ?? emptySnapshot,
  )
  return { queue, jobs }
}
const emptyJobs: ReturnType<AttachmentQueue["snapshot"]> = []
/** 未提供附件队列时返回空订阅。 */
function emptySubscribe() {
  return () => {}
}
/** 未提供附件队列时返回稳定空快照。 */
function emptySnapshot() {
  return emptyJobs
}
