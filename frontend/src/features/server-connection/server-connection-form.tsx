import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { ApiError } from "@/api/client"
import { connectServer } from "@/api/server-connection"
import { FormFieldMessage } from "@/components/form/form-field-message"
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
  createServerConnectionSchema,
  type ServerConnectionFormValues,
} from "@/features/server-connection/server-connection-schema"
import { applyServerFieldErrors } from "@/lib/form-errors"

export function ServerConnectionForm() {
  const { t } = useTranslation("connection")
  const navigate = useNavigate()
  const schema = useMemo(() => createServerConnectionSchema(t), [t])
  const form = useForm<ServerConnectionFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { serverUrl: "" },
  })

  async function submitServerConnection(values: ServerConnectionFormValues) {
    try {
      await connectServer(values.serverUrl)
      navigate("/inbox", { replace: true })
    } catch (error) {
      if (
        error instanceof ApiError &&
        applyServerFieldErrors(form.setError, error.fields, ["serverUrl"])
      ) {
        return
      }

      form.setError("root", {
        type: error instanceof ApiError ? "server" : "network",
        message: t("connectionError"),
      })
    }
  }

  const { errors, isSubmitting } = form.formState

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <CardDescription>{t("description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={form.handleSubmit(submitServerConnection)} noValidate>
          <FieldGroup className="gap-3">
            <FormInputField
              name="serverUrl"
              control={form.control}
              label={t("serverUrlLabel")}
              type="url"
              placeholder="https://cervi.example.com"
              autoCapitalize="none"
              autoCorrect="off"
              autoFocus
            />
            <FormFieldMessage error={errors.root} />
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
