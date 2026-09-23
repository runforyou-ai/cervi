/** 企业会话小结设置：判断模型、小结模型与小结语言，修改后自动保存。 */
import { useEffect, useRef } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  AIModelType,
  getServiceSummarySettings,
  isApiError,
  listAgentModelOptions,
  listAIProviders,
  Locale,
  updateServiceSummarySettings,
  type AIModelReference,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const serviceSummarySchema = z.object({
  decision: z.string(),
  summary: z.string(),
  locale: z.enum([Locale.LocaleChineseSimplified, Locale.LocaleEnglishUnitedStates]),
})

type ServiceSummaryFormValues = z.infer<typeof serviceSummarySchema>

/** 按供应商分组的可选模型。 */
type ModelGroup = { id: string; name: string; models: { identifier: string; name: string }[] }

/** 把模型引用编码为选择值，未设置时为空字符串。 */
function modelValue(reference: AIModelReference | null) {
  return reference ? JSON.stringify([reference.providerId, reference.modelIdentifier]) : ""
}

/** 解析模型选择值，空字符串表示不使用。 */
function modelReference(value: string): AIModelReference | null {
  if (!value) return null
  const [providerId, modelIdentifier] = JSON.parse(value) as [string, string]
  return { providerId, modelIdentifier }
}

/** 读取会话小结设置与可选模型并显示设置表单。 */
export function ServiceSummarySettings() {
  const { t } = useTranslation("settings")
  const settings = useResource(resourceKeys.serviceSummarySettings(), () => getServiceSummarySettings())
  const providers = useResource(resourceKeys.aiProviders(), () => listAIProviders(), { staleTime: 0 })
  const chatModels = useResource(resourceKeys.agentModelOptions(), () => listAgentModelOptions(), { staleTime: 0 })
  // 判断模型取判断用途的模型，小结模型取支持文本输入的对话模型。
  const decisionGroups: ModelGroup[] = (providers.data?.providers ?? [])
    .map((provider) => ({
      id: provider.id,
      name: provider.name,
      models: provider.models.filter((model) => model.type === AIModelType.AIModelTypeDecision),
    }))
    .filter((provider) => provider.models.length > 0)
  const summaryGroups: ModelGroup[] = []
  for (const model of chatModels.data ?? []) {
    let group = summaryGroups.find((item) => item.id === model.providerId)
    if (!group) {
      group = { id: model.providerId, name: model.providerName, models: [] }
      summaryGroups.push(group)
    }
    group.models.push({ identifier: model.modelIdentifier, name: model.modelName })
  }
  return (
    <ResourceContent resources={[settings, providers, chatModels]} errorMessage={t("customerService.summary.loadError")}>
      {settings.data && providers.data && chatModels.data ? (
        <ServiceSummaryForm
          groups={{ decision: decisionGroups, summary: summaryGroups }}
          values={{
            decision: modelValue(settings.data.decision),
            summary: modelValue(settings.data.summary),
            locale: settings.data.locale as ServiceSummaryFormValues["locale"],
          }}
        />
      ) : null}
    </ResourceContent>
  )
}

/** 维护判断模型、小结模型与小结语言，修改后自动保存。 */
function ServiceSummaryForm({
  groups,
  values,
}: {
  groups: Record<"decision" | "summary", ModelGroup[]>
  values: ServiceSummaryFormValues
}) {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const form = useForm<ServiceSummaryFormValues>({
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
  const { markSaved } = useAutoSave({ form, schema: serviceSummarySchema, save })

  /** 保存会话小结设置。 */
  async function save(submitted: ServiceSummaryFormValues) {
    try {
      await updateServiceSummarySettings({
        decision: modelReference(submitted.decision),
        summary: modelReference(submitted.summary),
        locale: submitted.locale,
      })
      void invalidate(resourceKeys.serviceSummarySettings())
      if (!mounted.current) return true
      markSaved(submitted)
      return true
    } catch (error) {
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存会话小结设置失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["decision", "summary", "locale"]))
        return false
      }
      toast.error(t("customerService.summary.saveError"))
      return false
    }
  }

  return (
    <form
      className="w-full"
      aria-label={t("customerService.summary.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        {(["decision", "summary"] as const).map((name) => (
            <Controller
              key={name}
              name={name}
              control={form.control}
              render={({ field }) => (
                <Field>
                  <FieldLabel htmlFor={`summary-${name}`}>
                    {t(`customerService.summary.${name}`)}
                  </FieldLabel>
                  <NativeSelect {...field} id={`summary-${name}`}>
                    <option value="">{t("customerService.summary.notUsed")}</option>
                    {groups[name].map((provider) => (
                      <optgroup key={provider.id} label={provider.name}>
                        {provider.models.map((model) => (
                          <option
                            key={model.identifier}
                            value={JSON.stringify([provider.id, model.identifier])}
                          >
                            {model.name}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </NativeSelect>
                  <FieldDescription>
                    {t(`customerService.summary.${name}Description`)}
                    {groups[name].length === 0 ? (
                      <>
                        {" "}
                        <Link to="/settings/model-services">
                          {t("customerService.summary.configureModels")}
                        </Link>
                      </>
                    ) : null}
                  </FieldDescription>
                </Field>
              )}
            />
        ))}
        <Controller
          name="locale"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor="summary-locale" required>
                {t("customerService.summary.locale")}
              </FieldLabel>
              <NativeSelect {...field} id="summary-locale" required>
                <option value={Locale.LocaleChineseSimplified}>
                  {t("preferences.languages.zhCN")}
                </option>
                <option value={Locale.LocaleEnglishUnitedStates}>
                  {t("preferences.languages.enUS")}
                </option>
              </NativeSelect>
              <FieldDescription>{t("customerService.summary.localeDescription")}</FieldDescription>
            </Field>
          )}
        />
      </FieldGroup>
    </form>
  )
}
