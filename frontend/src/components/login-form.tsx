import type { FormEvent } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"

export function LoginForm({
  onLogin,
  className,
  ...props
}: React.ComponentProps<"div"> & {
  onLogin: () => void
}) {
  const { t } = useTranslation("auth")

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    toast.success(t("success"))
    onLogin()
  }

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card>
        <CardHeader>
          <CardTitle>{t("title")}</CardTitle>
          <CardDescription>{t("description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="account">{t("accountLabel")}</FieldLabel>
                <Input
                  id="account"
                  placeholder={t("accountPlaceholder")}
                  autoComplete="username"
                  required
                />
              </Field>
              <Field>
                <div className="flex items-center">
                  <FieldLabel htmlFor="password">
                    {t("passwordLabel")}
                  </FieldLabel>
                  <a
                    href="#"
                    className="ml-auto inline-block text-sm underline-offset-4 hover:underline"
                    onClick={(event) => event.preventDefault()}
                  >
                    {t("forgotPassword")}
                  </a>
                </div>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  required
                />
              </Field>
              <Field>
                <Button type="submit">{t("submit")}</Button>
                <Button variant="outline" type="button">
                  {t("googleLogin")}
                </Button>
                <FieldDescription className="text-center">
                  {t("noAccount")} {" "}
                  <a href="#" onClick={(event) => event.preventDefault()}>
                    {t("signUp")}
                  </a>
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
