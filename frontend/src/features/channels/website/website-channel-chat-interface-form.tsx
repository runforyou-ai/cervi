/** 网站渠道聊天界面表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  updateWebsiteChannelChatInterface,
  type WebsiteChannelData,
  type WebsiteChannelChatInterface,
  type WebsiteChannelChatInterfaceInput,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
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
import { apiErrorMessage } from "@/lib/form-errors"

const presetColors = [
  defaultWebsiteChannelThemeColor,
  "#16A34A",
  "#9333EA",
  "#E11D48",
  "#EA580C",
]

/** 修改网站渠道聊天界面。 */
export function WebsiteChannelChatInterfaceForm({
  channel,
  onPreviewChange,
  onUpdated,
}: {
  channel: WebsiteChannelData
  onPreviewChange: (value: WebsiteChannelChatInterfaceInput) => void
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
  const previewValue = useWatch({
    control: form.control,
    compute: (value): WebsiteChannelChatInterfaceInput => ({
      title: value.title ?? "",
      subtitle: value.subtitle ?? "",
      greetingMessage: value.greetingMessage ?? "",
      themeColor: value.themeColor ?? defaultWebsiteChannelThemeColor,
    }),
  })

  useEffect(() => {
    onPreviewChange({
      title: previewValue.title,
      subtitle: previewValue.subtitle,
      greetingMessage: previewValue.greetingMessage,
      themeColor: previewValue.themeColor,
    })
  }, [
    onPreviewChange,
    previewValue.greetingMessage,
    previewValue.subtitle,
    previewValue.themeColor,
    previewValue.title,
  ])

  /** 提交聊天界面设置。 */
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
      console.info("网站渠道聊天界面已保存", { channel_id: channel.id })
      toast.success(t("chatInterface.saved"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (isNotFoundApiError(error)) {
        console.warn("网站渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
        return
      }
      if (isApiError(error)) {
        console.warn("保存网站渠道聊天界面失败", error)
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
      console.warn("保存网站渠道聊天界面失败", error)
      toast.error(t("form.networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-2xl"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <FormInputField
          name="title"
          control={form.control}
          label={t("chatInterface.form.title")}
          autoFocus
        />

        <FormInputField
          name="subtitle"
          control={form.control}
          label={t("chatInterface.form.subtitle")}
          required={false}
        />

        <Controller
          name="greetingMessage"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("chatInterface.form.greetingMessage")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                rows={4}
                aria-invalid={fieldState.invalid}
              />
            </Field>
          )}
        />

        <Controller
          name="themeColor"
          control={form.control}
          render={({ field, fieldState }) => {
            const colorValue = isWebsiteChannelThemeColor(field.value)
              ? field.value
              : defaultWebsiteChannelThemeColor
            return (
              <Field data-invalid={fieldState.invalid}>
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
                    aria-invalid={fieldState.invalid}
                  />
                </div>
              </Field>
            )
          }}
        />

        <div className="flex items-center gap-2">
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("form.saving") : t("form.save")}
          </Button>
          <Button variant="outline" asChild>
            <Link to="/integrations/channels">{t("form.cancel")}</Link>
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
