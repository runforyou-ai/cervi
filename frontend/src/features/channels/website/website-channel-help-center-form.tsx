/** 网站渠道帮助中心表单。 */
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  isApiError,
  isNotFoundApiError,
  listKnowledgeBases,
  updateWebsiteChannelHelpCenter,
  type WebsiteChannelData,
} from "@/api"
import { AgentResourcePickerField } from "@/components/agent-fields/agent-resource-picker-field"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const helpCenterSchema = z.object({
  knowledgeBaseIds: z.array(z.string()),
})

type WebsiteChannelHelpCenterFormValues = z.infer<typeof helpCenterSchema>

/** 选择发布到网站渠道帮助中心的知识库。 */
export function WebsiteChannelHelpCenterForm({
  channel,
  onUpdated,
}: {
  channel: WebsiteChannelData
  onUpdated: () => void
}) {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const form = useForm<WebsiteChannelHelpCenterFormValues>({
    resolver: zodResolver(helpCenterSchema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      knowledgeBaseIds: channel.helpCenter.knowledgeBaseIds,
    },
  })

  const { acceptSaved, saveNow } = useAutoSave({
    form,
    schema: helpCenterSchema,
    save: submit,
  })

  /** 提交帮助中心发布的知识库。 */
  async function submit(values: WebsiteChannelHelpCenterFormValues) {
    try {
      const updated = await updateWebsiteChannelHelpCenter(channel.id, values)
      acceptSaved(values, { knowledgeBaseIds: updated.knowledgeBaseIds })
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
        console.warn("保存网站渠道帮助中心失败", error)
        toast.error(apiErrorMessage(error, ["knowledgeBaseIds"]))
        return false
      }
      console.warn("保存网站渠道帮助中心失败", error)
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
        <Controller
          name="knowledgeBaseIds"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel>{t("helpCenter.knowledgeBases")}</FieldLabel>
              <AgentResourcePickerField
                value={field.value}
                onChange={field.onChange}
                disabled={form.formState.isSubmitting}
                resourceKey={resourceKeys.knowledgeBases()}
                load={() => listKnowledgeBases()}
                toOptions={(data) =>
                  data.knowledgeBases.map((base) => ({
                    id: base.id,
                    name: base.name,
                    detail: base.description,
                  }))
                }
                labels={{
                  title: t("helpCenter.pickerTitle"),
                  group: t("helpCenter.knowledgeBases"),
                  unconfigured: t("helpCenter.unconfigured"),
                  selected: (count, names) =>
                    names === ""
                      ? t("helpCenter.selected", { count })
                      : count === 1
                        ? t("helpCenter.selectedOne", { names })
                        : t("helpCenter.selectedNames", { names, count }),
                  empty: t("helpCenter.empty"),
                  loadError: t("helpCenter.loadError"),
                }}
              />
              <FieldDescription>
                {t("helpCenter.knowledgeBasesHelp")}
              </FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
    </form>
  )
}
