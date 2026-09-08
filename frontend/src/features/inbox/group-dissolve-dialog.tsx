/** 各端共用的群聊解散确认、提交与查询刷新。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  dissolveGroupConversation,
  isApiError,
  type GroupConversationData,
} from "@/api"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 解散成功后保留当前视图，失败时保留确认框供重试。 */
export function GroupDissolveDialog({
  group,
  open,
  onOpenChange,
  trigger,
}: {
  group: GroupConversationData
  open: boolean
  onOpenChange: (open: boolean) => void
  trigger?: HTMLElement | null
}) {
  const { t } = useTranslation(["inbox", "common"])
  const navigate = useNavigate()
  const save = useImmediateSave()
  const invalidate = useResourceInvalidator()

  /** 提交解散并刷新各端共享的群资料、消息和列表。 */
  async function dissolve() {
    const request = save.begin()
    if (request === null) return
    try {
      await dissolveGroupConversation(group.id)
      await Promise.all([
        invalidate(resourceKeys.groupConversation(group.id)),
        invalidate(resourceKeys.conversationMessages(group.id)),
        invalidate(resourceKeys.inbox()),
      ])
      if (save.isCurrent(request)) onOpenChange(false)
    } catch (error) {
      if (!save.isCurrent(request) || recoverSession(error, navigate)) return
      console.warn("解散群聊失败", { conversationID: group.id, error })
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("groupDissolveError"),
      )
      void invalidate(resourceKeys.groupConversation(group.id))
    } finally {
      save.finish(request)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (!save.isSaving()) onOpenChange(next)
      }}
    >
      <AlertDialogContent
        onCloseAutoFocus={
          trigger
            ? (event) => {
                event.preventDefault()
                trigger.focus({ preventScroll: true })
              }
            : undefined
        }
      >
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t("groupDissolveTitle", { name: group.title })}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t("groupDissolveDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel className="min-h-11" disabled={save.saving}>
            {t("common:actions.cancel")}
          </AlertDialogCancel>
          <Button
            className="min-h-11"
            variant="destructive"
            disabled={save.saving}
            onClick={() => void dissolve()}
          >
            {t("groupDissolveConfirm")}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
