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
import { useOutgoingMessageStore } from "./outgoing-message-context"

const AttachmentQueueContext = createContext<AttachmentQueue | null>(null)

/** 为所有聊天页面提供同一上传队列。 */
export function AttachmentQueueProvider({ children }: { children: ReactNode }) {
  const invalidate = useResourceInvalidator()
  const navigate = useNavigate()
  const { t } = useTranslation("inbox")
  const outgoing = useOutgoingMessageStore()
  const [queue] = useState(
    () =>
      new AttachmentQueue(
        outgoing,
        (conversationID) => {
          void invalidate(resourceKeys.conversationMessages(conversationID))
          void invalidate(resourceKeys.inbox())
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
    // 页面从往返缓存恢复时重新接收附件任务。
    const resume = (event: PageTransitionEvent) => {
      if (event.persisted) queue.start()
    }
    window.addEventListener("pagehide", leave)
    window.addEventListener("pageshow", resume)
    return () => {
      window.removeEventListener("pagehide", leave)
      window.removeEventListener("pageshow", resume)
      queue.dispose()
    }
  }, [queue])
  return (
    <AttachmentQueueContext value={queue}>{children}</AttachmentQueueContext>
  )
}

/** 读取当前工作台的附件队列。 */
export function useAttachmentQueue() {
  return useContext(AttachmentQueueContext)
}

/** 订阅与消息或附件对应的上传任务状态，其他任务变化时不重渲染。 */
export function useAttachmentJob(messageID: string, attachmentID: string) {
  const queue = useContext(AttachmentQueueContext)
  return useSyncExternalStore(queue?.subscribe ?? emptySubscribe, () =>
    queue?.find(messageID, attachmentID),
  )
}
/** 未提供附件队列时返回空订阅。 */
function emptySubscribe() {
  return () => {}
}
