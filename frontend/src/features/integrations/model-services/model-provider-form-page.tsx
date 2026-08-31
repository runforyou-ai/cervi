/** 模型服务供应商表单页。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import {
  Controller,
  type FieldErrors,
  useFieldArray,
  useForm,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  AIModelInputModality,
  AIModelType,
  createAIProvider,
  getAIProvider,
  isApiError,
  listAvailableAIModels,
  testAIProviderConnection,
  updateAIProvider,
  type AIProviderBrandId,
  type AIProviderModelData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { FormValidationMessage } from "@/components/form/form-validation-message"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
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
import {
  aiProviderBrandConfigs,
  aiProviderBrandOrder,
} from "@/features/integrations/model-services/model-provider-brands"
import {
  modelInputModalityNameKeys,
  modelInputModalityOrder,
  modelServiceSectionConfigs,
  modelTypeNameKeys,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import {
  createAIProviderSchema,
  parseTokenCount,
  type AIProviderFormValues,
} from "@/features/integrations/model-services/model-provider-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 把模型 Token 数转换为紧凑显示值。 */
function formatTokenCount(value: number) {
  if (value % 1_048_576 === 0) return `${value / 1_048_576}M`
  if (value % 1024 === 0) return `${value / 1024}K`
  return String(value)
}

/** 把模型契约转换为表单值。 */
function modelFormValue(model: AIProviderModelData) {
  return {
    identifier: model.identifier,
    name: model.name,
    type: model.type,
    inputModalities: model.inputModalities,
    contextWindow: formatTokenCount(model.contextWindow),
    maxOutputTokens:
      model.maxOutputTokens > 0 ? formatTokenCount(model.maxOutputTokens) : "",
  }
}

/** 返回模型目录中的第一条字段校验提示。 */
function modelValidationMessage(
  errors: FieldErrors<AIProviderFormValues>["models"],
) {
  if (!errors) return ""
  if (typeof errors.message === "string") return errors.message
  if (typeof errors.root?.message === "string") return errors.root.message
  if (!Array.isArray(errors)) return ""
  for (const model of errors) {
    if (!model) continue
    const error =
      model.identifier ??
      model.name ??
      model.contextWindow ??
      model.maxOutputTokens
    if (typeof error?.message === "string") return error.message
  }
  return ""
}

