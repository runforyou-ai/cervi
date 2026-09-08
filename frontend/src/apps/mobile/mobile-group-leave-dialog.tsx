/** 移动端普通成员退出群聊的确认交互。 */
import { useTranslation } from "react-i18next"
import { leaveGroupConversation, type GroupConversationData } from "@/api"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from "@/components/ui/alert-dialog"

/** 普通成员确认退出，成功后由外层返回消息列表。 */
export function MobileGroupLeaveDialog({
  group,
  busy,
  trigger,
  onClose,
  onSave,
}: {
  group: GroupConversationData
  busy: boolean
  trigger: HTMLElement | null
  onClose: () => void
  onSave: (action: () => Promise<unknown>, change: "leave") => Promise<boolean>
}) {
  const { t } = useTranslation("inbox")
  const { t: tm } = useTranslation("mobile")
  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose()
      }}
    >
      <AlertDialogContent
        onCloseAutoFocus={(event) => {
          event.preventDefault()
          trigger?.focus({ preventScroll: true })
        }}
      >
        <AlertDialogHeader>
          <AlertDialogTitle>{t("groupLeaveTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("groupLeaveDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel className="min-h-11" disabled={busy}>
            {tm("cancel")}
          </AlertDialogCancel>
          <Button
            className="min-h-11"
            variant="destructive"
            disabled={busy}
            onClick={async () => {
              const success = await onSave(
                () =>
                  leaveGroupConversation(group.id, { successorIdentityId: "" }),
                "leave",
              )
              if (success) onClose()
            }}
          >
            {t("groupLeaveConfirm")}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
