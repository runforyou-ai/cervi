import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  type WebsiteChannel,
  type WebsiteChannelChatInterface,
  updateWebsiteChannelChatInterface,
} from "@/api/channels"
import { ApiError } from "@/api/client"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  createWebsiteChannelChatInterfaceSchema,
  defaultWebsiteChannelThemeColor,
  isWebsiteChannelThemeColor,
  type WebsiteChannelChatInterfaceFormValues,
} from "@/features/channels/website/website-channel-chat-interface-schema"
import { WebsiteChatPreview } from "@/features/channels/website/website-chat-preview"
import { apiErrorMessage } from "@/lib/form-errors"

const presetColors = [
  "#2563EB",
  "#16A34A",
  "#9333EA",
  "#E11D48",
  "#EA580C",
]

export function WebsiteChannelChatInterfaceForm({
  channel,
  onUpdated,
}: {
  channel: WebsiteChannel
  onUpdated: (value: WebsiteChannelChatInterface) => void
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createWebsiteChannelChatInterfaceSchema({
        titleRequired: t("chatInterface.validation.titleRequired"),
        titleTooLong: t("chatInterface.validation.titleTooLong"),
        subtitleTooLong: t("chatInterface.validation.subtitleTooLong"),
        greetingTooLong: t("chatInterface.validation.greetingTooLong"),
        themeColorInvalid: t("chatInterface.validation.themeColorInvalid"),
      }),
    [t]
  )
  const form = useForm<WebsiteChannelChatInterfaceFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      title: channel.chatInterface.title,
      subtitle: channel.chatInterface.subtitle ?? "",
      greetingMessage: channel.chatInterface.greetingMessage ?? "",
      themeColor: channel.chatInterface.themeColor,
    },
  })
  const previewValue = useWatch({ control: form.control })

  async function submit(values: WebsiteChannelChatInterfaceFormValues) {
    try {
      const updated = await updateWebsiteChannelChatInterface(channel.id, values)
      form.reset({
        title: updated.title,
        subtitle: updated.subtitle ?? "",
        greetingMessage: updated.greetingMessage ?? "",
        themeColor: updated.themeColor,
      })
      onUpdated(updated)
      toast.success(t("chatInterface.saved"))
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
          apiErrorMessage(error, [
            "title",
            "subtitle",
            "greetingMessage",
            "themeColor",
          ])
        )
        return
      }
      toast.error(t("form.networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <div className="mt-6 grid gap-8 xl:grid-cols-[minmax(0,1fr)_400px]">
      <form className="w-full" onSubmit={form.handleSubmit(submit)} noValidate>
        <FieldGroup className="gap-6">
          <FormInputField
            name="title"
            control={form.control}
            label={t("chatInterface.form.title")}
            maxLength={100}
            autoFocus
          />

          <FormInputField
            name="subtitle"
            control={form.control}
            label={t("chatInterface.form.subtitle")}
            maxLength={120}
            required={false}
          />

          <Controller
            name="greetingMessage"
            control={form.control}
            render={({ field }) => (
              <Field>
                <FieldLabel htmlFor={field.name}>
                  {t("chatInterface.form.greetingMessage")}
                </FieldLabel>
                <Textarea {...field} id={field.name} rows={4} maxLength={500} />
              </Field>
            )}
          />

          <Controller
            name="themeColor"
            control={form.control}
            render={({ field }) => {
              const colorValue = isWebsiteChannelThemeColor(field.value)
                ? field.value
                : defaultWebsiteChannelThemeColor
              return (
                <Field>
                  <FieldLabel htmlFor={field.name} required>
                    {t("chatInterface.form.themeColor")}
                  </FieldLabel>
                  <div className="flex flex-wrap items-center gap-2.5">
                    {presetColors.map((color) => (
                      <button
                        key={color}
                        type="button"
                        className="size-8 rounded-full ring-offset-2 aria-pressed:ring-2 aria-pressed:ring-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
                        style={{ backgroundColor: color }}
                        aria-label={color}
                        aria-pressed={field.value.toUpperCase() === color}
                        title={color}
                        onClick={() => field.onChange(color)}
                      />
                    ))}
                    <input
                      type="color"
                      className="h-9 w-12 rounded-md border bg-transparent p-1"
                      value={colorValue}
                      aria-label={t("chatInterface.form.colorPicker")}
                      onChange={(event) =>
                        field.onChange(event.target.value.toUpperCase())
                      }
                    />
                    <Input
                      {...field}
                      id={field.name}
                      className="w-32 font-mono uppercase"
                      maxLength={7}
                      required
                    />
                  </div>
                </Field>
              )
            }}
          />

          <div>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting
                ? t("form.saving")
                : t("form.save")}
            </Button>
          </div>
        </FieldGroup>
      </form>

      <WebsiteChatPreview value={previewValue} />
    </div>
  )
}