/** 编辑供应商连接和模型目录。 */
export function ModelProviderFormPage({
  mode,
  returnSection,
}: {
  mode: "create" | "edit"
  returnSection: ModelServiceSection
}) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const { providerId = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const [modelDialogOpen, setModelDialogOpen] = useState(false)
  const [availableModels, setAvailableModels] = useState<AIProviderModelData[]>(
    [],
  )
  const [draftModelIDs, setDraftModelIDs] = useState<Set<string>>(new Set())
  const [loadingModels, setLoadingModels] = useState(false)
  const [testingConnection, setTestingConnection] = useState(false)
  const mounted = useRef(true)
  const listPath = `/integrations/model-services/${returnSection}`
  const initialBrand = modelServiceSectionConfigs[returnSection].defaultBrand
  const schema = useMemo(
    () =>
      createAIProviderSchema({
        brandInvalid: t("modelServices.validation.brandInvalid"),
        nameRequired: t("modelServices.validation.nameRequired"),
        nameTooLong: t("modelServices.validation.nameTooLong"),
        apiKeyRequired: t("modelServices.validation.apiKeyRequired"),
        apiKeyTooLong: t("modelServices.validation.apiKeyTooLong"),
        apiUrlRequired: t("modelServices.validation.apiUrlRequired"),
        apiUrlInvalid: t("modelServices.validation.apiUrlInvalid"),
        modelIdentifierRequired: t(
          "modelServices.validation.modelIdentifierRequired",
        ),
        modelIdentifierTooLong: t(
          "modelServices.validation.modelIdentifierTooLong",
        ),
        modelNameRequired: t("modelServices.validation.modelNameRequired"),
        modelNameTooLong: t("modelServices.validation.modelNameTooLong"),
        modelTypeInvalid: t("modelServices.validation.modelTypeInvalid"),
        inputModalityInvalid: t(
          "modelServices.validation.inputModalityInvalid",
        ),
        inputModalitiesRequired: t(
          "modelServices.validation.inputModalitiesRequired",
        ),
        contextWindowInvalid: t(
          "modelServices.validation.contextWindowInvalid",
        ),
        maxOutputTokensInvalid: t(
          "modelServices.validation.maxOutputTokensInvalid",
        ),
        modelIdentifierDuplicate: t(
          "modelServices.validation.modelIdentifierDuplicate",
        ),
        modelsRequired: t("modelServices.validation.modelsRequired"),
      }),
    [t],
  )
  const form = useForm<AIProviderFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      brand: initialBrand,
      name: "",
      apiKey: "",
      apiUrl: aiProviderBrandConfigs[initialBrand].defaultAPIURL,
      models: [],
    },
  })
  const modelFields = useFieldArray({
    control: form.control,
    name: "models",
  })

  const {
    data: provider,
    loading: providerLoading,
    refreshing: providerRefreshing,
    error: providerError,
    refresh,
  } = useResource(
    resourceKeys.aiProvider(providerId),
    () => getAIProvider(providerId),
    { enabled: mode === "edit" },
  )
  const loading =
    mode === "edit" &&
    (providerLoading || (Boolean(providerError) && providerRefreshing))
  const loadError = mode === "edit" && Boolean(providerError) && !loading

  /** 详情就绪后回填供应商表单。 */
  useEffect(() => {
    if (!provider) return
    form.reset({
      brand: provider.brand,
      name: provider.name,
      apiKey: provider.apiKey,
      apiUrl: provider.apiUrl,
      models: provider.models.map(modelFormValue),
    })
  }, [form, provider])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 获取当前品牌的预设模型并打开选择弹窗。 */
  async function openModelDialog() {
    if (loadingModels) return
    setLoadingModels(true)
    const brand = form.getValues("brand")
    try {
      const models = await listAvailableAIModels(brand)
      if (!mounted.current) return
      setAvailableModels(models)
      setDraftModelIDs(new Set(models.map((model) => model.identifier)))
      setModelDialogOpen(true)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("预设模型加载失败", { brand, error: requestError })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, ["brand"])
          : t("modelServices.models.loadError"),
      )
    } finally {
      if (mounted.current) setLoadingModels(false)
    }
  }

  /** 切换弹窗中的待选模型。 */
  function toggleDraftModel(identifier: string, checked: boolean) {
    setDraftModelIDs((current) => {
      const next = new Set(current)
      if (checked) next.add(identifier)
      else next.delete(identifier)
      return next
    })
  }

  /** 取消选择弹窗中的全部预设模型。 */
  function clearDraftModels() {
    setDraftModelIDs(new Set())
  }

  /** 确认模型选择并追加目录中尚不存在的模型。 */
  function confirmModels() {
    const current = form.getValues("models")
    const existingIDs = new Set(
      current.map((model) => model.identifier.trim()),
    )
    const modelsToAppend = availableModels
      .filter(
        (model) =>
          draftModelIDs.has(model.identifier) &&
          !existingIDs.has(model.identifier),
      )
      .map(modelFormValue)
    if (modelsToAppend.length > 0) modelFields.append(modelsToAppend)
    setModelDialogOpen(false)
  }

  /** 添加一个文本输入的自定义对话模型。 */
  function addCustomModel() {
    modelFields.append({
      identifier: "",
      name: "",
      type: AIModelType.AIModelTypeChat,
      inputModalities: [AIModelInputModality.AIModelInputModalityText],
      contextWindow: "",
      maxOutputTokens: "",
    })
  }

  /** 使用当前未保存的地址和密钥测试模型服务连接。 */
  async function testConnection() {
    if (testingConnection || form.formState.isSubmitting) return
    const valid = await form.trigger(["brand", "apiKey", "apiUrl"], {
      shouldFocus: true,
    })
    if (!valid || !mounted.current) return
    const { brand, apiKey, apiUrl } = form.getValues()
    setTestingConnection(true)
    try {
      await testAIProviderConnection({ brand, apiKey, apiUrl })
      if (!mounted.current) return
      toast.success(t("modelServices.form.testSuccess"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("模型服务连接测试失败", { brand, error: requestError })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, ["brand", "apiKey", "apiUrl"])
          : t("modelServices.form.testError"),
      )
    } finally {
      if (mounted.current) setTestingConnection(false)
    }
  }

  /** 创建或保存模型服务供应商。 */
  async function save(values: AIProviderFormValues) {
    const input = {
      brand: values.brand,
      name: values.name,
      apiKey: values.apiKey,
      apiUrl: values.apiUrl,
      models: values.models.map((model) => ({
        identifier: model.identifier,
        name: model.name,
        type: model.type,
        inputModalities: model.inputModalities,
        contextWindow: parseTokenCount(model.contextWindow)!,
        maxOutputTokens:
          model.type === AIModelType.AIModelTypeChat
            ? parseTokenCount(model.maxOutputTokens)!
            : 0,
      })),
    }
    try {
      if (mode === "create") {
        await createAIProvider(input)
      } else {
        await updateAIProvider(providerId, input)
        void invalidateResource(resourceKeys.aiProvider(providerId))
      }
      void invalidateResource(resourceKeys.aiProviders())
      if (!mounted.current) return
      form.reset(values)
      toast.success(
        mode === "create"
          ? t("modelServices.form.createSuccess")
          : t("modelServices.form.updateSuccess"),
      )
      navigate(listPath)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("模型服务供应商保存失败", {
        provider_id: providerId,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, [
              "brand",
              "name",
              "apiKey",
              "apiUrl",
              "models",
            ])
          : t("modelServices.form.saveError"),
      )
    }
  }

  const title =
    mode === "create"
      ? t("modelServices.form.createTitle")
      : t("modelServices.form.editTitle")
  const modelErrorMessage = modelValidationMessage(form.formState.errors.models)
  const watchedModels = form.watch("models")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("modelServices.loading")}
          </LoadingIndicator>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("modelServices.form.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("modelServices.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full space-y-9"
            onSubmit={form.handleSubmit(save)}
            noValidate
          >
            <FieldGroup className="max-w-2xl">
              <Controller
                name="brand"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="model-provider-brand" required>
                      {t("modelServices.form.brand")}
                    </FieldLabel>
                    <NativeSelect
                      {...field}
                      id="model-provider-brand"
                      required
                      aria-invalid={fieldState.invalid}
                      onChange={(event) => {
                        const previousBrand = field.value as AIProviderBrandId
                        const nextBrand = event.target.value as AIProviderBrandId
                        const previous = aiProviderBrandConfigs[previousBrand]
                        const currentAPIURL = form.getValues("apiUrl")
                        field.onChange(nextBrand)
                        const next = aiProviderBrandConfigs[nextBrand]
                        if (
                          currentAPIURL === "" ||
                          currentAPIURL === previous.defaultAPIURL
                        ) {
                          form.setValue("apiUrl", next.defaultAPIURL, {
                            shouldDirty: true,
                            shouldValidate: true,
                          })
                        }
                        modelFields.replace([])
                      }}
                    >
                      {aiProviderBrandOrder.map((brand) => (
                        <option key={brand} value={brand}>
                          {t(aiProviderBrandConfigs[brand].nameKey)}
                        </option>
                      ))}
                    </NativeSelect>
                  </Field>
                )}
              />
              <FormInputField
                name="name"
                control={form.control}
                label={t("modelServices.form.name")}
                autoFocus={mode === "create"}
              />
              <FormInputField
                name="apiKey"
                control={form.control}
                label={t("modelServices.form.apiKey")}
                autoComplete="off"
                passwordVisibilityLabels={{
                  show: t("modelServices.form.showAPIKey"),
                  hide: t("modelServices.form.hideAPIKey"),
                }}
              />
              <FormInputField
                name="apiUrl"
                control={form.control}
                label={t("modelServices.form.apiUrl")}
                inputMode="url"
              />
            </FieldGroup>

            <section className="relative">
              <div className="mb-3 flex items-center justify-between gap-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                  {t("modelServices.models.title")}
                  <span aria-hidden="true" className="text-destructive">
                    *
                  </span>
                </h3>
                <div className="flex items-center gap-3">
                  <Button
                    type="button"
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    onClick={addCustomModel}
                  >
                    {t("modelServices.models.manualAdd")}
                  </Button>
                  <Button
                    type="button"
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    disabled={loadingModels}
                    onClick={() => void openModelDialog()}
                  >
                    {loadingModels
                      ? t("modelServices.models.loading")
                      : t("modelServices.models.fetch")}
                  </Button>
                </div>
              </div>
              <div className="overflow-hidden rounded-lg border bg-card">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead>{t("modelServices.models.columns.type")}</TableHead>
                      <TableHead>{t("modelServices.models.columns.identifier")}</TableHead>
                      <TableHead>{t("modelServices.models.columns.name")}</TableHead>
                      <TableHead>{t("modelServices.models.columns.inputModalities")}</TableHead>
                      <TableHead>{t("modelServices.models.columns.contextWindow")}</TableHead>
                      <TableHead>{t("modelServices.models.columns.maxOutputTokens")}</TableHead>
                      <TableHead className="text-right">
                        {t("modelServices.models.columns.actions")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {modelFields.fields.length === 0 ? (
                      <TableRow className="hover:bg-transparent">
                        <TableCell
                          colSpan={7}
                          className="h-24 text-center text-muted-foreground"
                        >
                          {t("modelServices.models.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      modelFields.fields.map((model, index) => {
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
                                  form.formState.errors.models?.[index]
                                    ?.contextWindow,
                                )}
                              />
                            </TableCell>
                            <TableCell>
                              {modelType === AIModelType.AIModelTypeChat ? (
                                <Input
                                  {...form.register(
                                    `models.${index}.maxOutputTokens`,
                                  )}
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
                            <TableCell className="text-right">
                              <Button
                                type="button"
                                variant="outline"
                                size="xs"
                                onClick={() => modelFields.remove(index)}
                              >
                                {t("modelServices.models.delete")}
                              </Button>
                            </TableCell>
                          </TableRow>
                        )
                      })
                    )}
                  </TableBody>
                </Table>
              </div>
              {/* 校验提示使用表单分区间距，不改变操作按钮位置。 */}
              <FormValidationMessage
                className="absolute top-full right-0 left-0 mt-2"
                message={modelErrorMessage}
              />
            </section>

            <div className="flex items-center gap-2">
              <Button
                type="submit"
                disabled={form.formState.isSubmitting || testingConnection}
              >
                {form.formState.isSubmitting ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {form.formState.isSubmitting
                  ? t("modelServices.form.saving")
                  : t("modelServices.form.save")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={testingConnection || form.formState.isSubmitting}
                onClick={() => void testConnection()}
              >
                {testingConnection ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {testingConnection
                  ? t("modelServices.form.testing")
                  : t("modelServices.form.test")}
              </Button>
              <Button type="button" variant="outline" asChild>
                <Link to={listPath}>{t("modelServices.form.cancel")}</Link>
              </Button>
            </div>
          </form>
        )}
      </PageContent>

      <Dialog open={modelDialogOpen} onOpenChange={setModelDialogOpen}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>{t("modelServices.models.dialogTitle")}</DialogTitle>
          </DialogHeader>
          <div className="max-h-[60vh] overflow-auto rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-12">
                    <span className="sr-only">
                      {t("modelServices.models.select")}
                    </span>
                  </TableHead>
                  <TableHead>{t("modelServices.models.columns.identifier")}</TableHead>
                  <TableHead>{t("modelServices.models.columns.type")}</TableHead>
                  <TableHead>{t("modelServices.models.columns.inputModalities")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {availableModels.map((model) => (
                  <TableRow key={model.identifier}>
                    <TableCell>
                      <input
                        type="checkbox"
                        className="size-4 accent-primary"
                        checked={draftModelIDs.has(model.identifier)}
                        onChange={(event) =>
                          toggleDraftModel(model.identifier, event.target.checked)
                        }
                        aria-label={t("modelServices.models.toggle", {
                          name: model.name,
                        })}
                      />
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {model.identifier}
                    </TableCell>
                    <TableCell>{t(modelTypeNameKeys[model.type])}</TableCell>
                    <TableCell>
                      {model.inputModalities
                        .map((modality) =>
                          t(modelInputModalityNameKeys[modality]),
                        )
                        .join("、")}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="flex items-center justify-between gap-3">
            <Button type="button" variant="link" onClick={clearDraftModels}>
              {t("modelServices.models.clearAll")}
            </Button>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => setModelDialogOpen(false)}
              >
                {t("modelServices.models.cancel")}
              </Button>
              <Button type="button" onClick={confirmModels}>
                {t("modelServices.models.confirm")}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
