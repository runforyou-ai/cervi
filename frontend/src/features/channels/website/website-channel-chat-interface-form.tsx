/** 网站渠道 Messenger 外观、页签与对话功能表单。 */
import { useEffect, useId, useMemo, type ReactNode } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch, type Control } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
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
import { SwitchCardField } from "@/components/form/switch-card-field"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  createWebsiteChannelChatInterfaceSchema,
  defaultWebsiteChannelThemeColor,
  isWebsiteChannelThemeColor,
  type WebsiteChannelChatInterfaceFormValues,
} from "@/features/channels/website/website-channel-chat-interface-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { apiErrorMessage } from "@/lib/form-errors"

const presetColors = [
  defaultWebsiteChannelThemeColor,
  "#16A34A",
  "#9333EA",
  "#E11D48",
  "#EA580C",
]

/** 修改网站渠道 Messenger 外观、页签与对话功能。 */
export function WebsiteChannelChatInterfaceForm({
  channel,
  onPreviewChange,
  onUpdated,
}: {
  channel: WebsiteChannelData
  onPreviewChange: (value: WebsiteChannelChatInterfaceInput) => void
  onUpdated: (value: WebsiteChannelChatInterface) => void
}) {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createWebsiteChannelChatInterfaceSchema({
        titleRequired: t("chatInterface.validation.titleRequired"),
        titleTooLong: t("chatInterface.validation.titleTooLong"),
        greetingTooLong: t("chatInterface.validation.greetingTooLong"),
        themeColorInvalid: t("chatInterface.validation.themeColorInvalid"),
      }),
    [t]
  )
  const form = useForm<WebsiteChannelChatInterfaceFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      title: channel.chatInterface.title,
      greetingMessage: channel.chatInterface.greetingMessage ?? "",
      themeColor: channel.chatInterface.themeColor,
      homeEnabled: channel.chatInterface.homeEnabled,
      attachmentsEnabled: channel.chatInterface.attachmentsEnabled,
      emojiEnabled: channel.chatInterface.emojiEnabled,
      ratingEnabled: channel.chatInterface.ratingEnabled,
    },
  })
  const previewValue = useWatch({
    control: form.control,
    compute: (value): WebsiteChannelChatInterfaceInput => ({
      title: value.title ?? "",
      greetingMessage: value.greetingMessage ?? "",
      themeColor: value.themeColor ?? defaultWebsiteChannelThemeColor,
      homeEnabled: value.homeEnabled ?? true,
      attachmentsEnabled: value.attachmentsEnabled ?? true,
      emojiEnabled: value.emojiEnabled ?? true,
      ratingEnabled: value.ratingEnabled ?? true,
    }),
  })

  useEffect(() => {
    onPreviewChange(previewValue)
  }, [onPreviewChange, previewValue])

  const { acceptSaved, saveNow } = useAutoSave({ form, schema, save: submit })

  /** 提交 Messenger 设置。 */
  async function submit(values: WebsiteChannelChatInterfaceFormValues) {
    try {
      const updated = await updateWebsiteChannelChatInterface(channel.id, values)
      const next = {
        title: updated.title,
        greetingMessage: updated.greetingMessage ?? "",
        themeColor: updated.themeColor,
        homeEnabled: updated.homeEnabled,
        attachmentsEnabled: updated.attachmentsEnabled,
        emojiEnabled: updated.emojiEnabled,
        ratingEnabled: updated.ratingEnabled,
      }
      acceptSaved(values, next)
      onUpdated(updated)
      return true
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return false
      }
      if (isNotFoundApiError(error)) {
        console.warn("网站渠道不存在", { channel_id: channel.id })
        navigate("/channels", { replace: true })
        return false
      }
      if (isApiError(error)) {
        console.warn("保存网站渠道 Messenger 设置失败", error)
        toast.error(
          apiErrorMessage(error, [
            "title",
            "greetingMessage",
            "themeColor",
            "homeEnabled",
            "attachmentsEnabled",
            "emojiEnabled",
            "ratingEnabled",
          ])
        )
        return false
      }
      console.warn("保存网站渠道 Messenger 设置失败", error)
      toast.error(t("form.networkError"))
      return false
    }
  }

  return (
    <form
      className="w-full"
      onSubmit={form.handleSubmit(() => saveNow())}
      noValidate
    >
      <FieldGroup>
        <FormInputField
          name="title"
          control={form.control}
          label={t("chatInterface.form.title")}
          autoFocus
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

        <SwitchSection title={t("chatInterface.form.tabs")}>
          <ChatInterfaceSwitch
            control={form.control}
            name="homeEnabled"
            label={t("chatInterface.form.homeEnabled")}
            description={t("chatInterface.form.homeEnabledDescription")}
          />
        </SwitchSection>

        <SwitchSection title={t("chatInterface.form.conversationFeatures")}>
          <ChatInterfaceSwitch
            control={form.control}
            name="attachmentsEnabled"
            label={t("chatInterface.form.attachmentsEnabled")}
          />
          <ChatInterfaceSwitch
            control={form.control}
            name="emojiEnabled"
            label={t("chatInterface.form.emojiEnabled")}
          />
          <ChatInterfaceSwitch
            control={form.control}
            name="ratingEnabled"
            label={t("chatInterface.form.ratingEnabled")}
            description={t("chatInterface.form.ratingEnabledDescription")}
          />
        </SwitchSection>
      </FieldGroup>
    </form>
  )
}

/** 带分组标题的开关字段组。 */
function SwitchSection({
  title,
  children,
}: {
  title: string
  children: ReactNode
}) {
  const id = useId()
  return (
    <div className="space-y-3" role="group" aria-labelledby={id}>
      <div id={id} className="text-sm font-medium">
        {title}
      </div>
      {children}
    </div>
  )
}

/** 绑定到 Messenger 表单布尔字段的开关。 */
function ChatInterfaceSwitch({
  control,
  name,
  label,
  description,
}: {
  control: Control<WebsiteChannelChatInterfaceFormValues>
  name: "homeEnabled" | "attachmentsEnabled" | "emojiEnabled" | "ratingEnabled"
  label: string
  description?: string
}) {
  return (
    <Controller
      name={name}
      control={control}
      render={({ field }) => (
        <SwitchCardField
          id={field.name}
          name={field.name}
          label={label}
          description={description}
          checked={field.value}
          onBlur={field.onBlur}
          onCheckedChange={field.onChange}
          ref={field.ref}
        />
      )}
    />
  )
}
