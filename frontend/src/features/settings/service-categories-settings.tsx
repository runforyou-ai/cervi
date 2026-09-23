/** 企业咨询分类目录：列表、新增编辑弹窗与删除确认。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { PlusIcon, TagIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createServiceCategory,
  deleteServiceCategory,
  isApiError,
  listAllTeams,
  listServiceCategories,
  updateServiceCategory,
  type ServiceCategory,
  type Team,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { ResourceContent } from "@/components/resource-content"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
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
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 弹窗编辑对象：category 为空表示新增分类，session 标识本次打开的弹窗。 */
type EditingCategory = { category?: ServiceCategory; session: number } | null

/** 读取咨询分类与团队，展示分类列表并承载新增、编辑和删除。 */
export function ServiceCategoriesSettings() {
  const { t } = useTranslation(["settings", "common"])
  const categories = useResource(resourceKeys.serviceCategories(), () =>
    listServiceCategories(),
  )
  const teams = useResource(resourceKeys.teams({ all: true }), () =>
    listAllTeams(),
  )
  const invalidate = useResourceInvalidator()
  const [editing, setEditing] = useState<EditingCategory>(null)
  const editorSession = useRef(0)

  /** 打开新增或编辑弹窗，并开始新的弹窗会话。 */
  function openEditor(category?: ServiceCategory) {
    editorSession.current += 1
    setEditing({ category, session: editorSession.current })
  }
  const deletion = useConfirmedAction<ServiceCategory>({
    action: (category) => deleteServiceCategory(category.id),
    invalidateKeys: () => [resourceKeys.serviceCategories()],
    successMessage: () => t("customerService.categories.deleted"),
    errorMessage: () => t("customerService.categories.deleteError"),
    logLabel: "删除咨询分类",
  })

  return (
    <ResourceContent
      resources={[categories, teams]}
      errorMessage={t("customerService.categories.loadError")}
    >
      <div className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            {t("customerService.categories.description")}
          </p>
          <Button
            variant="ghost"
            size="icon-sm"
            className="shrink-0"
            aria-label={t("customerService.categories.create")}
            title={t("customerService.categories.create")}
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
              header: t("customerService.categories.form.name"),
              cell: (category) => (
                <ResourceRowIdentity
                  icon={TagIcon}
                  name={category.name}
                  secondary={
                    category.team?.name ??
                    t("customerService.categories.channelFallback")
                  }
                  description={category.description || undefined}
                />
              ),
            },
          ]}
          rows={categories.data?.categories ?? []}
          rowKey={(category) => category.id}
          empty={t("customerService.categories.empty")}
          onRowActivate={(category) => openEditor(category)}
          rowActions={(category) => [
            {
              key: "edit",
              label: t("common:actions.edit"),
              onSelect: () => openEditor(category),
            },
            {
              key: "delete",
              label: t("common:actions.delete"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => deletion.select(category),
            },
          ]}
        />
      </div>

      <Dialog
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
      >
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing?.category
                ? t("customerService.categories.edit")
                : t("customerService.categories.create")}
            </DialogTitle>
            <DialogDescription>
              {t("customerService.categories.formDescription")}
            </DialogDescription>
          </DialogHeader>
          {editing !== null ? (
            <ServiceCategoryForm
              category={editing.category}
              teams={teams.data ?? []}
              onSaved={() => {
                void invalidate(resourceKeys.serviceCategories())
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
        title={t("customerService.categories.deleteTitle", {
          name: deletion.item?.name ?? "",
        })}
        description={t("customerService.categories.deleteDescription")}
        pendingLabel={t("common:actions.deleting")}
      />
    </ResourceContent>
  )
}

/** 创建咨询分类表单校验规则。 */
function createServiceCategorySchema(messages: {
  nameRequired: string
  nameTooLong: string
  descriptionTooLong: string
}) {
  return z.object({
    name: z.string().trim().min(1, messages.nameRequired).max(64, messages.nameTooLong),
    description: z.string().trim().max(500, messages.descriptionTooLong),
    teamId: z.string(),
  })
}

type ServiceCategoryFormValues = z.infer<
  ReturnType<typeof createServiceCategorySchema>
>

/** 保存新咨询分类或现有咨询分类。 */
function ServiceCategoryForm({
  category,
  teams,
  onSaved,
  onCancel,
}: {
  category?: ServiceCategory
  teams: Team[]
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createServiceCategorySchema({
        nameRequired: t("customerService.categories.validation.nameRequired"),
        nameTooLong: t("customerService.categories.validation.nameTooLong"),
        descriptionTooLong: t(
          "customerService.categories.validation.descriptionTooLong",
        ),
      }),
    [t],
  )
  const form = useForm<ServiceCategoryFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: category?.name ?? "",
      description: category?.description ?? "",
      teamId: category?.team?.id ?? "",
    },
  })
  useEffect(() => {
    if (!category) form.setFocus("name")
  }, [category, form])

  /** 提交咨询分类表单，团队为空表示按渠道失败路由。 */
  async function submit(values: ServiceCategoryFormValues) {
    const input = {
      name: values.name,
      description: values.description,
      teamId: values.teamId || null,
    }
    try {
      if (category) {
        await updateServiceCategory(category.id, input)
      } else {
        await createServiceCategory(input)
      }
      toast.success(
        t(
          category
            ? "customerService.categories.updated"
            : "customerService.categories.created",
        ),
      )
      onSaved()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("保存咨询分类失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "description", "teamId"])
          : t("customerService.categories.saveError"),
      )
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField
          name="name"
          control={form.control}
          label={t("customerService.categories.form.name")}
          required
        />
        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("customerService.categories.form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldDescription>
                {t("customerService.categories.form.descriptionHelp")}
              </FieldDescription>
            </Field>
          )}
        />
        <Controller
          name="teamId"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor={field.name}>
                {t("customerService.categories.form.team")}
              </FieldLabel>
              <NativeSelect {...field} id={field.name}>
                <option value="">
                  {t("customerService.categories.channelFallback")}
                </option>
                {teams.map((team) => (
                  <option key={team.id} value={team.id}>
                    {team.name}
                  </option>
                ))}
              </NativeSelect>
              <FieldDescription>
                {t("customerService.categories.form.teamHelp")}
              </FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}
