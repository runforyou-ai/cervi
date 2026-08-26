/** 创建和编辑知识库两级分组的弹窗。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createKnowledgeGroup,
  isApiError,
  type KnowledgeBaseData,
  type KnowledgeGroupData,
  updateKnowledgeGroup,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupNameMaxLength = 120

export type KnowledgeGroupDialogState = {
  knowledgeBase: KnowledgeBaseData
  group?: KnowledgeGroupData
  parentId?: string
}

/** 编辑知识库一级或二级分组。 */
export function KnowledgeGroupDialog({
  state,
  onOpenChange,
  onSaved,
}: {
  state: KnowledgeGroupDialogState | null
  onOpenChange: (open: boolean) => void
  onSaved: (knowledgeBase: KnowledgeBaseData) => void
}) {
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      z.object({
        name: z
          .string()
          .trim()
          .min(1, t("validation.groupNameRequired"))
          .max(groupNameMaxLength, t("validation.groupNameTooLong")),
        parentId: z.string(),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: "", parentId: "" },
  })

  useEffect(() => {
    mounted.current = true
    if (state) {
      form.reset({
        name: state.group?.name ?? "",
        parentId: state.group?.parentId ?? state.parentId ?? "",
      })
    }
    return () => {
      mounted.current = false
    }
  }, [form, state])

  /** 保存分组并同步窄侧栏。 */
  async function save(values: z.infer<typeof schema>) {
    if (!state) return
    try {
      const knowledgeBase = state.group
        ? await updateKnowledgeGroup(
            state.knowledgeBase.id,
            state.group.id,
            values,
          )
        : await createKnowledgeGroup(state.knowledgeBase.id, values)
      if (!mounted.current) return
      onSaved(knowledgeBase)
      onOpenChange(false)
      toast.success(
        state.group
          ? t("group.updateSuccess")
          : t("group.createSuccess"),
      )
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) return
      console.warn("知识库分组保存失败", {
        knowledge_base_id: state.knowledgeBase.id,
        group_id: state.group?.id,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["name", "parentId"])
          : t("group.saveError"),
      )
    }
  }

  const parentLocked = Boolean(state?.parentId && !state.group)
  const topLevelGroups =
    state?.knowledgeBase.groups.filter(
      (group) => !group.isDefault && group.id !== state.group?.id,
    ) ?? []

  return (
    <Dialog open={state !== null} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>
            {state?.group ? t("group.editTitle") : t("group.createTitle")}
          </DialogTitle>
        </DialogHeader>
        <form
          className="grid gap-9"
          onSubmit={form.handleSubmit(save)}
          noValidate
        >
          <FieldGroup>
            <FormInputField
              name="name"
              control={form.control}
              label={t("group.name")}
              autoFocus
              maxLength={groupNameMaxLength}
            />
            <Controller
              name="parentId"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor="knowledge-group-parent" required={false}>
                    {t("group.parent")}
                  </FieldLabel>
                  <NativeSelect
                    {...field}
                    id="knowledge-group-parent"
                    disabled={parentLocked}
                    aria-invalid={fieldState.invalid}
                  >
                    <option value="">{t("group.topLevel")}</option>
                    {topLevelGroups.map((group) => (
                      <option key={group.id} value={group.id}>
                        {group.name}
                      </option>
                    ))}
                  </NativeSelect>
                </Field>
              )}
            />
          </FieldGroup>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={form.formState.isSubmitting}
              onClick={() => onOpenChange(false)}
            >
              {t("group.cancel")}
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting
                ? t("group.saving")
                : t("group.save")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
