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
import { messageChannelTypeDefinitions } from "@/features/channels/message-channel-types"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 创建或修改消息渠道基础信息。 */
export function MessageChannelForm({
  channel,
  onUpdated,
}: {
  channel?: MessageChannelSummary
  onUpdated?: (value: MessageChannelSummary) => void
}) {
  const { t } = useTranslation("channels")
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
    defaultValues: {
      type: channel?.type ?? ChannelType.ChannelTypeWebsite,
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
        form.reset({
          type: updated.type,
          name: updated.name,
          description: updated.description ?? "",
          defaultLocale: updated.defaultLocale,
          newConversationTarget: updated.newConversationTarget,
          fallbackTarget: updated.fallbackTarget,
        })
        onUpdated?.(updated)
        void invalidateResource(resourceKeys.messageChannels())
        void invalidateResource(resourceKeys.channelOptions())
        console.info("消息渠道已保存", {
          channel_id: channel.id,
          channel_type: channel.type,
        })
        toast.success(t("form.saved"))
        return
      }

      const created = await createMessageChannel(values)
      void invalidateResource(resourceKeys.messageChannels())
      void invalidateResource(resourceKeys.channelOptions())
      console.info("消息渠道已创建", {
        channel_id: created.id,
        channel_type: created.type,
      })
      form.reset(values)
      navigate(
        `/integrations/channels/${created.type}/${created.id}?tab=basic`,
        { replace: true },
      )
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (channel && isNotFoundApiError(error)) {
        console.warn("消息渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
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

  return (
    <form
      className="w-full max-w-2xl space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        {!channel ? (
          <Controller
            name="type"
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor={field.name} required>
                  {t("form.type")}
                </FieldLabel>
                <NativeSelect
                  {...field}
                  id={field.name}
                  required
                  aria-invalid={fieldState.invalid}
                >
                  {messageChannelTypeDefinitions.map((definition) => (
                    <option key={definition.type} value={definition.type}>
                      {t(`types.${definition.translationKey}`)}
                    </option>
                  ))}
                </NativeSelect>
              </Field>
            )}
          />
        ) : null}

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
          <ChannelReceptionSettingsFields control={form.control} />
        ) : null}

      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? t("form.saving") : t("form.save")}
        </Button>
        <Button variant="outline" asChild>
          <Link to="/integrations/channels">{t("form.cancel")}</Link>
        </Button>
      </div>
    </form>
  )
}
