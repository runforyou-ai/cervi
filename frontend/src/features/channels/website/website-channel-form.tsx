import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"

import type { WebsiteChannel } from "@/api/channels"
import {
  createWebsiteChannel,
  updateWebsiteChannel,
} from "@/api/channels"
import { ApiError } from "@/api/client"
import { FormFieldMessage } from "@/components/form/form-field-message"
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
import { applyServerFieldErrors } from "@/lib/form-errors"

export function WebsiteChannelForm({
  channel,
}: {
  channel?: WebsiteChannel
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
    defaultValues: {
      name: channel?.name ?? "",
      description: channel?.description ?? "",
      defaultLocale: channel?.defaultLocale ?? "zh-CN",
    },
  })

  async function submit(values: WebsiteChannelFormValues) {
    try {
      if (channel) {
        await updateWebsiteChannel(channel.id, values)
      } else {
        await createWebsiteChannel(values)
      }
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
        if (
          applyServerFieldErrors(form.setError, error.fields, [
            "name",
            "description",
            "defaultLocale",
          ])
        ) {
          return
        }
        form.setError("root", {
          type: "server",
          message: channel ? t("form.updateError") : t("form.createError"),
        })
        return
      }
      form.setError("root", {
        type: "network",
        message: t("form.networkError"),
      })
    }
  }

  const { errors, isSubmitting } = form.formState

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
          placeholder={t("form.namePlaceholder")}
          autoFocus
        />

        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field className="gap-1.5" data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
                aria-describedby={`${field.name}-error`}
              />
              <FormFieldMessage
                id={`${field.name}-error`}
                error={fieldState.error}
              />
            </Field>
          )}
        />

        <Controller
          name="defaultLocale"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field className="gap-1.5" data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("form.defaultLocale")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
                aria-describedby={`${field.name}-error`}
              >
                <option value="zh-CN">{t("locales.zhCN")}</option>
                <option value="en-US">{t("locales.enUS")}</option>
              </NativeSelect>
              <FormFieldMessage
                id={`${field.name}-error`}
                error={fieldState.error}
              />
            </Field>
          )}
        />

        <FormFieldMessage error={errors.root} />

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
