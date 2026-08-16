import { useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { EyeIcon, EyeOffIcon, LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ApiError } from "@/api/client"
import { install } from "@/api/installation"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createSetupSchema,
  type SetupFormValues,
} from "@/features/installation/setup-schema"
import { apiErrorMessage } from "@/lib/form-errors"

export function SetupForm() {
  const { t } = useTranslation("setup")
  const navigate = useNavigate()
  const [showPassword, setShowPassword] = useState(false)
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

  async function submitSetup(values: SetupFormValues) {
    try {
      await install(values)
      navigate("/inbox", { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "SERVER_CONNECTION_REQUIRED") {
          navigate("/connect", { replace: true })
          return
        }
        if (error.code === "ALREADY_INITIALIZED") {
          navigate("/login", { replace: true })
          return
        }
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
        <form onSubmit={form.handleSubmit(submitSetup)}>
          <FieldGroup className="gap-3">
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
            <Controller
              name="password"
              control={form.control}
              render={({ field }) => (
                <Field className="gap-1.5">
                  <FieldLabel htmlFor={field.name} required>
                    {t("passwordLabel")}
                  </FieldLabel>
                  <div className="relative">
                    <Input
                      {...field}
                      id={field.name}
                      type={showPassword ? "text" : "password"}
                      autoComplete="new-password"
                      className="pr-10"
                      required
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      className="absolute top-1/2 right-1 -translate-y-1/2"
                      aria-label={
                        showPassword ? t("hidePassword") : t("showPassword")
                      }
                      onClick={() => setShowPassword((visible) => !visible)}
                    >
                      {showPassword ? <EyeOffIcon /> : <EyeIcon />}
                    </Button>
                  </div>
                </Field>
              )}
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
