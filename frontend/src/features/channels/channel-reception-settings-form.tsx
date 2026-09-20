/** 消息渠道接待设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  updateMessageChannel,
  type MessageChannelSummary,
} from "@/api"
import { FieldGroup } from "@/components/ui/field"
import { ChannelReceptionSettingsFields } from "@/features/channels/reception/channel-reception-settings-fields"
import {
  createChannelReceptionSchema,
  type ChannelReceptionSettingsFormValues,
} from "@/features/channels/reception/channel-reception-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 修改消息渠道的接待设置。 */
export function ChannelReceptionSettingsForm({
  channel,
  onUpdated,
}: {
  channel: MessageChannelSummary
  onUpdated: (channel: MessageChannelSummary) => void
}) {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createChannelReceptionSchema({
        teamRequired: t("validation.teamRequired"),
        memberRequired: t("validation.memberRequired"),
        fallbackDifferent: t("validation.fallbackDifferent"),
      }),
    [t],
  )
  const form = useForm<ChannelReceptionSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      newConversationTarget: channel.newConversationTarget,
      fallbackTarget: channel.fallbackTarget,
    },
  })
  /** 保存消息渠道接待设置。 */
  const markSaved = useAutoSave({ form, schema, save: submit })

  async function submit(values: ChannelReceptionSettingsFormValues) {
    try {
      const updated = await updateMessageChannel(channel.id, {
        name: channel.name,
        description: channel.description ?? "",
        defaultLocale: channel.defaultLocale,
        ...values,
      })
      const next = {
        newConversationTarget: updated.newConversationTarget,
        fallbackTarget: updated.fallbackTarget,
      }
      form.reset(next)
      markSaved(next)
      onUpdated(updated)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("消息渠道不存在", { channel_id: channel.id })
        navigate("/settings/channels", { replace: true })
        return
      }
      if (isApiError(error)) {
        console.warn("保存消息渠道接待设置失败", error)
        toast.error(
          apiErrorMessage(error, ["newConversationTarget", "fallbackTarget"]),
        )
        return
      }
      console.warn("保存消息渠道接待设置失败", error)
      toast.error(t("form.networkError"))
    }
  }

  return (
    <form
      className="w-full max-w-2xl"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <ChannelReceptionSettingsFields
          control={form.control}
          channelType={channel.type}
        />
      </FieldGroup>
    </form>
  )
}
