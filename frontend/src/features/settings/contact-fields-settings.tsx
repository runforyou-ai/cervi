/** 企业联系人字段：列表、新增编辑弹窗与删除确认。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ListIcon, PlusIcon, XIcon } from "lucide-react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  ContactFieldType,
  createContactField,
  deleteContactField,
  isApiError,
  listContactFields,
  updateContactField,
  type ContactFieldData,
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
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 弹窗编辑对象：field 为空表示新增字段，session 标识本次打开的弹窗。 */
type EditingField = { field?: ContactFieldData; session: number } | null

const fieldTypes = [
  ContactFieldType.ContactFieldTypeText,
  ContactFieldType.ContactFieldTypeNumber,
  ContactFieldType.ContactFieldTypeDate,
  ContactFieldType.ContactFieldTypeSelect,
] as const

/** 读取联系人字段，展示字段列表并承载新增、编辑和删除。 */
export function ContactFieldsSettings() {
  const { t } = useTranslation(["settings", "common"])
  const fields = useResource(resourceKeys.contactFields(), () =>
    listContactFields(),
  )
  const invalidate = useResourceInvalidator()
  const [editing, setEditing] = useState<EditingField>(null)
  const editorSession = useRef(0)

  /** 打开新增或编辑弹窗，并开始新的弹窗会话。 */
  function openEditor(field?: ContactFieldData) {
    editorSession.current += 1
    setEditing({ field, session: editorSession.current })
  }
  const deletion = useConfirmedAction<ContactFieldData>({
    action: (field) => deleteContactField(field.id),
    invalidateKeys: () => [
      resourceKeys.contactFields(),
      resourceKeys.contact(),
    ],
    successMessage: () => t("customerService.contactFields.deleted"),
    errorMessage: () => t("customerService.contactFields.deleteError"),
    logLabel: "删除联系人字段",
  })

  return (
    <ResourceContent
      resources={[fields]}
      errorMessage={t("customerService.contactFields.loadError")}
    >
      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-sm font-medium">
              {t("customerService.contactFields.title")}
            </h3>
            <p className="text-sm text-muted-foreground">
              {t("customerService.contactFields.description")}
            </p>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0"
            aria-label={t("customerService.contactFields.create")}
            title={t("customerService.contactFields.create")}
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
              header: t("customerService.contactFields.form.name"),
              cell: (field) => (
                <ResourceRowIdentity
                  icon={ListIcon}
                  name={field.name}
                  secondary={t(`customerService.contactFields.types.${field.type}`)}
                  description={
                    [
                      field.type === ContactFieldType.ContactFieldTypeSelect
                        ? field.options.map((option) => option.name).join("、")
                        : "",
                      field.aiInstruction
                        ? t("customerService.contactFields.aiFilled")
                        : "",
                    ]
                      .filter(Boolean)
                      .join(" · ") || undefined
                  }
                />
              ),
            },
          ]}
          rows={fields.data?.fields ?? []}
          rowKey={(field) => field.id}
          empty={t("customerService.contactFields.empty")}
          onRowActivate={(field) => openEditor(field)}
          rowActions={(field) => [
            {
              key: "edit",
              label: t("common:actions.edit"),
              onSelect: () => openEditor(field),
            },
            {
              key: "delete",
              label: t("common:actions.delete"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => deletion.select(field),
            },
          ]}
        />
      </section>

      <Dialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing?.field
                ? t("customerService.contactFields.edit")
                : t("customerService.contactFields.create")}
            </DialogTitle>
          </DialogHeader>
          {editing !== null ? (
            <ContactFieldForm
              field={editing.field}
              onSaved={() => {
                void invalidate(resourceKeys.contactFields())
                void invalidate(resourceKeys.contact())
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
        title={t("customerService.contactFields.deleteTitle", {
          name: deletion.item?.name ?? "",
        })}
        description={t("customerService.contactFields.deleteDescription")}
        pendingLabel={t("common:actions.deleting")}
      />
    </ResourceContent>
  )
}

/** 创建联系人字段表单校验规则；单选字段至少一个选项。 */
function createContactFieldSchema(messages: {
  nameRequired: string
  nameTooLong: string
  optionRequired: string
  optionTooLong: string
  aiInstructionTooLong: string
}) {
  return z.object({
    name: z.string().trim().min(1, messages.nameRequired).max(50, messages.nameTooLong),
    type: z.enum(fieldTypes),
    options: z.array(
      z.object({
        optionId: z.string(),
        name: z.string().trim().min(1, messages.optionRequired).max(50, messages.optionTooLong),
      }),
    ),
    aiInstruction: z.string().trim().max(500, messages.aiInstructionTooLong),
  })
}

type ContactFieldFormValues = z.infer<
  ReturnType<typeof createContactFieldSchema>
>

/** 保存新联系人字段或现有字段；类型创建后只读，单选字段可增删选项，可设置 AI 填写说明。 */
function ContactFieldForm({
  field,
  onSaved,
  onCancel,
}: {
  field?: ContactFieldData
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createContactFieldSchema({
        nameRequired: t("customerService.contactFields.validation.nameRequired"),
        nameTooLong: t("customerService.contactFields.validation.nameTooLong"),
        optionRequired: t("customerService.contactFields.validation.optionRequired"),
        optionTooLong: t("customerService.contactFields.validation.optionTooLong"),
        aiInstructionTooLong: t("customerService.contactFields.validation.aiInstructionTooLong"),
      }),
    [t],
  )
  const form = useForm<ContactFieldFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: field?.name ?? "",
      type: field?.type ?? ContactFieldType.ContactFieldTypeText,
      options: (field?.options ?? []).map((option) => ({
        optionId: option.id,
        name: option.name,
      })),
      aiInstruction: field?.aiInstruction ?? "",
    },
  })
  const options = useFieldArray({ control: form.control, name: "options" })
  const type = form.watch("type")
  useEffect(() => {
    if (!field) form.setFocus("name")
  }, [field, form])

  /** 提交联系人字段表单，非单选字段不提交选项。 */
  async function submit(values: ContactFieldFormValues) {
    const input = {
      name: values.name,
      type: values.type,
      options:
        values.type === ContactFieldType.ContactFieldTypeSelect
          ? values.options.map((option) => ({
              id: option.optionId,
              name: option.name,
            }))
          : [],
      aiInstruction: values.aiInstruction,
    }
    try {
      if (field) {
        await updateContactField(field.id, input)
      } else {
        await createContactField(input)
      }
      toast.success(
        t(
          field
            ? "customerService.contactFields.updated"
            : "customerService.contactFields.created",
        ),
      )
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存联系人字段失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "type", "options", "aiInstruction"])
          : t("customerService.contactFields.saveError"),
      )
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="name"
          control={form.control}
          label={t("customerService.contactFields.form.name")}
          required
        />
        <Controller
          name="type"
          control={form.control}
          render={({ field: typeField }) => (
            <Field>
              <FieldLabel htmlFor={typeField.name} required>
                {t("customerService.contactFields.form.type")}
              </FieldLabel>
              <NativeSelect
                {...typeField}
                id={typeField.name}
                disabled={Boolean(field)}
                onChange={(event) => {
                  typeField.onChange(event.target.value)
                  // 切换为单选时提供第一个空选项，切换为其他类型时清空选项。
                  if (event.target.value === ContactFieldType.ContactFieldTypeSelect) {
                    if (options.fields.length === 0) {
                      options.append({ optionId: "", name: "" })
                    }
                  } else {
                    options.replace([])
                  }
                }}
              >
                {fieldTypes.map((value) => (
                  <option key={value} value={value}>
                    {t(`customerService.contactFields.types.${value}`)}
                  </option>
                ))}
              </NativeSelect>
              {field ? (
                <FieldDescription>
                  {t("customerService.contactFields.form.typeLocked")}
                </FieldDescription>
              ) : null}
            </Field>
          )}
        />
        {type === ContactFieldType.ContactFieldTypeSelect ? (
          <Field>
            <FieldLabel required>
              {t("customerService.contactFields.form.options")}
            </FieldLabel>
            <div className="grid gap-2">
              {options.fields.map((option, index) => (
                <div key={option.id} className="flex items-center gap-2">
                  <Input
                    {...form.register(`options.${index}.name`)}
                    aria-label={t("customerService.contactFields.form.optionName", {
                      index: index + 1,
                    })}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="shrink-0"
                    disabled={options.fields.length === 1}
                    aria-label={t("customerService.contactFields.form.removeOption")}
                    title={t("customerService.contactFields.form.removeOption")}
                    onClick={() => options.remove(index)}
                  >
                    <XIcon />
                  </Button>
                </div>
              ))}
              <div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="-ml-2"
                  onClick={() => options.append({ optionId: "", name: "" })}
                >
                  <PlusIcon />
                  {t("customerService.contactFields.form.addOption")}
                </Button>
              </div>
            </div>
            {field ? (
              <FieldDescription>
                {t("customerService.contactFields.form.optionsHelp")}
              </FieldDescription>
            ) : null}
          </Field>
        ) : null}
        <Controller
          name="aiInstruction"
          control={form.control}
          render={({ field: instructionField, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={instructionField.name}>
                {t("customerService.contactFields.form.aiInstruction")}
              </FieldLabel>
              <AutoGrowTextarea
                {...instructionField}
                id={instructionField.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldDescription>
                {t("customerService.contactFields.form.aiInstructionHelp")}
              </FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}
