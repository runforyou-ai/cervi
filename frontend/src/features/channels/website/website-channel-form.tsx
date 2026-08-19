import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  Locale,
  createWebsiteChannel,
  updateWebsiteChannel,
  type WebsiteChannelSummary,
} from "@/api/channels"
import { ApiError } from "@/api/client"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import {
  createWebsiteChannelSchema,
  type WebsiteChannelFormValues,
} from "@/features/channels/website/website-channel-schema"
import { apiErrorMessage } from "@/lib/form-errors"

export function WebsiteChannelForm({
  channel,
  onUpdated,
}: {
  channel?: WebsiteChannelSummary
  onUpdated?: (value: WebsiteChannelSummary) => void
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createWebsiteChannelSchema({
        nameRequired: t("validation.nameRequired"),
        nameTooLong: t("validation.nameTooLong"),
        descriptionTooLong: t("validation.descriptionTooLong"),
      }),
    [t]
  )
  const form = useForm<WebsiteChannelFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: channel?.name ?? "",
      description: channel?.description ?? "",
      defaultLocale: channel?.defaultLocale ?? Locale.LocaleChineseSimplified,
    },
  })

  async function submit(values: WebsiteChannelFormValues) {
    try {
      if (channel) {
        const updated = await updateWebsiteChannel(channel.id, values)
        form.reset({
          name: updated.name,
          description: updated.description ?? "",
          defaultLocale: updated.defaultLocale,
        })
        onUpdated?.(updated)
        toast.success(t("form.saved"))
        return
      }

      await createWebsiteChannel(values)
      navigate("/channels/website", { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "AUTH_REQUIRED") {
          navigate("/login", { replace: true })
          return
        }
        if (error.code === "CHANNEL_NOT_FOUND") {
          navigate("/channels/website", { replace: true })
          return
        }
        toast.error(
          apiErrorMessage(error, ["name", "description", "defaultLocale"])
        )
        return
      }
      toast.error(t("form.networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="mt-6 w-full"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup className="gap-6">
        <FormInputField
          name="name"
          control={form.control}
          label={t("form.name")}
          autoFocus
        />

        <Controller
          name="description"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor={field.name}>
                {t("form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
              />
            </Field>
          )}
        />

        <Controller
          name="defaultLocale"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor={field.name} required>
                {t("form.defaultLocale")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                required
              >
                <option value={Locale.LocaleChineseSimplified}>{t("locales.zhCN")}</option>
                <option value={Locale.LocaleEnglishUnitedStates}>{t("locales.enUS")}</option>
              </NativeSelect>
            </Field>
          )}
        />

        <div className="flex items-center gap-4">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("form.saving") : t("form.save")}
          </Button>
          <Button variant="outline" asChild>
            <Link to="/channels/website">{t("form.cancel")}</Link>
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
