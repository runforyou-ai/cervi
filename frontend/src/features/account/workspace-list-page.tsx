/** 工作区列表页：进入已加入的工作区、创建工作区，部署管理员可修改注册开关。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ChevronRightIcon, LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  createWorkspace,
  getDeploymentSettings,
  isApiError,
  listWorkspaces,
  loadAccount,
  logout,
  updateDeploymentSettings,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import { useStartup } from "@/contexts/startup-context"
import {
  createWorkspaceSchema,
  suggestWorkspaceSlug,
  type WorkspaceFormValues,
} from "@/features/account/workspace-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { enterWorkspace } from "@/lib/workspace-route"

/** 展示账号可进入的工作区、创建入口和部署设置。 */
export function WorkspaceListPage() {
  const { t } = useTranslation("account")
  const navigate = useNavigate()
  const { usesOfficialLogin } = useStartup()
  const account = useResource(resourceKeys.account(), (signal) => loadAccount(signal))
  const workspaces = useResource(resourceKeys.workspaces(), (signal) => listWorkspaces(signal), { staleTime: 0 })
  const [loggingOut, setLoggingOut] = useState(false)

  /** 退出登录后回到登录页。 */
  async function signOut() {
    setLoggingOut(true)
    navigate("/login", { replace: true })
    try {
      await logout()
    } catch (error) {
      console.warn("退出登录失败", error)
    }
  }

  if (!account.data || !workspaces.data) {
    return (
      <main className="flex min-h-dvh items-center justify-center px-6 text-center text-sm text-muted-foreground">
        {account.error || workspaces.error ? t("loadError") : <LoadingIndicator><span className="sr-only">Loading</span></LoadingIndicator>}
      </main>
    )
  }

  return (
    <main className="flex min-h-dvh w-full justify-center px-6 pt-[max(2.5rem,env(safe-area-inset-top))] pb-[max(2.5rem,env(safe-area-inset-bottom))] md:py-16">
      <div className="w-full max-w-md space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t("title")}</CardTitle>
            <CardDescription>{t("description")}</CardDescription>
          </CardHeader>
          <CardContent>
            {workspaces.data.items.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("empty")}</p>
            ) : (
              <ul className="-mx-2">
                {workspaces.data.items.map((workspace) => (
                  <li key={workspace.id}>
                    <button
                      type="button"
                      className="flex w-full items-center gap-3 rounded-md px-2 py-2.5 text-left transition-colors hover:bg-muted"
                      onClick={() => enterWorkspace(workspace.slug)}
                    >
                      <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-sm font-medium text-primary">
                        {workspace.name.slice(0, 1)}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium">{workspace.name}</span>
                        <span className="block truncate text-xs text-muted-foreground">{workspace.slug}</span>
                      </span>
                      <ChevronRightIcon className="size-4 text-muted-foreground" aria-label={t("enter")} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>
        <CreateWorkspaceCard />
        {account.data.isDeploymentAdmin && !usesOfficialLogin ? <DeploymentSettingsCard /> : null}
        <div className="flex items-center justify-between gap-4 text-sm text-muted-foreground">
          <span className="min-w-0 truncate">{t("signedInAs", { email: account.data.email })}</span>
          <Button type="button" variant="ghost" size="sm" disabled={loggingOut} onClick={() => void signOut()}>
            {t("logout")}
          </Button>
        </div>
      </div>
    </main>
  )
}

/** 创建工作区，创建成功后进入新工作区。 */
function CreateWorkspaceCard() {
  const { t } = useTranslation("account")
  const navigate = useNavigate()
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
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["name", "slug"]))
        return
      }
      console.warn("创建工作区失败", error)
    }
  }

  const { isSubmitting } = form.formState

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("createTitle")}</CardTitle>
        <CardDescription>{t("createDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={form.handleSubmit(submitWorkspace)} noValidate>
          <FieldGroup>
            <FormInputField name="name" control={form.control} label={t("nameLabel")} />
            <FormInputField name="slug" control={form.control} label={t("slugLabel")} autoCapitalize="none" autoCorrect="off" />
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
              {isSubmitting ? t("creating") : t("create")}
            </Button>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}

/** 部署管理员修改注册开关，切换后立即保存。 */
function DeploymentSettingsCard() {
  const { t } = useTranslation("account")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const settings = useResource(resourceKeys.deploymentSettings(), (signal) => getDeploymentSettings(signal))
  const [saving, setSaving] = useState(false)

  /** 保存注册开关。 */
  async function saveRegistration(registrationOpen: boolean) {
    setSaving(true)
    try {
      await updateDeploymentSettings({ registrationOpen })
      await invalidate(resourceKeys.deploymentSettings())
      void invalidate(resourceKeys.installationStatus())
      toast.success(t("deploymentSaved"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isApiError(error)) toast.error(apiErrorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("deploymentTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        <Field orientation="horizontal">
          <FieldContent>
            <FieldLabel htmlFor="registration-open">{t("registrationOpen")}</FieldLabel>
            <FieldDescription>{t("registrationOpenHelp")}</FieldDescription>
          </FieldContent>
          <Switch
            id="registration-open"
            checked={settings.data?.registrationOpen ?? false}
            disabled={!settings.data || saving}
            onCheckedChange={(checked) => void saveRegistration(checked)}
          />
        </Field>
      </CardContent>
    </Card>
  )
}
