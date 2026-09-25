/** 企业翻译设置：翻译客户会话消息的模型，修改后自动保存。 */
import { useEffect, useRef } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import { getTranslationSettings, isApiError, listAgentModelOptions, updateTranslationSettings } from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const translationSchema = z.object({ model: z.string() })

type TranslationFormValues = z.infer<typeof translationSchema>

/** 按供应商分组的可选模型。 */
type ModelGroup = { id: string; name: string; models: { identifier: string; name: string }[] }

/** 读取翻译设置与可选对话模型并显示设置表单。 */
export function TranslationSettings() {
  const { t } = useTranslation("settings")
  const settings = useResource(resourceKeys.translationSettings(), () => getTranslationSettings())
  const chatModels = useResource(resourceKeys.agentModelOptions(), () => listAgentModelOptions(), { staleTime: 0 })
  // 翻译模型取支持文本输入的对话模型，按供应商分组。
  const groups: ModelGroup[] = []
  for (const model of chatModels.data ?? []) {
    let group = groups.find((item) => item.id === model.providerId)
    if (!group) {
      group = { id: model.providerId, name: model.providerName, models: [] }
      groups.push(group)
    }
    group.models.push({ identifier: model.modelIdentifier, name: model.modelName })
  }
  const model = settings.data?.model
  return (
    <ResourceContent resources={[settings, chatModels]} errorMessage={t("customerService.translation.loadError")}>
      {settings.data && chatModels.data ? (
        <TranslationForm
          groups={groups}
          values={{ model: model ? JSON.stringify([model.providerId, model.modelIdentifier]) : "" }}
        />
      ) : null}
    </ResourceContent>
  )
}

/** 维护翻译模型，修改后自动保存。 */
function TranslationForm({ groups, values }: { groups: ModelGroup[]; values: TranslationFormValues }) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const form = useForm<TranslationFormValues>({
    shouldUseNativeValidation: true,
    mode: "onChange",
    defaultValues: values,
  })
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const { markSaved } = useAutoSave({ form, schema: translationSchema, save })

  /** 保存翻译模型，空值表示关闭翻译。 */
  async function save(submitted: TranslationFormValues) {
    try {
      const [providerId, modelIdentifier] = submitted.model ? (JSON.parse(submitted.model) as [string, string]) : []
      await updateTranslationSettings({
        model: providerId && modelIdentifier ? { providerId, modelIdentifier } : null,
      })
      void invalidate(resourceKeys.translationSettings())
      void invalidate(resourceKeys.conversationTranslation())
      if (!mounted.current) return true
      markSaved(submitted)
      return true
    } catch (error) {
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存翻译设置失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error, ["model"]) : t("customerService.translation.saveError"))
      return false
    }
  }

  return (
    <form
      className="w-full"
      aria-label={t("customerService.translation.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <Controller
          name="model"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor="translation-model">{t("customerService.translation.model")}</FieldLabel>
              <NativeSelect {...field} id="translation-model">
                <option value="">{t("customerService.models.notUsed")}</option>
                {groups.map((provider) => (
                  <optgroup key={provider.id} label={provider.name}>
                    {provider.models.map((model) => (
                      <option key={model.identifier} value={JSON.stringify([provider.id, model.identifier])}>
                        {model.name}
                      </option>
                    ))}
                  </optgroup>
                ))}
              </NativeSelect>
              <FieldDescription>
                {t("customerService.translation.modelDescription")}
                {groups.length === 0 ? (
                  <>
                    {" "}
                    <Link to="/settings/model-services">{t("customerService.models.configureModels")}</Link>
                  </>
                ) : null}
              </FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
    </form>
  )
}
