import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ApiError, install } from "@/api"
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
import { apiErrorMessage } from "@/lib/form-errors"

export function SetupForm() {
  const { t } = useTranslation("setup")
  const navigate = useNavigate()
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
