/** 企业联系人标签：列表、新增编辑弹窗与删除确认。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { PlusIcon, TagsIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createContactTag,
  deleteContactTag,
  isApiError,
  listContactTags,
  updateContactTag,
  type ContactTag,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { AutoGrowTextarea } from "@/components/form/auto-grow-textarea"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { ResourceContent } from "@/components/resource-content"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 弹窗编辑对象：tag 为空表示新增标签，session 标识本次打开的弹窗。 */
type EditingTag = { tag?: ContactTag; session: number } | null

/** 读取联系人标签，展示标签列表并承载新增、编辑和删除。 */
export function ContactTagsSettings() {
  const { t } = useTranslation(["settings", "common"])
  const tags = useResource(resourceKeys.contactTags(), () => listContactTags())
  const invalidate = useResourceInvalidator()
  const [editing, setEditing] = useState<EditingTag>(null)
  const editorSession = useRef(0)

  /** 打开新增或编辑弹窗，并开始新的弹窗会话。 */
  function openEditor(tag?: ContactTag) {
    editorSession.current += 1
    setEditing({ tag, session: editorSession.current })
  }
  const deletion = useConfirmedAction<ContactTag>({
    action: (tag) => deleteContactTag(tag.id),
    invalidateKeys: () => [
      resourceKeys.contactTags(),
      resourceKeys.contact(),
      resourceKeys.contacts(),
    ],
    successMessage: () => t("customerService.contactTags.deleted"),
    errorMessage: () => t("customerService.contactTags.deleteError"),
    logLabel: "删除联系人标签",
  })

  return (
    <ResourceContent
      resources={[tags]}
      errorMessage={t("customerService.contactTags.loadError")}
    >
      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-sm font-medium">
              {t("customerService.contactTags.title")}
            </h3>
            <p className="text-sm text-muted-foreground">
              {t("customerService.contactTags.description")}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0"
            aria-label={t("customerService.contactTags.create")}
            title={t("customerService.contactTags.create")}
            onClick={() => openEditor()}
          >
            <PlusIcon />
          </Button>
        </div>
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "name",
              header: t("customerService.contactTags.form.name"),
              cell: (tag) => (
                <ResourceRowIdentity
                  icon={TagsIcon}
                  name={tag.name}
                  description={
                    tag.aiInstruction
                      ? t("customerService.contactTags.aiCondition", {
                          condition: tag.aiInstruction,
                        })
                      : undefined
                  }
                />
              ),
            },
          ]}
          rows={tags.data?.tags ?? []}
          rowKey={(tag) => tag.id}
          empty={t("customerService.contactTags.empty")}
          onRowActivate={(tag) => openEditor(tag)}
          rowActions={(tag) => [
            {
              key: "edit",
              label: t("common:actions.edit"),
              onSelect: () => openEditor(tag),
            },
            {
              key: "delete",
              label: t("common:actions.delete"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => deletion.select(tag),
            },
          ]}
        />
      </section>

      <Dialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {editing?.tag
                ? t("customerService.contactTags.edit")
                : t("customerService.contactTags.create")}
            </DialogTitle>
          </DialogHeader>
          {editing !== null ? (
            <ContactTagForm
              tag={editing.tag}
              onSaved={() => {
                void invalidate(resourceKeys.contactTags())
                void invalidate(resourceKeys.contact())
                void invalidate(resourceKeys.contacts())
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
        title={t("customerService.contactTags.deleteTitle", {
          name: deletion.item?.name ?? "",
        })}
        description={t("customerService.contactTags.deleteDescription")}
        pendingLabel={t("common:actions.deleting")}
      />
    </ResourceContent>
  )
}

/** 保存新联系人标签或现有标签的名称与 AI 添加条件。 */
function ContactTagForm({
  tag,
  onSaved,
  onCancel,
}: {
  tag?: ContactTag
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      z.object({
        name: z
          .string()
          .trim()
          .min(1, t("customerService.contactTags.validation.nameRequired"))
          .max(30, t("customerService.contactTags.validation.nameTooLong")),
        aiInstruction: z
          .string()
          .trim()
          .max(500, t("customerService.contactTags.validation.aiInstructionTooLong")),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: tag?.name ?? "", aiInstruction: tag?.aiInstruction ?? "" },
  })
  useEffect(() => {
    form.setFocus("name")
  }, [form])

  /** 提交联系人标签表单。 */
  async function submit(values: z.infer<typeof schema>) {
    try {
      if (tag) {
        await updateContactTag(tag.id, values)
      } else {
        await createContactTag(values)
      }
      toast.success(
        t(
          tag
            ? "customerService.contactTags.updated"
            : "customerService.contactTags.created",
        ),
      )
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存联系人标签失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "aiInstruction"])
          : t("customerService.contactTags.saveError"),
      )
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="name"
          control={form.control}
          label={t("customerService.contactTags.form.name")}
          required
        />
        <Controller
          name="aiInstruction"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("customerService.contactTags.form.aiInstruction")}
              </FieldLabel>
              <AutoGrowTextarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldDescription>
                {t("customerService.contactTags.form.aiInstructionHelp")}
              </FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}
