/** 模型服务供应商表单页。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import {
  Controller,
  type FieldErrors,
  useFieldArray,
  useForm,
  useWatch,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  AIModelInputModality,
  AIModelType,
  AIProviderBrand,
  AIProviderCredentialType,
  createAIProvider,
  getAIProvider,
  isApiError,
  listAIProviders,
  testAIProviderConnection,
  updateAIProvider,
  type AIProviderBrandId,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { FormValidationMessage } from "@/components/form/form-validation-message"
import { ResourceContent } from "@/components/resource-content"
import { PageContent } from "@/components/page-content"
import { PageBackButton } from "@/components/page-back-button"
import { PageHeader } from "@/components/page-header"
import { ProfileAvatar } from "@/components/profile-avatar"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  ModelEditDialog,
  useAIModelSchemaMessages,
} from "@/features/integrations/model-services/model-edit-dialog"
import { ModelPickerDialog } from "@/features/integrations/model-services/model-picker-dialog"
import {
  aiProviderBrandConfigs,
  aiProviderBrandOrder,
} from "@/features/integrations/model-services/model-provider-brands"
import { ModelProviderModelList } from "@/features/integrations/model-services/model-provider-model-list"
import {
  modelFormValue,
  rollbackModels,
} from "@/features/integrations/model-services/model-provider-model-values"
import {
  createAIProviderSchema,
  parseTokenCount,
  type AIModelFormValues,
  type AIProviderFormValues,
} from "@/features/integrations/model-services/model-provider-schema"
import { useFormSave } from "@/hooks/use-form-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 返回模型目录中的第一条字段校验提示。 */
function modelValidationMessage(errors: FieldErrors<AIProviderFormValues>["models"]) {
  if (!errors) return ""
  if (typeof errors.message === "string") return errors.message
  if (typeof errors.root?.message === "string") return errors.root.message
  if (!Array.isArray(errors)) return ""
  for (const model of errors) {
    if (!model) continue
    const error =
      model.identifier ?? model.name ?? model.contextWindow ?? model.maxOutputTokens
    if (typeof error?.message === "string") return error.message
  }
  return ""
}

const listPath = "/settings/model-services"

