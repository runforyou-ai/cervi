/** 本地问答的删除确认和缓存刷新。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import {
  deleteKnowledgeQAEntry,
  isApiError,
  type KnowledgeQASummaryData,
} from "@/api"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 确认删除整条问答，失败时保留确认窗口。 */
export function KnowledgeQADeleteDialog({
  knowledgeBaseId,
  entry,
  onClose,
}: {
  knowledgeBaseId: string
  entry: KnowledgeQASummaryData | null
  onClose: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  /** 删除问答后刷新同一知识库的列表和详情。 */
  async function confirmDelete() {
    if (!entry || busy) return
    setBusy(true)
    try {
      await deleteKnowledgeQAEntry(knowledgeBaseId, entry.id)
      await Promise.all([
        invalidate(resourceKeys.knowledgeQAEntries(knowledgeBaseId)),
        invalidate(resourceKeys.knowledgeQAEntry(knowledgeBaseId, entry.id)),
      ])
      if (!mounted.current) return
      onClose()
      toast.success(t("qa.deleteSuccess"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("qa.deleteError"),
      )
    } finally {
      if (mounted.current) setBusy(false)
    }
  }

  return (
    <AlertDialog
      open={entry !== null}
      onOpenChange={(open) => {
        if (!open && !busy) onClose()
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("qa.deleteTitle")}</AlertDialogTitle>
          <AlertDialogDescription className="whitespace-pre-wrap break-words">
            {t("qa.deleteDescription", {
              question: entry?.question ?? "",
            })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>
            {t("qa.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            disabled={busy}
            onClick={(event) => {
              event.preventDefault()
              void confirmDelete()
            }}
          >
            {busy ? t("qa.deleting") : t("qa.delete")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
