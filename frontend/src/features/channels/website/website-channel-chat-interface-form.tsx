/** 网站渠道聊天界面表单。 */
import { useEffect, useId, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { ArrowDownIcon, ArrowUpIcon, XIcon } from "lucide-react"
import {
  Controller,
  useFieldArray,
  useForm,
  useWatch,
  type Control,
} from "react-hook-form"
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
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
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
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createWebsiteChannelChatInterfaceSchema({
        titleRequired: t("chatInterface.validation.titleRequired"),
        titleTooLong: t("chatInterface.validation.titleTooLong"),
        greetingTooLong: t("chatInterface.validation.greetingTooLong"),
        themeColorInvalid: t("chatInterface.validation.themeColorInvalid"),
        homeLinkTitleRequired: t(
          "chatInterface.validation.homeLinkTitleRequired"
        ),
        homeLinkTitleTooLong: t("chatInterface.validation.homeLinkTitleTooLong"),
        homeLinkURLInvalid: t("chatInterface.validation.homeLinkURLInvalid"),
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
      homeLinks: channel.chatInterface.homeLinks ?? [],
    },
  })
  const previewValue = useWatch({
    control: form.control,
    compute: (value): WebsiteChannelChatInterfaceInput => ({
      title: value.title ?? "",
      greetingMessage: value.greetingMessage ?? "",
      themeColor: value.themeColor ?? defaultWebsiteChannelThemeColor,
      homeLinks: (value.homeLinks ?? []).map((link) => ({
        title: link?.title ?? "",
        url: link?.url ?? "",
      })),
    }),
  })

  useEffect(() => {
    onPreviewChange(previewValue)
  }, [onPreviewChange, previewValue])

  const { acceptSaved, saveNow } = useAutoSave({ form, schema, save: submit })

  /** 提交聊天界面设置。 */
  async function submit(values: WebsiteChannelChatInterfaceFormValues) {
    try {
      const updated = await updateWebsiteChannelChatInterface(channel.id, values)
      const next = {
        title: updated.title,
        greetingMessage: updated.greetingMessage ?? "",
        themeColor: updated.themeColor,
        homeLinks: updated.homeLinks ?? [],
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
        console.warn("保存网站渠道聊天界面失败", error)
        toast.error(
          apiErrorMessage(error, [
            "title",
            "greetingMessage",
            "themeColor",
            "homeLinks",
          ])
        )
        return false
      }
      console.warn("保存网站渠道聊天界面失败", error)
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

        <HomeLinkFields control={form.control} />
      </FieldGroup>
    </form>
  )
}

/** 编辑 Messenger 首页按顺序展示的链接列表。 */
function HomeLinkFields({
  control,
}: {
  control: Control<WebsiteChannelChatInterfaceFormValues>
}) {
  const { t } = useTranslation("channels")
  const id = useId()
  const { fields, append, remove, move } = useFieldArray({
    control,
    name: "homeLinks",
    keyName: "fieldKey",
  })
  return (
    <div className="space-y-3" role="group" aria-labelledby={`${id}-label`}>
      <div>
        <div id={`${id}-label`} className="text-sm font-medium">
          {t("chatInterface.form.homeLinks")}
        </div>
        <FieldDescription className="mt-1">
          {t("chatInterface.form.homeLinksDescription")}
        </FieldDescription>
      </div>
      {fields.length > 0 ? (
        <div className="divide-y rounded-lg border">
          {fields.map((item, index) => (
            <div
              className="flex items-end gap-3 px-4 py-3"
              key={item.fieldKey}
            >
              <div className="grid min-w-0 flex-1 gap-3 sm:grid-cols-2">
                <FormInputField
                  control={control}
                  name={`homeLinks.${index}.title`}
                  id={`${id}-${item.fieldKey}-title`}
                  label={t("chatInterface.form.homeLinkTitle")}
                />
                <FormInputField
                  control={control}
                  name={`homeLinks.${index}.url`}
                  id={`${id}-${item.fieldKey}-url`}
                  label={t("chatInterface.form.homeLinkURL")}
                  type="url"
                />
              </div>
              <div className="flex shrink-0 items-center">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === 0}
                  aria-label={t("chatInterface.form.moveHomeLinkUp", {
                    number: index + 1,
                  })}
                  title={t("chatInterface.form.moveHomeLinkUp", {
                    number: index + 1,
                  })}
                  onClick={() => move(index, index - 1)}
                >
                  <ArrowUpIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === fields.length - 1}
                  aria-label={t("chatInterface.form.moveHomeLinkDown", {
                    number: index + 1,
                  })}
                  title={t("chatInterface.form.moveHomeLinkDown", {
                    number: index + 1,
                  })}
                  onClick={() => move(index, index + 1)}
                >
                  <ArrowDownIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={t("chatInterface.form.removeHomeLink", {
                    number: index + 1,
                  })}
                  title={t("chatInterface.form.removeHomeLink", {
                    number: index + 1,
                  })}
                  onClick={() => remove(index)}
                >
                  <XIcon />
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : null}
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => append({ title: "", url: "" })}
      >
        {t("chatInterface.form.addHomeLink")}
      </Button>
    </div>
  )
}