/** 编辑供应商连接和模型目录；新建时品牌取自路由，品牌创建后不可修改。 */
export function ModelProviderFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const { providerId = "", brand: routeBrand = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const [testingConnection, setTestingConnection] = useState(false)
  const [editingModel, setEditingModel] = useState<{
    index: number | null
    model: AIModelFormValues
  } | null>(null)
  const [removingModel, setRemovingModel] = useState<number | null>(null)
  const createBrand = aiProviderBrandOrder.find((brand) => brand === routeBrand) ?? null
  const initialBrand = createBrand ?? AIProviderBrand.AIProviderBrandDeepSeek
  const modelMessages = useAIModelSchemaMessages()
  const schema = useMemo(
    () =>
      createAIProviderSchema({
        ...modelMessages,
        brandInvalid: t("modelServices.validation.brandInvalid"),
        credentialTypeInvalid: t("modelServices.validation.credentialTypeInvalid"),
        nameRequired: t("modelServices.validation.nameRequired"),
        nameTooLong: t("modelServices.validation.nameTooLong"),
        apiKeyRequired: t("modelServices.validation.apiKeyRequired"),
        apiKeyTooLong: t("modelServices.validation.apiKeyTooLong"),
        apiUrlRequired: t("modelServices.validation.apiUrlRequired"),
        apiUrlInvalid: t("modelServices.validation.apiUrlInvalid"),
        modelsRequired: t("modelServices.validation.modelsRequired"),
      }),
    [t, modelMessages],
  )
  const form = useForm<AIProviderFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    // 编辑时离开字段即校验以便自动保存；新建时等提交再校验，避免原生校验把焦点锁在必填项上。
    mode: mode === "edit" ? "onBlur" : "onSubmit",
    defaultValues: {
      brand: initialBrand,
      name: t(aiProviderBrandConfigs[initialBrand].nameKey),
      credentialType: AIProviderCredentialType.AIProviderCredentialTypeAPIKey,
      apiKey: "",
      apiUrl: aiProviderBrandConfigs[initialBrand].defaultAPIURL,
      models: [],
    },
  })
  const modelFields = useFieldArray({
    control: form.control,
    name: "models",
  })

  const detail = useResource(resourceKeys.aiProvider(providerId), () => getAIProvider(providerId), {
    enabled: mode === "edit",
  })
  const provider = detail.data
  // 服务端最近一次确认的模型目录，目录改动被服务端拒绝时恢复到该目录。
  const savedModels = useRef<AIModelFormValues[]>([])
  const providers = useResource(resourceKeys.aiProviders(), () => listAIProviders(), {
    enabled: mode === "create",
  })

  /** 新建时把默认名称设为不与已有供应商重名的品牌名。 */
  useEffect(() => {
    if (mode !== "create" || !providers.data || form.getFieldState("name").isDirty) return
    const brandName = t(aiProviderBrandConfigs[initialBrand].nameKey)
    const taken = new Set(
      providers.data.providers.map((item) => item.name.trim().toLocaleLowerCase()),
    )
    let name = brandName
    for (let suffix = 2; taken.has(name.toLocaleLowerCase()); suffix += 1) {
      name = `${brandName} ${suffix}`
    }
    form.setValue("name", name)
  }, [form, initialBrand, mode, providers.data, t])

  /** 详情就绪后回填供应商表单。 */
  useEffect(() => {
    if (!provider) return
    savedModels.current = provider.models.map(modelFormValue)
    form.reset({
      brand: provider.brand,
      name: provider.name,
      credentialType: provider.credentialType,
      apiKey: provider.apiKey,
      apiUrl: provider.apiUrl,
      models: provider.models.map(modelFormValue),
    })
  }, [form, provider])

  const watchedModels = useWatch({ control: form.control, name: "models" })
  // 编辑中的模型不与自身标识冲突。
  const takenIdentifiers = useMemo(
    () =>
      new Set(
        watchedModels
          .filter((_, index) => index !== editingModel?.index)
          .map((model) => model.identifier.trim()),
      ),
    [watchedModels, editingModel],
  )

  /** 打开弹窗添加一个文本输入的自定义对话模型。 */
  function addCustomModel() {
    setEditingModel({
      index: null,
      model: {
        identifier: "",
        name: "",
        type: AIModelType.AIModelTypeChat,
        inputModalities: [AIModelInputModality.AIModelInputModalityText],
        contextWindow: "",
        maxOutputTokens: "",
      },
    })
  }

  /** 修改模型目录；编辑已保存的供应商时立即校验目录，不合法时显示提示并等待补全后自动保存。 */
  function changeModels(change: () => void) {
    change()
    if (mode === "edit") void form.trigger("models")
  }

  /** 保存弹窗中的模型：新增时追加，编辑时替换原位置。 */
  function saveModel(model: AIModelFormValues) {
    const index = editingModel?.index ?? null
    changeModels(() =>
      index === null ? modelFields.append(model) : modelFields.update(index, model),
    )
    setEditingModel(null)
  }

  /** 移除模型；已保存的供应商在确认后移除。 */
  function removeModel(index: number) {
    if (mode === "edit") setRemovingModel(index)
    else modelFields.remove(index)
  }

  /** 使用当前未保存的地址和密钥测试模型服务连接。 */
  async function testConnection() {
    if (testingConnection || form.formState.isSubmitting) return
    const valid = await form.trigger(["brand", "credentialType", "apiKey", "apiUrl"], {
      shouldFocus: true,
    })
    if (!valid || !mounted.current) return
    const { brand, credentialType, apiKey, apiUrl } = form.getValues()
    setTestingConnection(true)
    try {
      await testAIProviderConnection({ brand, credentialType, apiKey, apiUrl })
      if (!mounted.current) return
      toast.success(t("modelServices.form.testSuccess"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("模型服务连接测试失败", { brand, error: requestError })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, [
              "brand",
              "credentialType",
              "apiKey",
              "apiUrl",
            ])
          : t("modelServices.form.testError"),
      )
    } finally {
      if (mounted.current) setTestingConnection(false)
    }
  }

  /** 创建或保存模型服务供应商。 */
  // 编辑已有供应商时边改边存，新建仍由底部按钮提交并跳回列表。
  const { submit, mounted } = useFormSave({
    form,
    schema,
    autoSave: mode === "edit",
    save: async (values) => {
      const input = {
        name: values.name,
        credentialType: values.credentialType,
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
      if (mode === "create") {
        await createAIProvider({ ...input, brand: values.brand })
      } else {
        try {
          await updateAIProvider(providerId, input)
        } catch (error) {
          // 服务端拒绝模型目录时撤回本次请求的目录改动，其余字段和请求后的编辑保留。
          if (isApiError(error) && error.fields.models) {
            modelFields.replace(
              rollbackModels(savedModels.current, values.models, form.getValues("models")),
            )
          }
          throw error
        }
        savedModels.current = values.models
        void invalidateResource(resourceKeys.aiProvider(providerId))
      }
      void invalidateResource(resourceKeys.aiProviders())
    },
    onSubmitted: () => {
      toast.success(t("modelServices.form.createSuccess"))
      navigate(listPath)
    },
    // 已保存的供应商目录不合法时暂不保存，离开页面前提示未保存的改动。
    unsaved: mode === "edit" && Boolean(form.formState.errors.models),
    errorMessage: t("modelServices.form.saveError"),
    errorFields: ["brand", "name", "credentialType", "apiKey", "apiUrl", "models"],
    logLabel: "模型服务供应商保存",
  })

  const title =
    mode === "create"
      ? t("modelServices.form.createTitle")
      : t("modelServices.form.editTitle")
  const modelErrorMessage = modelValidationMessage(form.formState.errors.models)
  const watchedBrand = form.watch("brand") as AIProviderBrandId
  const brandConfig = aiProviderBrandConfigs[watchedBrand]
  const brandName = t(brandConfig.nameKey)
  const usesAPIKey =
    form.watch("credentialType") ===
    AIProviderCredentialType.AIProviderCredentialTypeAPIKey

  if (mode === "create" && !createBrand) return <Navigate to={listPath} replace />

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={title}
        description={t(
          mode === "create"
            ? "modelServices.form.createDescription"
            : "modelServices.form.editDescription",
        )}
      >
        {mode === "edit" ? <PageBackButton to={listPath} /> : null}
      </PageHeader>
      <PageContent variant="form">
        <ResourceContent
          resources={mode === "edit" ? detail : []}
          errorMessage={t("modelServices.form.loadError")}
        >
          <form
            className="w-full space-y-9"
            onSubmit={form.handleSubmit(submit)}
            noValidate
          >
            <FieldGroup>
              <Field>
                <FieldLabel>{t("modelServices.form.brand")}</FieldLabel>
                <div className="flex h-9 items-center gap-2 text-sm">
                  <ProfileAvatar name={brandName} className="size-6 rounded-md text-xs" />
                  {brandName}
                </div>
              </Field>
              <FormInputField
                name="name"
                control={form.control}
                label={t("modelServices.form.name")}
                autoFocus={mode === "create"}
              />
              {brandConfig.supportsNoCredential ? (
                <Controller
                  name="credentialType"
                  control={form.control}
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor="model-provider-credential-type" required>
                        {t("modelServices.form.credentialType")}
                      </FieldLabel>
                      <NativeSelect
                        {...field}
                        id="model-provider-credential-type"
                        required
                        aria-invalid={fieldState.invalid}
                        onChange={(event) => {
                          const next = event.target
                            .value as AIProviderFormValues["credentialType"]
                          field.onChange(next)
                          // 不需要凭据的服务不保留已填写的密钥。
                          if (
                            next ===
                            AIProviderCredentialType.AIProviderCredentialTypeNone
                          ) {
                            form.setValue("apiKey", "", { shouldDirty: true })
                          }
                        }}
                      >
                        <option
                          value={
                            AIProviderCredentialType.AIProviderCredentialTypeAPIKey
                          }
                        >
                          {t("modelServices.form.credentialTypes.apiKey")}
                        </option>
                        <option
                          value={AIProviderCredentialType.AIProviderCredentialTypeNone}
                        >
                          {t("modelServices.form.credentialTypes.none")}
                        </option>
                      </NativeSelect>
                    </Field>
                  )}
                />
              ) : null}
              {usesAPIKey ? (
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
              ) : null}
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
                  <ModelPickerDialog
                    form={form}
                    onAppend={(models) => changeModels(() => modelFields.append(models))}
                  />
                </div>
              </div>
              <ModelProviderModelList
                models={modelFields.fields.map((field, index) => ({
                  key: field.id,
                  model: watchedModels[index] ?? field,
                }))}
                removable={mode === "create" || modelFields.fields.length > 1}
                onEdit={(index) =>
                  setEditingModel({ index, model: form.getValues(`models.${index}`) })
                }
                onRemove={removeModel}
              />
              {/* 校验提示使用表单分区间距，不改变操作按钮位置。 */}
              <FormValidationMessage
                className="absolute top-full right-0 left-0 mt-2"
                message={modelErrorMessage}
              />
            </section>

            <FormActions
              saving={form.formState.isSubmitting}
              disabled={testingConnection}
              cancelTo={listPath}
              submit={mode === "create"}
            >
              <Button
                type="button"
                variant="outline"
                disabled={testingConnection || form.formState.isSubmitting}
                onClick={() => void testConnection()}
              >
                {testingConnection ? <LoaderCircleIcon className="animate-spin" /> : null}
                {testingConnection
                  ? t("modelServices.form.testing")
                  : t("modelServices.form.test")}
              </Button>
            </FormActions>
          </form>
        </ResourceContent>
      </PageContent>

      <ModelEditDialog
        editing={
          editingModel
            ? { creating: editingModel.index === null, model: editingModel.model }
            : null
        }
        takenIdentifiers={takenIdentifiers}
        onOpenChange={(open) => !open && setEditingModel(null)}
        onSave={saveModel}
      />
      <ConfirmationDialog
        open={removingModel !== null}
        pending={false}
        title={
          removingModel !== null
            ? t("modelServices.models.removeTitle", {
                name: watchedModels[removingModel]?.name ?? "",
              })
            : ""
        }
        description={t("modelServices.models.removeDescription")}
        onOpenChange={(open) => !open && setRemovingModel(null)}
        onConfirm={() => {
          if (removingModel !== null) changeModels(() => modelFields.remove(removingModel))
          setRemovingModel(null)
        }}
      />
    </div>
  )
}
