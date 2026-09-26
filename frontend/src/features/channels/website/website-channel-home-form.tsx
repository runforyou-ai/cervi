/** 网站渠道 Messenger 首页表单。 */
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
  updateWebsiteChannelHome,
  WebsiteHomeBlockType,
  type WebsiteChannelData,
  type WebsiteChannelHomeInput,
  type WebsiteHomeBlockTypeId,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { FieldDescription, FieldGroup } from "@/components/ui/field"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  createWebsiteChannelHomeSchema,
  type WebsiteChannelHomeFormValues,
} from "@/features/channels/website/website-channel-home-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 首页卡片类型对应的名称文案键。 */
const blockLabelKeys = {
  [WebsiteHomeBlockType.WebsiteHomeBlockRecentConversation]:
    "home.form.blocks.recentConversation",
  [WebsiteHomeBlockType.WebsiteHomeBlockStartConversation]:
    "home.form.blocks.startConversation",
  [WebsiteHomeBlockType.WebsiteHomeBlockLinks]: "home.form.blocks.links",
} as const satisfies Record<WebsiteHomeBlockTypeId, string>

/** 把服务端首页设置转换为表单值。 */
function homeFormValues(
  home: WebsiteChannelData["home"]
): WebsiteChannelHomeFormValues {
  return {
    welcome: home.welcome,
    headline: home.headline,
    blocks: home.blocks.map((block) => ({
      type: block.type as WebsiteHomeBlockTypeId,
      enabled: block.enabled,
    })),
    links: home.links,
  }
}

/** 修改网站渠道 Messenger 首页问候语、卡片与链接。 */
export function WebsiteChannelHomeForm({
  channel,
  onPreviewChange,
  onUpdated,
}: {
  channel: WebsiteChannelData
  onPreviewChange: (value: WebsiteChannelHomeInput) => void
  onUpdated: () => void
}) {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const greetingId = useId()
  const schema = useMemo(
    () =>
      createWebsiteChannelHomeSchema({
        welcomeTooLong: t("home.validation.welcomeTooLong"),
        headlineTooLong: t("home.validation.headlineTooLong"),
        linkTitleRequired: t("home.validation.linkTitleRequired"),
        linkTitleTooLong: t("home.validation.linkTitleTooLong"),
        linkURLInvalid: t("home.validation.linkURLInvalid"),
      }),
    [t]
  )
  const form = useForm<WebsiteChannelHomeFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: homeFormValues(channel.home),
  })
  const previewValue = useWatch({
    control: form.control,
    compute: (value): WebsiteChannelHomeInput => ({
      welcome: value.welcome ?? "",
      headline: value.headline ?? "",
      blocks: (value.blocks ?? []).flatMap((block) =>
        block?.type
          ? [{ type: block.type, enabled: block.enabled ?? false }]
          : []
      ),
      links: (value.links ?? []).map((link) => ({
        title: link?.title ?? "",
        url: link?.url ?? "",
      })),
    }),
  })

  useEffect(() => {
    onPreviewChange(previewValue)
  }, [onPreviewChange, previewValue])

  const { acceptSaved, saveNow } = useAutoSave({ form, schema, save: submit })

  /** 提交首页设置。 */
  async function submit(values: WebsiteChannelHomeFormValues) {
    try {
      const updated = await updateWebsiteChannelHome(channel.id, values)
      acceptSaved(values, homeFormValues(updated))
      onUpdated()
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
        console.warn("保存网站渠道 Messenger 首页失败", error)
        toast.error(
          apiErrorMessage(error, ["welcome", "headline", "blocks", "links"])
        )
        return false
      }
      console.warn("保存网站渠道 Messenger 首页失败", error)
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
        <div
          className="space-y-3"
          role="group"
          aria-labelledby={`${greetingId}-label`}
        >
          <div>
            <div id={`${greetingId}-label`} className="text-sm font-medium">
              {t("home.form.greeting")}
            </div>
            <FieldDescription className="mt-1">
              {t("home.form.greetingDescription")}
            </FieldDescription>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <FormInputField
              control={form.control}
              name="welcome"
              id={`${greetingId}-welcome`}
              label={t("home.form.welcome")}
              required={false}
            />
            <FormInputField
              control={form.control}
              name="headline"
              id={`${greetingId}-headline`}
              label={t("home.form.headline")}
              required={false}
            />
          </div>
        </div>

        <HomeBlockFields control={form.control} />

        <HomeLinkFields control={form.control} />
      </FieldGroup>
    </form>
  )
}

