/** 消息渠道接待设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  updateMessageChannel,
  type MessageChannelSummary,
} from "@/api"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { useWorkspaceTabDirty } from "@/contexts/workspace-tab-lifecycle"
import { ChannelReceptionSettingsFields } from "@/features/channels/reception/channel-reception-settings-fields"
import {
  createChannelReceptionSchema,
  type ChannelReceptionSettingsFormValues,
} from "@/features/channels/reception/channel-reception-schema"
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
  const { t } = useTranslation("channels")
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
    defaultValues: {
      newConversationTarget: channel.newConversationTarget,
      fallbackTarget: channel.fallbackTarget,
    },
  })
  useWorkspaceTabDirty(
    form.formState.isDirty && !form.formState.isSubmitting,
  )

  /** 保存消息渠道接待设置。 */
  async function submit(values: ChannelReceptionSettingsFormValues) {
    try {
      const updated = await updateMessageChannel(channel.id, {
        name: channel.name,
        description: channel.description ?? "",
        defaultLocale: channel.defaultLocale,
        ...values,
      })
      form.reset({
        newConversationTarget: updated.newConversationTarget,
        fallbackTarget: updated.fallbackTarget,
      })
      onUpdated(updated)
      console.info("消息渠道接待设置已保存", {
        channel_id: channel.id,
        channel_type: channel.type,
      })
      toast.success(t("routing.saved"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("消息渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
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
      className="w-full max-w-2xl space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup>
        <ChannelReceptionSettingsFields control={form.control} />
      </FieldGroup>
      <div className="flex items-center gap-2">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting ? t("form.saving") : t("form.save")}
        </Button>
        <Button variant="outline" asChild>
          <Link to="/integrations/channels">{t("form.cancel")}</Link>
        </Button>
      </div>
    </form>
  )
}
