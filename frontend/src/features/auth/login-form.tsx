/** 登录表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ApiError, login } from "@/api"
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
import { apiErrorMessage } from "@/lib/form-errors"

/** 校验并提交登录。 */
export function LoginForm({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  const { t } = useTranslation("auth")
  const navigate = useNavigate()
  const schema = useMemo(() => createLoginSchema(t), [t])
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      email: "",
      password: "",
    },
  })

  /** 提交登录并进入收件箱。 */
  async function submitLogin(values: LoginFormValues) {
    try {
      await login(values)
      navigate("/inbox", { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "SERVER_CONNECTION_REQUIRED") {
          navigate("/connect", { replace: true })
          return
        }
        if (error.code === "INSTALLATION_REQUIRED") {
          navigate(allowServerChange ? "/connect" : "/setup", {
            replace: true,
          })
          return
        }
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
            {allowServerChange ? (
              <Button
                type="button"
                variant="outline"
                disabled={isSubmitting}
                onClick={() => navigate("/connect")}
              >
                {t("changeServer")}
              </Button>
            ) : null}
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
