/** 在知识库表单中展示独立的模型与处理参数。 */
import { Controller, type Control } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { AIModelType, type AIProviderSummaryData } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { knowledgeEmbeddingDimensions, type KnowledgeBaseFormValues } from "./knowledge-base-schema"

/** 渲染向量模型、数值参数和可清空的重排模型选择。 */
export function KnowledgeBaseSettingsFields({ control, isQA, providers }: {
  control: Control<KnowledgeBaseFormValues>
  isQA: boolean
  providers: AIProviderSummaryData[]
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  return (
    <>
      <KnowledgeModelField control={control} name="embeddingModel" providers={providers} />
      <Controller control={control} name="embeddingDimension" render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor="embeddingDimension" required>{t("form.embeddingDimension")}</FieldLabel>
          <NativeSelect {...field} id="embeddingDimension" required aria-invalid={fieldState.invalid}>
            <option value="">{t("form.selectEmbeddingDimension")}</option>
            {knowledgeEmbeddingDimensions.map((dimension) => (
              <option key={dimension} value={dimension}>{dimension}</option>
            ))}
          </NativeSelect>
        </Field>
      )} />
      {!isQA && (
        <>
          <FormInputField control={control} name="chunkLength" label={t("form.chunkLength")} type="number" min={256} max={2048} step={1} />
          <FormInputField control={control} name="chunkOverlap" label={t("form.chunkOverlap")} type="number" min={0} max={200} step={1} />
        </>
      )}
      <FormInputField control={control} name="retrievalCount" label={t("form.retrievalCount")} type="number" min={1} max={20} step={1} />
      <KnowledgeModelField control={control} name="rerankModel" providers={providers} />
    </>
  )
}

/** 按供应商和模型用途展示知识库的模型选项。 */
function KnowledgeModelField({ control, name, providers }: {
  control: Control<KnowledgeBaseFormValues>
  name: "embeddingModel" | "rerankModel"
  providers: AIProviderSummaryData[]
}) {
  const { t } = useTranslation("knowledgeBase")
  const required = name === "embeddingModel"
  const type = required ? AIModelType.AIModelTypeEmbedding : AIModelType.AIModelTypeRerank
  const groups = providers.map((provider) => ({
    ...provider,
    models: provider.models.filter((model) => model.type === type),
  })).filter((provider) => provider.models.length > 0)
  return (
    <Controller control={control} name={name} render={({ field, fieldState }) => (
      <Field data-invalid={fieldState.invalid}>
        <FieldLabel htmlFor={name} required={required}>{t(`form.${name}`)}</FieldLabel>
        <NativeSelect {...field} id={name} required={required} aria-invalid={fieldState.invalid}>
          <option value="">{t(required ? "form.selectEmbeddingModel" : "form.noRerank")}</option>
          {groups.map((provider) => (
            <optgroup key={provider.id} label={provider.name}>
              {provider.models.map((model) => (
                <option key={model.identifier} value={JSON.stringify([provider.id, model.identifier])}>{model.name}</option>
              ))}
            </optgroup>
          ))}
        </NativeSelect>
        {groups.length === 0 && <FieldDescription>{t(required ? "form.noEmbeddingModels" : "form.noRerankModels")}</FieldDescription>}
      </Field>
    )} />
  )
}
