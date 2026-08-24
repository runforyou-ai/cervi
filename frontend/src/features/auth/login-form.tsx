/** 登录表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { isApiError, login } from "@/api"
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
  createLoginSchema,
  type LoginFormValues,
} from "@/features/auth/login-schema"
import { useSessionController } from "@/features/session/session-context"
import { useSessionRecovery } from "@/lib/session-navigation"
import { apiErrorMessage } from "@/lib/form-errors"

/** 校验并提交登录。 */
export function LoginForm() {
  const { t } = useTranslation("auth")
  const controller = useSessionController()
  const recoverSession = useSessionRecovery()
  const schema = useMemo(() => createLoginSchema(t), [t])
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      email: "",
      password: "",
    },
  })

  /** 提交登录并重新读取权威会话。 */
  async function submitLogin(values: LoginFormValues) {
    try {
      await login(values)
      await controller.reload("login")
    } catch (error) {
      if (recoverSession(error)) {
        return
      }
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["email", "password"]))
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
        <form onSubmit={form.handleSubmit(submitLogin)} noValidate>
          <FieldGroup>
            <FormInputField
              name="email"
              control={form.control}
              label={t("emailLabel")}
              type="email"
              autoComplete="email"
              autoFocus
            />
            <FormInputField
              name="password"
              control={form.control}
              label={t("passwordLabel")}
              type="password"
              autoComplete="current-password"
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
