/** 助理记忆页签：列表、编辑弹窗与删除确认。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { BookmarkIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  deleteAssistantMemory,
  isApiError,
  listAssistantMemories,
  updateAssistantMemory,
  type AssistantMemory,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { ResourceContent } from "@/components/resource-content"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 弹窗编辑对象，session 标识本次打开的弹窗。 */
type EditingMemory = { memory: AssistantMemory; session: number } | null

/** 读取助理的记忆，按最近更新列出并承载编辑与删除。 */
export function AssistantMemoryPanel({ assistantId }: { assistantId: string }) {
  const { t } = useTranslation(["contacts", "common"])
  const { formatDateTime } = useDateTime()
  // 记忆由后台提取任务写入，每次打开页签或回到窗口时重新读取。
  const memories = useResource(
    resourceKeys.assistantMemories(assistantId),
    () => listAssistantMemories(assistantId),
    { staleTime: 0, refetchOnWindowFocus: true },
  )
  const invalidate = useResourceInvalidator()
  const [editing, setEditing] = useState<EditingMemory>(null)
  const editorSession = useRef(0)

  /** 打开编辑弹窗，并开始新的弹窗会话。 */
  function openEditor(memory: AssistantMemory) {
    editorSession.current += 1
    setEditing({ memory, session: editorSession.current })
  }
  const deletion = useConfirmedAction<AssistantMemory>({
    action: (memory) => deleteAssistantMemory(assistantId, memory.id),
    invalidateKeys: () => [resourceKeys.assistantMemories(assistantId)],
    successMessage: () => t("assistants.memory.deleted"),
    errorMessage: () => t("assistants.memory.deleteError"),
    logLabel: "删除助理记忆",
  })

  return (
    <ResourceContent
      resources={[memories]}
      errorMessage={t("assistants.memory.loadError")}
    >
      <ResourceTable
        columns={[
          {
            key: "name",
            header: t("assistants.memory.form.name"),
            cell: (memory) => (
              <ResourceRowIdentity
                icon={BookmarkIcon}
                name={memory.name}
                description={memory.description}
              />
            ),
          },
          {
            key: "updatedAt",
            header: t("assistants.memory.updatedAtColumn"),
            cellClassName: "w-px whitespace-nowrap text-right text-muted-foreground tabular-nums",
            cell: (memory) =>
              t("assistants.memory.updatedAt", { time: formatDateTime(memory.updatedAt) }),
          },
        ]}
        rows={memories.data?.memories ?? []}
        rowKey={(memory) => memory.id}
        empty={t("assistants.memory.empty")}
        onRowActivate={(memory) => openEditor(memory)}
        rowActions={(memory) => [
          {
            key: "edit",
            label: t("common:actions.edit"),
            onSelect: () => openEditor(memory),
          },
          {
            key: "delete",
            label: t("common:actions.delete"),
            destructive: true,
            separatorBefore: true,
            onSelect: () => deletion.select(memory),
          },
        ]}
      />

      <Dialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>{t("assistants.memory.edit")}</DialogTitle>
            <DialogDescription>
              {t("assistants.memory.editDescription")}
            </DialogDescription>
          </DialogHeader>
          {editing !== null ? (
            <AssistantMemoryForm
              assistantId={assistantId}
              memory={editing.memory}
              onSaved={() => {
                void invalidate(resourceKeys.assistantMemories(assistantId))
                // 保存期间弹窗已关闭并重新打开时，保留新弹窗的编辑内容。
                if (editing.session === editorSession.current) setEditing(null)
              }}
              onCancel={() => setEditing(null)}
            />
          ) : null}
        </DialogContent>
      </Dialog>

      <ConfirmationDialog
        {...deletion.dialog}
        title={t("assistants.memory.deleteTitle", {
          name: deletion.item?.name ?? "",
        })}
        description={t("assistants.memory.deleteDescription")}
        pendingLabel={t("common:actions.deleting")}
      />
    </ResourceContent>
  )
}

/** 助理记忆表单校验规则。 */
function createAssistantMemorySchema(messages: {
  nameRequired: string
  nameTooLong: string
  descriptionRequired: string
  descriptionTooLong: string
  bodyRequired: string
  bodyTooLong: string
}) {
  return z.object({
    name: z.string().trim().min(1, messages.nameRequired).max(60, messages.nameTooLong),
    description: z.string().trim().min(1, messages.descriptionRequired).max(200, messages.descriptionTooLong),
    body: z.string().trim().min(1, messages.bodyRequired).max(2000, messages.bodyTooLong),
  })
}

type AssistantMemoryFormValues = z.infer<
  ReturnType<typeof createAssistantMemorySchema>
>

/** 保存助理记忆的名称、说明与内容。 */
function AssistantMemoryForm({
  assistantId,
  memory,
  onSaved,
  onCancel,
}: {
  assistantId: string
  memory: AssistantMemory
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAssistantMemorySchema({
        nameRequired: t("assistants.memory.validation.nameRequired"),
        nameTooLong: t("assistants.memory.validation.nameTooLong"),
        descriptionRequired: t("assistants.memory.validation.descriptionRequired"),
        descriptionTooLong: t("assistants.memory.validation.descriptionTooLong"),
        bodyRequired: t("assistants.memory.validation.bodyRequired"),
        bodyTooLong: t("assistants.memory.validation.bodyTooLong"),
      }),
    [t],
  )
  const form = useForm<AssistantMemoryFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: memory.name,
      description: memory.description,
      body: memory.body,
    },
  })
  useEffect(() => {
    form.setFocus("body")
  }, [form])

  /** 提交记忆的修改。 */
  async function submit(values: AssistantMemoryFormValues) {
    try {
      await updateAssistantMemory(assistantId, memory.id, values)
      toast.success(t("assistants.memory.saved"))
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存助理记忆失败", { assistant_id: assistantId, memory_id: memory.id, error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "description", "body"])
          : t("assistants.memory.saveError"),
      )
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="name"
          control={form.control}
          label={t("assistants.memory.form.name")}
          required
        />
        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("assistants.memory.form.description")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldDescription>
                {t("assistants.memory.form.descriptionHelp")}
              </FieldDescription>
            </Field>
          )}
        />
        <Controller
          name="body"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("assistants.memory.form.body")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
            </Field>
          )}
        />
      </FieldGroup>
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}