/** 编辑首页卡片的开关与展示顺序。 */
function HomeBlockFields({
  control,
}: {
  control: Control<WebsiteChannelHomeFormValues>
}) {
  const { t } = useTranslation("channels")
  const id = useId()
  const { fields, move } = useFieldArray({
    control,
    name: "blocks",
    keyName: "fieldKey",
  })
  return (
    <div className="space-y-3" role="group" aria-labelledby={`${id}-label`}>
      <div>
        <div id={`${id}-label`} className="text-sm font-medium">
          {t("home.form.blocks.title")}
        </div>
        <FieldDescription className="mt-1">
          {t("home.form.blocks.description")}
        </FieldDescription>
      </div>
      <div className="divide-y rounded-lg border">
        {fields.map((item, index) => {
          const name = t(blockLabelKeys[item.type])
          const switchId = `${id}-${item.fieldKey}`
          return (
            <div className="flex items-center gap-3 px-4 py-2" key={item.fieldKey}>
              <Controller
                name={`blocks.${index}.enabled`}
                control={control}
                render={({ field }) => (
                  <Switch
                    id={switchId}
                    name={field.name}
                    checked={field.value}
                    onBlur={field.onBlur}
                    onCheckedChange={field.onChange}
                    ref={field.ref}
                  />
                )}
              />
              <Label htmlFor={switchId} className="min-w-0 flex-1">
                {name}
              </Label>
              <div className="flex shrink-0 items-center">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === 0}
                  aria-label={t("home.form.blocks.moveUp", { name })}
                  title={t("home.form.blocks.moveUp", { name })}
                  onClick={() => move(index, index - 1)}
                >
                  <ArrowUpIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === fields.length - 1}
                  aria-label={t("home.form.blocks.moveDown", { name })}
                  title={t("home.form.blocks.moveDown", { name })}
                  onClick={() => move(index, index + 1)}
                >
                  <ArrowDownIcon />
                </Button>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

/** 编辑 Messenger 首页按顺序展示的链接列表。 */
function HomeLinkFields({
  control,
}: {
  control: Control<WebsiteChannelHomeFormValues>
}) {
  const { t } = useTranslation("channels")
  const id = useId()
  const { fields, append, remove, move } = useFieldArray({
    control,
    name: "links",
    keyName: "fieldKey",
  })
  return (
    <div className="space-y-3" role="group" aria-labelledby={`${id}-label`}>
      <div>
        <div id={`${id}-label`} className="text-sm font-medium">
          {t("home.form.links.title")}
        </div>
        <FieldDescription className="mt-1">
          {t("home.form.links.description")}
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
                  name={`links.${index}.title`}
                  id={`${id}-${item.fieldKey}-title`}
                  label={t("home.form.links.linkTitle")}
                />
                <FormInputField
                  control={control}
                  name={`links.${index}.url`}
                  id={`${id}-${item.fieldKey}-url`}
                  label={t("home.form.links.linkURL")}
                  type="url"
                />
              </div>
              <div className="flex shrink-0 items-center">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === 0}
                  aria-label={t("home.form.links.moveUp", {
                    number: index + 1,
                  })}
                  title={t("home.form.links.moveUp", { number: index + 1 })}
                  onClick={() => move(index, index - 1)}
                >
                  <ArrowUpIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  disabled={index === fields.length - 1}
                  aria-label={t("home.form.links.moveDown", {
                    number: index + 1,
                  })}
                  title={t("home.form.links.moveDown", { number: index + 1 })}
                  onClick={() => move(index, index + 1)}
                >
                  <ArrowDownIcon />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={t("home.form.links.remove", {
                    number: index + 1,
                  })}
                  title={t("home.form.links.remove", { number: index + 1 })}
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
        {t("home.form.links.add")}
      </Button>
    </div>
  )
}
