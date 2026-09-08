/** 文档移动分组和删除确认，完成后刷新列表与详情。 */
import { useEffect, useMemo, useRef } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import {
  deleteKnowledgeDocument,
  moveKnowledgeDocument,
  isApiError,
  type KnowledgeDocument,
  type KnowledgeBaseData,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { Field, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"

export type DocumentAction = { document: KnowledgeDocument; kind: "move" | "delete"; trigger: HTMLButtonElement | null }

/** 在列表上完成移动或确认删除，失败时保留原操作。 */
export function KnowledgeDocumentActions({
  base,
  action,
  onClose,
}: {
  base: KnowledgeBaseData
  action: DocumentAction
  onClose: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const schema = useMemo(() => z.object({ groupId: z.string().min(1, t("documents.groupRequired")) }), [t])
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { groupId: action.document.groupId },
  })
  const busy = form.formState.isSubmitting
  const groups = base.groups.flatMap((group) => [group, ...group.children])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 提交文档操作并失效对应列表、详情和预览。 */
  async function save(values: z.infer<typeof schema>) {
    try {
      if (action.kind === "move") await moveKnowledgeDocument(base.id, action.document.id, values)
      else await deleteKnowledgeDocument(base.id, action.document.id)
      await Promise.all([
        invalidate(resourceKeys.knowledgeDocuments(base.id)),
        invalidate(resourceKeys.knowledgeDocument(base.id, action.document.id)),
        invalidate(resourceKeys.knowledgeDocumentFile(base.id, action.document.id)),
      ])
      if (!mounted.current) return
      toast.success(t(action.kind === "move" ? "documents.moveSuccess" : "documents.deleteSuccess"))
      onClose()
    } catch (error) {
      if (mounted.current && !recoverSession(error, navigate))
        toast.error(isApiError(error) ? apiErrorMessage(error) : t("documents.operationFailed"))
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose()
      }}
    >
      <DialogContent
        closeDisabled={busy}
        onCloseAutoFocus={(event) => {
          event.preventDefault()
          action.trigger?.focus()
        }}
      >
        <DialogHeader>
          <DialogTitle>{t(action.kind === "move" ? "documents.move" : "documents.delete")}</DialogTitle>
          <DialogDescription>
            {action.kind === "delete"
              ? t("documents.deleteDescription", { name: action.document.name })
              : action.document.name}
          </DialogDescription>
        </DialogHeader>
        <form className="space-y-9" onSubmit={form.handleSubmit(save)}>
          {action.kind === "move" && (
            <Field>
              <FieldLabel htmlFor="document-target-group" required>
                {t("documents.group")}
              </FieldLabel>
              <NativeSelect id="document-target-group" {...form.register("groupId")} required disabled={busy}>
                {groups.map((group) => (
                  <option key={group.id} value={group.id} disabled={group.id === action.document.groupId}>
                    {group.isDefault
                      ? t("group.default")
                      : group.parentId
                        ? `${groups.find((parent) => parent.id === group.parentId)?.name} / ${group.name}`
                        : group.name}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          )}
          <div className="flex justify-end gap-3">
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              {t("documents.cancel")}
            </Button>
            <Button
              type="submit"
              variant={action.kind === "delete" ? "destructive" : "default"}
              disabled={busy || (action.kind === "move" && form.watch("groupId") === action.document.groupId)}
            >
              {t(busy ? "documents.saving" : action.kind === "delete" ? "documents.delete" : "documents.move")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
