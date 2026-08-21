/** AI 供应商新建和编辑页。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  AIProviderBrand,
  createAIProvider,
  getAIProvider,
  isApiError,
  listAvailableAIModels,
  updateAIProvider,
  type AIProviderModel,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
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
  createAIProviderSchema,
  parseTokenCount,
  type AIProviderFormValues,
} from "@/features/settings/ai-provider-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const defaultAPIURL = "https://api.deepseek.com"

/** 把模型 Token 数转换为紧凑显示值。 */
function formatTokenCount(value: number) {
  if (value % 1_048_576 === 0) return `${value / 1_048_576}M`
  if (value % 1024 === 0) return `${value / 1024}K`
  return String(value)
}

/** 显示供应商资料、连接设置和模型目录表单。 */
export function AIProviderFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const { providerId = "" } = useParams()
  const [loading, setLoading] = useState(mode === "edit")
  const [loadError, setLoadError] = useState(false)
  const [modelDialogOpen, setModelDialogOpen] = useState(false)
  const [availableModels, setAvailableModels] = useState<AIProviderModel[]>([])
  const [modelSelectionInitialized, setModelSelectionInitialized] =
    useState(false)
  const [draftModelIDs, setDraftModelIDs] = useState<Set<string>>(new Set())
  const [loadingModels, setLoadingModels] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const formElement = useRef<HTMLFormElement>(null)
  const schema = useMemo(
    () =>
      createAIProviderSchema({
        brandInvalid: t("aiProviders.validation.brandInvalid"),
        nameRequired: t("aiProviders.validation.nameRequired"),
        nameTooLong: t("aiProviders.validation.nameTooLong"),
        apiKeyRequired: t("aiProviders.validation.apiKeyRequired"),
        apiKeyTooLong: t("aiProviders.validation.apiKeyTooLong"),
        apiUrlRequired: t("aiProviders.validation.apiUrlRequired"),
        apiUrlInvalid: t("aiProviders.validation.apiUrlInvalid"),
        modelIdentifierRequired: t(
          "aiProviders.validation.modelIdentifierRequired",
        ),
        modelIdentifierTooLong: t(
          "aiProviders.validation.modelIdentifierTooLong",
        ),
        modelNameRequired: t("aiProviders.validation.modelNameRequired"),
        modelNameTooLong: t("aiProviders.validation.modelNameTooLong"),
        contextWindowInvalid: t("aiProviders.validation.contextWindowInvalid"),
        maxOutputTokensInvalid: t(
          "aiProviders.validation.maxOutputTokensInvalid",
        ),
        modelIdentifierDuplicate: t(
          "aiProviders.validation.modelIdentifierDuplicate",
        ),
      }),
    [t],
  )
  const form = useForm<AIProviderFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      brand: AIProviderBrand.AIProviderBrandDeepSeek,
      name: "",
      apiKey: "",
      apiUrl: defaultAPIURL,
      models: [],
    },
  })
  const modelFields = useFieldArray({
    control: form.control,
    name: "models",
  })

  /** 读取待编辑的 AI 供应商。 */
  const loadProvider = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const provider = await getAIProvider(providerId)
      if (version !== loadVersion.current) return
      form.reset({
        brand: provider.brand,
        name: provider.name,
        apiKey: provider.apiKey,
        apiUrl: provider.apiUrl,
        models: provider.models.map((model) => ({
          identifier: model.identifier,
          name: model.name,
          contextWindow: formatTokenCount(model.contextWindow),
          maxOutputTokens: formatTokenCount(model.maxOutputTokens),
        })),
      })
      setModelSelectionInitialized(true)
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("AI 供应商详情加载失败", {
        provider_id: providerId,
        error: requestError,
      })
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [form, navigate, providerId])

  useEffect(() => {
    mounted.current = true
    if (mode === "edit") void loadProvider()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [loadProvider, mode])

  /** 获取当前品牌的可用模型并打开选择弹窗。 */
  async function openModelDialog() {
    if (loadingModels) return
    setLoadingModels(true)
    try {
      const models = await listAvailableAIModels(form.getValues("brand"))
      if (!mounted.current) return
      const current = form.getValues("models")
      const catalogIDs = new Set(models.map((model) => model.identifier))
      const selectedCatalogIDs = current
        .filter((model) => catalogIDs.has(model.identifier))
        .map((model) => model.identifier)
      setAvailableModels(models)
      setDraftModelIDs(
        new Set(
          modelSelectionInitialized
            ? selectedCatalogIDs
            : models.map((model) => model.identifier),
        ),
      )
      setModelDialogOpen(true)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("AI 可用模型加载失败", requestError)
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, ["brand"])
          : t("aiProviders.models.loadError"),
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

  /** 确认模型选择并更新页面模型目录。 */
  function confirmModels() {
    const current = form.getValues("models")
    const catalogIDs = new Set(availableModels.map((model) => model.identifier))
    const customModels = current.filter(
      (model) => !catalogIDs.has(model.identifier),
    )
    const presetModels = availableModels
      .filter((model) => draftModelIDs.has(model.identifier))
      .map((model) => {
        const existing = current.find(
          (item) => item.identifier === model.identifier,
        )
        return (
          existing ?? {
            identifier: model.identifier,
            name: model.name,
            contextWindow: formatTokenCount(model.contextWindow),
            maxOutputTokens: formatTokenCount(model.maxOutputTokens),
          }
        )
      })
    modelFields.replace([...customModels, ...presetModels])
    setModelSelectionInitialized(true)
    setModelDialogOpen(false)
  }

  /** 添加一个空白自定义模型。 */
  function addCustomModel() {
    modelFields.append({
      identifier: "",
      name: "",
      contextWindow: "",
      maxOutputTokens: "",
    })
  }

  /** 创建或保存 AI 供应商。 */
  async function save(values: AIProviderFormValues) {
    const input = {
      brand: values.brand,
      name: values.name,
      apiKey: values.apiKey,
      apiUrl: values.apiUrl,
      models: values.models.map((model) => ({
        identifier: model.identifier,
        name: model.name,
        contextWindow: parseTokenCount(model.contextWindow)!,
        maxOutputTokens: parseTokenCount(model.maxOutputTokens)!,
      })),
    }
    try {
      if (mode === "create") {
        await createAIProvider(input)
      } else {
        await updateAIProvider(providerId, input)
      }
      if (!mounted.current) return
      toast.success(
        mode === "create"
          ? t("aiProviders.form.createSuccess")
          : t("aiProviders.form.updateSuccess"),
      )
      navigate("/settings/ai-providers")
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("AI 供应商保存失败", {
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
          : t("aiProviders.form.saveError"),
      )
    }
  }

  /** 显示第一个无效输入框的浏览器原生提示。 */
  function showValidationError() {
    formElement.current?.reportValidity()
  }

  const title =
    mode === "create"
      ? t("aiProviders.form.createTitle")
      : t("aiProviders.form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("aiProviders.loading")}
          </div>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("aiProviders.form.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void loadProvider()}
            >
              {t("aiProviders.retry")}
            </Button>
          </div>
        ) : (
          <form
            ref={formElement}
            className="w-full max-w-3xl space-y-8"
            onSubmit={form.handleSubmit(save, showValidationError)}
            noValidate
          >
            <FieldGroup>
              <Controller
                name="brand"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="ai-provider-brand" required>
                      {t("aiProviders.form.brand")}
                    </FieldLabel>
                    <NativeSelect
                      {...field}
                      id="ai-provider-brand"
                      required
                      aria-invalid={fieldState.invalid}
                    >
                      <option value={AIProviderBrand.AIProviderBrandDeepSeek}>
                        {t("aiProviders.brands.deepseek")}
                      </option>
                    </NativeSelect>
                  </Field>
                )}
              />
              <FormInputField
                name="name"
                control={form.control}
                label={t("aiProviders.form.name")}
                autoFocus={mode === "create"}
              />
              <FormInputField
                name="apiKey"
                control={form.control}
                label={t("aiProviders.form.apiKey")}
                autoComplete="off"
                passwordVisibilityLabels={{
                  show: t("aiProviders.form.showAPIKey"),
                  hide: t("aiProviders.form.hideAPIKey"),
                }}
              />
            </FieldGroup>

            <FormInputField
              name="apiUrl"
              control={form.control}
              label={t("aiProviders.form.apiUrl")}
              inputMode="url"
            />

            <section>
              <div className="mb-3 flex items-center justify-between gap-3">
                <h3 className="text-sm font-medium">
                  {t("aiProviders.models.title")}
                </h3>
                <div className="flex items-center gap-3">
                  <Button
                    type="button"
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    onClick={addCustomModel}
                  >
                    {t("aiProviders.models.manualAdd")}
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
                      ? t("aiProviders.models.loading")
                      : t("aiProviders.models.fetch")}
                  </Button>
                </div>
              </div>
              <div className="overflow-x-auto rounded-lg border bg-card">
                <Table>
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      <TableHead>
                        {t("aiProviders.models.columns.identifier")}
                      </TableHead>
                      <TableHead>
                        {t("aiProviders.models.columns.name")}
                      </TableHead>
                      <TableHead>
                        {t("aiProviders.models.columns.contextWindow")}
                      </TableHead>
                      <TableHead>
                        {t("aiProviders.models.columns.maxOutputTokens")}
                      </TableHead>
                      <TableHead className="text-right">
                        {t("aiProviders.models.columns.actions")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {modelFields.fields.length === 0 ? (
                      <TableRow className="hover:bg-transparent">
                        <TableCell
                          colSpan={5}
                          className="h-24 text-center text-muted-foreground"
                        >
                          {t("aiProviders.models.empty")}
                        </TableCell>
                      </TableRow>
                    ) : (
                      modelFields.fields.map((model, index) => (
                        <TableRow key={model.id}>
                          <TableCell>
                            <Input
                              {...form.register(`models.${index}.identifier`)}
                              required
                              maxLength={200}
                              autoComplete="off"
                              className="min-w-40 font-mono text-xs"
                              aria-label={t("aiProviders.models.editField", {
                                field: t(
                                  "aiProviders.models.columns.identifier",
                                ),
                                row: index + 1,
                              })}
                              aria-invalid={Boolean(
                                form.formState.errors.models?.[index]
                                  ?.identifier,
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
                              aria-label={t("aiProviders.models.editField", {
                                field: t("aiProviders.models.columns.name"),
                                row: index + 1,
                              })}
                              aria-invalid={Boolean(
                                form.formState.errors.models?.[index]?.name,
                              )}
                            />
                          </TableCell>
                          <TableCell>
                            <Input
                              {...form.register(
                                `models.${index}.contextWindow`,
                              )}
                              required
                              inputMode="decimal"
                              autoComplete="off"
                              className="min-w-28"
                              aria-label={t("aiProviders.models.editField", {
                                field: t(
                                  "aiProviders.models.columns.contextWindow",
                                ),
                                row: index + 1,
                              })}
                              aria-invalid={Boolean(
                                form.formState.errors.models?.[index]
                                  ?.contextWindow,
                              )}
                            />
                          </TableCell>
                          <TableCell>
                            <Input
                              {...form.register(
                                `models.${index}.maxOutputTokens`,
                              )}
                              required
                              inputMode="decimal"
                              autoComplete="off"
                              className="min-w-28"
                              aria-label={t("aiProviders.models.editField", {
                                field: t(
                                  "aiProviders.models.columns.maxOutputTokens",
                                ),
                                row: index + 1,
                              })}
                              aria-invalid={Boolean(
                                form.formState.errors.models?.[index]
                                  ?.maxOutputTokens,
                              )}
                            />
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              type="button"
                              variant="outline"
                              size="xs"
                              onClick={() => modelFields.remove(index)}
                            >
                              {t("aiProviders.models.delete")}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </section>

            <div className="flex items-center gap-2">
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {form.formState.isSubmitting
                  ? t("aiProviders.form.saving")
                  : t("aiProviders.form.save")}
              </Button>
              <Button type="button" variant="outline" asChild>
                <Link to="/settings/ai-providers">
                  {t("aiProviders.form.cancel")}
                </Link>
              </Button>
            </div>
          </form>
        )}
      </PageContent>

      <Dialog open={modelDialogOpen} onOpenChange={setModelDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("aiProviders.models.dialogTitle")}</DialogTitle>
          </DialogHeader>
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-12">
                    <span className="sr-only">
                      {t("aiProviders.models.select")}
                    </span>
                  </TableHead>
                  <TableHead>
                    {t("aiProviders.models.columns.identifier")}
                  </TableHead>
                  <TableHead>{t("aiProviders.models.columns.name")}</TableHead>
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
                        aria-label={t("aiProviders.models.toggle", {
                          name: model.name,
                        })}
                        onChange={(event) =>
                          toggleDraftModel(
                            model.identifier,
                            event.target.checked,
                          )
                        }
                      />
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {model.identifier}
                    </TableCell>
                    <TableCell>{model.name}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              disabled={draftModelIDs.size === 0}
              onClick={clearDraftModels}
            >
              {t("aiProviders.models.clearAll")}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => setModelDialogOpen(false)}
            >
              {t("aiProviders.models.cancel")}
            </Button>
            <Button type="button" onClick={confirmModels}>
              {t("aiProviders.models.confirm")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
