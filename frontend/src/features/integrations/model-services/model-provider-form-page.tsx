/** 模型服务供应商表单页。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, type FieldErrors, useFieldArray, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  AIModelInputModality,
  AIModelType,
  AIProviderCredentialType,
  createAIProvider,
  getAIProvider,
  isApiError,
  testAIProviderConnection,
  updateAIProvider,
  type AIProviderBrandId,
} from "@/api"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { FormValidationMessage } from "@/components/form/form-validation-message"
import { ResourceContent } from "@/components/resource-content"
import { PageContent } from "@/components/page-content"
import { PageBackButton } from "@/components/page-back-button"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  aiProviderBrandConfigs,
  aiProviderBrandOrder,
} from "@/features/integrations/model-services/model-provider-brands"
import { ModelPickerDialog } from "@/features/integrations/model-services/model-picker-dialog"
import { modelFormValue } from "@/features/integrations/model-services/model-provider-model-values"
import { ModelProviderModelsTable } from "@/features/integrations/model-services/model-provider-models-table"
import {
  modelServiceSectionConfigs,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import {
  createAIProviderSchema,
  parseTokenCount,
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

/** 编辑供应商连接和模型目录。 */
export function ModelProviderFormPage({
  mode,
  returnSection,
}: {
  mode: "create" | "edit"
  returnSection: ModelServiceSection
}) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const { providerId = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const [testingConnection, setTestingConnection] = useState(false)
  const listPath = `/settings/model-services/${returnSection}`
  const initialBrand = modelServiceSectionConfigs[returnSection].defaultBrand
  const schema = useMemo(
    () =>
      createAIProviderSchema({
        brandInvalid: t("modelServices.validation.brandInvalid"),
        credentialTypeInvalid: t("modelServices.validation.credentialTypeInvalid"),
        nameRequired: t("modelServices.validation.nameRequired"),
        nameTooLong: t("modelServices.validation.nameTooLong"),
        apiKeyRequired: t("modelServices.validation.apiKeyRequired"),
        apiKeyTooLong: t("modelServices.validation.apiKeyTooLong"),
        apiUrlRequired: t("modelServices.validation.apiUrlRequired"),
        apiUrlInvalid: t("modelServices.validation.apiUrlInvalid"),
        modelIdentifierRequired: t("modelServices.validation.modelIdentifierRequired"),
        modelIdentifierTooLong: t("modelServices.validation.modelIdentifierTooLong"),
        modelNameRequired: t("modelServices.validation.modelNameRequired"),
        modelNameTooLong: t("modelServices.validation.modelNameTooLong"),
        modelTypeInvalid: t("modelServices.validation.modelTypeInvalid"),
        inputModalityInvalid: t("modelServices.validation.inputModalityInvalid"),
        inputModalitiesRequired: t("modelServices.validation.inputModalitiesRequired"),
        contextWindowInvalid: t("modelServices.validation.contextWindowInvalid"),
        maxOutputTokensInvalid: t("modelServices.validation.maxOutputTokensInvalid"),
        modelIdentifierDuplicate: t("modelServices.validation.modelIdentifierDuplicate"),
        modelsRequired: t("modelServices.validation.modelsRequired"),
      }),
    [t],
  )
  const form = useForm<AIProviderFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    // 编辑时离开字段即校验以便自动保存；新建时等提交再校验，避免原生校验把焦点锁在必填项上。
    mode: mode === "edit" ? "onBlur" : "onSubmit",
    defaultValues: {
      brand: initialBrand,
      name: "",
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

  /** 详情就绪后回填供应商表单。 */
  useEffect(() => {
    if (!provider) return
    form.reset({
      brand: provider.brand,
      name: provider.name,
      credentialType: provider.credentialType,
      apiKey: provider.apiKey,
      apiUrl: provider.apiUrl,
      models: provider.models.map(modelFormValue),
    })
  }, [form, provider])

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
        brand: values.brand,
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
        await createAIProvider(input)
      } else {
        await updateAIProvider(providerId, input)
        void invalidateResource(resourceKeys.aiProvider(providerId))
      }
      void invalidateResource(resourceKeys.aiProviders())
    },
    onSubmitted: () => {
      toast.success(t("modelServices.form.createSuccess"))
      navigate(listPath)
    },
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
  const usesAPIKey =
    form.watch("credentialType") ===
    AIProviderCredentialType.AIProviderCredentialTypeAPIKey

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
                        // 只有自建或本机部署的服务可以不配置凭据。
                        if (!next.supportsNoCredential) {
                          form.setValue(
                            "credentialType",
                            AIProviderCredentialType.AIProviderCredentialTypeAPIKey,
                            { shouldDirty: true, shouldValidate: true },
                          )
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
                    onAppend={(models) => modelFields.append(models)}
                  />
                </div>
              </div>
              <ModelProviderModelsTable
                form={form}
                fields={modelFields.fields}
                onRemove={modelFields.remove}
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
    </div>
  )
}
