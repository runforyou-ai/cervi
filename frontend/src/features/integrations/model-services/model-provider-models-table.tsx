/** 模型服务表单中可逐行编辑的模型目录表格。 */
import { Controller, useWatch, type FieldArrayWithId, type UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { AIModelType } from "@/api"
import { ResourceListFrame } from "@/components/resource-list"
import { Button } from "@/components/ui/button"
import { FieldRequiredMark } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { AIProviderFormValues } from "@/features/integrations/model-services/model-provider-schema"
import {
  modelInputModalityNameKeys,
  modelInputModalityOrder,
} from "@/features/integrations/model-services/model-service-options"

/** 编辑模型的类型、标识、名称、输入类型和 Token 上限，对话模型才填写最大输出。 */
export function ModelProviderModelsTable({
  form,
  fields,
  onRemove,
}: {
  form: UseFormReturn<AIProviderFormValues>
  fields: FieldArrayWithId<AIProviderFormValues, "models", "id">[]
  onRemove: (index: number) => void
}) {
  const { t } = useTranslation(["integrations", "common"])
  const watchedModels = useWatch({ control: form.control, name: "models" })
  const hasChatModel = watchedModels.some(
    (model) => model.type === AIModelType.AIModelTypeChat,
  )

  return (
    <ResourceListFrame>
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.type")}
                <FieldRequiredMark />
              </span>
            </TableHead>
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.identifier")}
                <FieldRequiredMark />
              </span>
            </TableHead>
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.name")}
                <FieldRequiredMark />
              </span>
            </TableHead>
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.inputModalities")}
                <FieldRequiredMark />
              </span>
            </TableHead>
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.contextWindow")}
                <FieldRequiredMark />
              </span>
            </TableHead>
            <TableHead>
              <span className="inline-flex items-center gap-1">
                {t("modelServices.models.columns.maxOutputTokens")}
                {hasChatModel ? <FieldRequiredMark /> : null}
              </span>
            </TableHead>
            <TableHead className="w-px">{t("common:table.actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {fields.length === 0 ? (
            <TableRow className="hover:bg-transparent">
              <TableCell
                colSpan={7}
                className="h-24 text-center text-muted-foreground"
              >
                {t("modelServices.models.empty")}
              </TableCell>
            </TableRow>
          ) : (
            fields.map((model, index) => {
              const modelType = watchedModels[index]?.type ?? model.type
              return (
                <TableRow key={model.id}>
                  <TableCell>
                    <Controller
                      name={`models.${index}.type`}
                      control={form.control}
                      render={({ field, fieldState }) => (
                        <NativeSelect
                          {...field}
                          required
                          className="min-w-28"
                          aria-label={t("modelServices.models.editField", {
                            field: t("modelServices.models.columns.type"),
                            row: index + 1,
                          })}
                          aria-invalid={fieldState.invalid}
                        >
                          <option value={AIModelType.AIModelTypeChat}>
                            {t("modelServices.models.types.chat")}
                          </option>
                          <option value={AIModelType.AIModelTypeEmbedding}>
                            {t("modelServices.models.types.embedding")}
                          </option>
                          <option value={AIModelType.AIModelTypeRerank}>
                            {t("modelServices.models.types.rerank")}
                          </option>
                        </NativeSelect>
                      )}
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      {...form.register(`models.${index}.identifier`)}
                      required
                      maxLength={200}
                      autoComplete="off"
                      className="min-w-40 font-mono text-xs"
                      aria-label={t("modelServices.models.editField", {
                        field: t("modelServices.models.columns.identifier"),
                        row: index + 1,
                      })}
                      aria-invalid={Boolean(
                        form.formState.errors.models?.[index]?.identifier,
                      )}
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      {...form.register(`models.${index}.name`)}
                      required
                      maxLength={200}
                      autoComplete="off"
                      className="min-w-36"
                      aria-label={t("modelServices.models.editField", {
                        field: t("modelServices.models.columns.name"),
                        row: index + 1,
                      })}
                      aria-invalid={Boolean(
                        form.formState.errors.models?.[index]?.name,
                      )}
                    />
                  </TableCell>
                  <TableCell className="min-w-56">
                    <div className="flex flex-wrap gap-x-3 gap-y-2">
                      {modelInputModalityOrder.map((modality) => (
                        <label
                          key={modality}
                          className="inline-flex items-center gap-1.5 text-xs"
                        >
                          <input
                            {...form.register(
                              `models.${index}.inputModalities`,
                            )}
                            type="checkbox"
                            value={modality}
                            className="size-4 accent-primary"
                          />
                          {t(modelInputModalityNameKeys[modality])}
                        </label>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Input
                      {...form.register(`models.${index}.contextWindow`)}
                      required
                      inputMode="decimal"
                      autoComplete="off"
                      className="min-w-24"
                      aria-label={t("modelServices.models.editField", {
                        field: t("modelServices.models.columns.contextWindow"),
                        row: index + 1,
                      })}
                      aria-invalid={Boolean(
                        form.formState.errors.models?.[index]?.contextWindow,
                      )}
                    />
                  </TableCell>
                  <TableCell>
                    {modelType === AIModelType.AIModelTypeChat ? (
                      <Input
                        {...form.register(`models.${index}.maxOutputTokens`)}
                        required
                        inputMode="decimal"
                        autoComplete="off"
                        className="min-w-24"
                        aria-label={t("modelServices.models.editField", {
                          field: t(
                            "modelServices.models.columns.maxOutputTokens",
                          ),
                          row: index + 1,
                        })}
                        aria-invalid={Boolean(
                          form.formState.errors.models?.[index]
                            ?.maxOutputTokens,
                        )}
                      />
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="whitespace-nowrap">
                    <Button
                      type="button"
                      variant="outline"
                      size="xs"
                      onClick={() => onRemove(index)}
                    >
                      {t("common:actions.delete")}
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })
          )}
        </TableBody>
      </Table>
    </ResourceListFrame>
  )
}
