/** 网站渠道接待设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  updateWebsiteChannel,
  type WebsiteChannelSummary,
} from "@/api"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { ChannelReceptionSettingsFields } from "@/features/channels/reception/channel-reception-settings-fields"
import {
  createChannelReceptionSchema,
  type ChannelReceptionSettingsFormValues,
} from "@/features/channels/reception/channel-reception-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 修改网站渠道的接待设置。 */
export function WebsiteChannelReceptionSettingsForm({
  channel,
  onUpdated,
}: {
  channel: WebsiteChannelSummary
  onUpdated: (channel: WebsiteChannelSummary) => void
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

  /** 保存网站渠道接待设置。 */
  async function submit(values: ChannelReceptionSettingsFormValues) {
    try {
      const updated = await updateWebsiteChannel(channel.id, {
        type: channel.type,
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
      console.info("网站渠道接待设置已保存", { channel_id: channel.id })
      toast.success(t("routing.saved"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("网站渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
        return
      }
      if (isApiError(error)) {
        console.warn("保存网站渠道接待设置失败", error)
        toast.error(
          apiErrorMessage(error, ["newConversationTarget", "fallbackTarget"]),
        )
        return
      }
      console.warn("保存网站渠道接待设置失败", error)
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
        <ChannelReceptionSettingsFields control={form.control} />
        <div>
          <Button type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting ? t("form.saving") : t("form.save")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
