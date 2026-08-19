import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ApiError, connectServer, getServerURL } from "@/api"
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
import { apiErrorMessage } from "@/lib/form-errors"

export function ServerConnectionForm() {
  const { t } = useTranslation("connection")
  const navigate = useNavigate()
  const [hasServer, setHasServer] = useState(false)
  const schema = useMemo(() => createServerConnectionSchema(t), [t])
  const form = useForm<ServerConnectionFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { serverUrl: "" },
  })
  const { reset } = form

  useEffect(() => {
    void getServerURL().then((serverUrl) => {
      setHasServer(serverUrl !== "")
      reset({ serverUrl })
    })
  }, [reset])

  async function submitServerConnection(values: ServerConnectionFormValues) {
    try {
      await connectServer(values.serverUrl)
      navigate("/login", { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        toast.error(apiErrorMessage(error, ["serverUrl"]))
        return
      }

      toast.error(t("connectionError"))
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
        <form onSubmit={form.handleSubmit(submitServerConnection)} noValidate>
          <FieldGroup>
            <FormInputField
              name="serverUrl"
              control={form.control}
              label={t("serverUrlLabel")}
              type="url"
              autoCapitalize="none"
              autoCorrect="off"
              autoFocus
            />
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
              {isSubmitting ? t("submitting") : t("submit")}
            </Button>
            {hasServer ? (
              <Button
                type="button"
                variant="outline"
                disabled={isSubmitting}
                onClick={() => navigate("/login", { replace: true })}
              >
                {t("backToLogin")}
              </Button>
            ) : null}
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
