/** 消息渠道基础信息表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  ChannelRoutingTargetType,
  Locale,
  createMessageChannel,
  isApiError,
  isNotFoundApiError,
  updateMessageChannel,
  type MessageChannelSummary,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import { ChannelReceptionSettingsFields } from "@/features/channels/reception/channel-reception-settings-fields"
import {
  createMessageChannelSchema,
  type MessageChannelFormValues,
} from "@/features/channels/message-channel-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 创建或修改消息渠道基础信息。 */
export function MessageChannelForm({
  channel,
  type = ChannelType.ChannelTypeWebsite,
  onUpdated,
}: {
  channel?: MessageChannelSummary
  /** 新建时的渠道类别，由所在的渠道二级菜单决定，表单内不再选择。 */
  type?: ChannelType
  onUpdated?: (value: MessageChannelSummary) => void
}) {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const invalidateResource = useResourceInvalidator()
  const schema = useMemo(
    () =>
      createMessageChannelSchema({
        nameRequired: t("validation.nameRequired"),
        nameTooLong: t("validation.nameTooLong"),
        descriptionTooLong: t("validation.descriptionTooLong"),
        teamRequired: t("validation.teamRequired"),
        memberRequired: t("validation.memberRequired"),
        fallbackDifferent: t("validation.fallbackDifferent"),
      }),
    [t],
  )
  const form = useForm<MessageChannelFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    // 编辑时离开字段即校验以便自动保存；新建时等提交再校验，避免原生校验把焦点锁在必填项上。
    mode: channel ? "onBlur" : "onSubmit",
    defaultValues: {
      type: channel?.type ?? type,
      name: channel?.name ?? "",
      description: channel?.description ?? "",
      defaultLocale: channel?.defaultLocale ?? Locale.LocaleChineseSimplified,
      newConversationTarget: channel?.newConversationTarget ?? {
        type: ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue,
        id: "",
      },
      fallbackTarget: channel?.fallbackTarget ?? {
        type: ChannelRoutingTargetType.ChannelRoutingTargetTypePublicQueue,
        id: "",
      },
    },
  })
  /** 提交消息渠道基础信息。 */
  // 已有渠道时边改边存，新建仍由底部按钮提交。
  const markSaved = useAutoSave({
    form,
    schema,
    save: submit,
    enabled: Boolean(channel),
  })

  async function submit(values: MessageChannelFormValues) {
    try {
      if (channel) {
        const updated = await updateMessageChannel(channel.id, {
          name: values.name,
          description: values.description,
          defaultLocale: values.defaultLocale,
          newConversationTarget: channel.newConversationTarget,
          fallbackTarget: channel.fallbackTarget,
        })
        const next = {
          type: updated.type,
          name: updated.name,
          description: updated.description ?? "",
          defaultLocale: updated.defaultLocale,
          newConversationTarget: updated.newConversationTarget,
          fallbackTarget: updated.fallbackTarget,
        }
        form.reset(next)
        markSaved(next)
        onUpdated?.(updated)
        void invalidateResource(resourceKeys.messageChannels())
        void invalidateResource(resourceKeys.channelOptions())
        return
      }

      const created = await createMessageChannel(values)
      void invalidateResource(resourceKeys.messageChannels())
      void invalidateResource(resourceKeys.channelOptions())
      form.reset(values)
      navigate(
        `/channels/${created.type}/${created.id}?tab=basic`,
        { replace: true },
      )
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (channel && isNotFoundApiError(error)) {
        console.warn("消息渠道不存在", { channel_id: channel.id })
        navigate(`/channels/${channel.type}`, { replace: true })
        return
      }
      if (isApiError(error)) {
        console.warn("保存消息渠道失败", error)
        toast.error(
          apiErrorMessage(error, [
            "type",
            "name",
            "description",
            "defaultLocale",
            "newConversationTarget",
            "fallbackTarget",
          ]),
        )
        return
      }
      console.warn("保存消息渠道失败", error)
      toast.error(t("form.networkError"))
    }
  }

  const { isSubmitting } = form.formState
  const channelType = form.watch("type")

  return (
    <form
      className={cn("w-full", channel ? undefined : "space-y-9")}
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>

        <FormInputField
          name="name"
          control={form.control}
          label={t("form.name")}
          autoFocus
        />

        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>
                {t("form.description")}
              </FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
            </Field>
          )}
        />

        <Controller
          name="defaultLocale"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("form.defaultLocale")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                required
                aria-invalid={fieldState.invalid}
              >
                <option value={Locale.LocaleChineseSimplified}>
                  {t("locales.zhCN")}
                </option>
                <option value={Locale.LocaleEnglishUnitedStates}>
                  {t("locales.enUS")}
                </option>
              </NativeSelect>
            </Field>
          )}
        />

        {!channel ? (
          <ChannelReceptionSettingsFields
            control={form.control}
            channelType={channelType}
          />
        ) : null}

      </FieldGroup>
      {channel ? null : (
        <div className="flex items-center justify-end gap-2">
          <Button variant="outline" asChild>
            <Link to={`/channels/${channelType}`}>{t("common:actions.cancel")}</Link>
          </Button>
          <Button type="submit" disabled={isSubmitting}>
            {isSubmitting ? t("common:actions.saving") : t("common:actions.save")}
          </Button>
        </div>
      )}
    </form>
  )
}
