/** 企业初始化表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  install,
  loadStartup,
  SessionState,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { FieldGroup } from "@/components/ui/field"
import {
  createSetupSchema,
  type SetupFormValues,
} from "@/features/installation/setup-schema"
import { useStartup } from "@/features/startup/startup-context"
import { apiErrorMessage } from "@/lib/form-errors"

/** 创建企业和第一个管理员账号。 */
export function SetupForm() {
  const { t } = useTranslation("setup")
  const navigate = useNavigate()
  const { completeStartup } = useStartup()
  const schema = useMemo(() => createSetupSchema(t), [t])
  const form = useForm<SetupFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      organizationName: "",
      displayName: "",
      email: "",
      password: "",
    },
  })

  /** 提交企业初始化并进入收件箱。 */
  async function submitSetup(values: SetupFormValues) {
    try {
      const identity = await install(values)
      completeStartup(identity.organization.name)
      navigate("/inbox", { replace: true })
    } catch (error) {
      if (
        isApiError(error) &&
        error.state === SessionState.SessionStateLogin
      ) {
        try {
          const startup = await loadStartup()
          if (
            startup.state === SessionState.SessionStateReady &&
            startup.organizationName
          ) {
            console.info("企业已完成初始化，进入登录页")
            completeStartup(startup.organizationName)
            navigate("/login", { replace: true })
            return
          }
          console.warn("企业初始化错误与启动状态不一致", {
            state: startup.state,
          })
          toast.error(apiErrorMessage(error))
        } catch (startupError) {
          console.warn("同步企业初始化状态失败", startupError)
          toast.error(t("networkError"))
        }
        return
      }
      if (recoverSession(error, navigate)) {
        return
      }
      if (isApiError(error)) {
        toast.error(
          apiErrorMessage(error, [
            "organizationName",
            "displayName",
            "email",
            "password",
          ]),
        )
        return
      }

      toast.error(t("networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <CardDescription>{t("description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={form.handleSubmit(submitSetup)} noValidate>
          <FieldGroup>
            <FormInputField
              name="organizationName"
              control={form.control}
              label={t("organizationNameLabel")}
              autoFocus
            />
            <FormInputField
              name="displayName"
              control={form.control}
              label={t("displayNameLabel")}
              autoComplete="name"
            />
            <FormInputField
              name="email"
              control={form.control}
              label={t("emailLabel")}
              type="email"
              autoComplete="email"
            />
            <FormInputField
              name="password"
              control={form.control}
              label={t("passwordLabel")}
              type="password"
              autoComplete="new-password"
              passwordVisibilityLabels={{
                show: t("showPassword"),
                hide: t("hidePassword"),
              }}
            />
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
              {isSubmitting ? t("submitting") : t("submit")}
            </Button>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
