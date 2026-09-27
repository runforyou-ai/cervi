/** 创建工作区页：填写名称和标识，创建后进入新工作区。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowLeftIcon, LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { createWorkspace, isApiError } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { AccountShell } from "@/features/account/account-shell"
import {
  createWorkspaceSchema,
  suggestWorkspaceSlug,
  type WorkspaceFormValues,
} from "@/features/account/workspace-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { resolveServerURL } from "@/lib/server-url"
import { recoverSession } from "@/lib/session-navigation"
import { enterWorkspace } from "@/lib/workspace-route"

/** 校验并创建工作区，标识未手动修改时按名称自动建议。 */
export function WorkspaceCreatePage() {
  const { t } = useTranslation("account")
  const navigate = useNavigate()
  const serverURL = useResource(resourceKeys.serverURL(), () => resolveServerURL())
  const host = serverURL.data ? new URL(serverURL.data).host : ""
  const schema = useMemo(
    () =>
      createWorkspaceSchema({
        nameRequired: t("nameRequired"),
        nameTooLong: t("nameTooLong"),
        slugRequired: t("slugRequired"),
        slugInvalid: t("slugInvalid"),
      }),
    [t],
  )
  const form = useForm<WorkspaceFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: "", slug: "" },
  })

  // 工作区标识未手动修改时按名称自动建议。
  const { getFieldState, setValue, watch } = form
  const name = watch("name")
  const slug = watch("slug").trim().toLowerCase()
  useEffect(() => {
    if (!getFieldState("slug").isDirty) {
      setValue("slug", suggestWorkspaceSlug(name))
    }
  }, [getFieldState, name, setValue])

  /** 提交新建工作区并进入。 */
  async function submitWorkspace(values: WorkspaceFormValues) {
    try {
      const workspace = await createWorkspace(values)
      enterWorkspace(workspace.slug)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error, ["name", "slug"]) : t("createError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <AccountShell
      title={t("createTitle")}
      description={t("createDescription")}
      leading={
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="mb-3 -ml-2 text-muted-foreground"
          aria-label={t("back")}
          title={t("back")}
          onClick={() => navigate("/workspaces")}
        >
          <ArrowLeftIcon />
        </Button>
      }
    >
      <form onSubmit={form.handleSubmit(submitWorkspace)} noValidate>
        <FieldGroup>
          <FormInputField name="name" control={form.control} label={t("nameLabel")} autoFocus />
          <div className="space-y-2">
            <FormInputField name="slug" control={form.control} label={t("slugLabel")} autoCapitalize="none" autoCorrect="off" />
            <p className="truncate text-xs text-muted-foreground">
              {t("addressPreview", { address: `${host}/#/w/${slug || "…"}` })}
            </p>
          </div>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
            {isSubmitting ? t("creating") : t("create")}
          </Button>
        </FieldGroup>
      </form>
    </AccountShell>
  )
}
