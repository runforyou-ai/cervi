/** 连接器新增和编辑页。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  IntegrationConnectionType,
  createIntegrationConnection,
  getIntegrationConnection,
  isApiError,
  testIntegrationConnection,
  updateIntegrationConnection,
  type IntegrationConnectionInput,
  type IntegrationConnectionTypeId,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import {
  connectorTypeConfigs,
  connectorTypeOrder,
} from "@/features/integrations/connectors/connector-options"
import {
  createConnectorSchema,
  type ConnectorFormValues,
} from "@/features/integrations/connectors/connector-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const listPath = "/integrations/connectors"

/** 转换连接器表单值。 */
function connectorInput(values: ConnectorFormValues): IntegrationConnectionInput {
  return {
    type: values.type,
    name: values.name,
    description: values.description,
    configuration: {
      apiUrl: values.apiURL,
      apiKey: values.apiKey,
    },
  }
}

/** 创建或编辑连接器。 */
export function ConnectorFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const { connectionId = "" } = useParams()
  const [loading, setLoading] = useState(mode === "edit")
  const [loadError, setLoadError] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const initialType = IntegrationConnectionType.IntegrationConnectionTypeDify
  const schema = useMemo(
    () =>
      createConnectorSchema({
        typeInvalid: t("connectors.validation.typeInvalid"),
        nameRequired: t("connectors.validation.nameRequired"),
        nameTooLong: t("connectors.validation.nameTooLong"),
        descriptionTooLong: t("connectors.validation.descriptionTooLong"),
        apiURLRequired: t("connectors.validation.apiUrlRequired"),
        apiURLInvalid: t("connectors.validation.apiUrlInvalid"),
        apiKeyRequired: t("connectors.validation.apiKeyRequired"),
        apiKeyTooLong: t("connectors.validation.apiKeyTooLong"),
      }),
    [t],
  )
  const form = useForm<ConnectorFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      type: initialType,
      name: "",
      description: "",
      apiURL: connectorTypeConfigs[initialType].defaultAPIURL,
      apiKey: "",
    },
  })
  const selectedType = form.watch("type") as IntegrationConnectionTypeId
  const selectedTypeConfig = connectorTypeConfigs[selectedType]

  /** 读取待编辑的连接器。 */
  const load = useCallback(async () => {
    if (mode !== "edit") return
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const connection = await getIntegrationConnection(connectionId)
      if (version !== loadVersion.current) return
      form.reset({
        type: connection.type,
        name: connection.name,
        description: connection.description,
        apiURL: connection.configuration.apiUrl,
        apiKey: connection.configuration.apiKey,
      })
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("连接器详情加载失败", {
        connection_id: connectionId,
        error: requestError,
      })
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [connectionId, form, mode, navigate])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 保存连接器并返回列表。 */
  async function save(values: ConnectorFormValues) {
    if (saving) return
    setSaving(true)
    try {
      const input = connectorInput(values)
      if (mode === "create") {
        await createIntegrationConnection(input)
      } else {
        await updateIntegrationConnection(connectionId, input)
      }
      if (!mounted.current) return
      toast.success(
        mode === "create"
          ? t("connectors.form.createSuccess")
          : t("connectors.form.updateSuccess"),
      )
      navigate(listPath)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("连接器保存失败", {
        connection_id: connectionId,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, [
              "type",
              "name",
              "description",
              "configuration.apiUrl",
              "configuration.apiKey",
            ])
          : t("connectors.form.saveError"),
      )
    } finally {
      if (mounted.current) setSaving(false)
    }
  }

  /** 测试当前未保存的连接器配置。 */
  async function test(values: ConnectorFormValues) {
    if (testing) return
    setTesting(true)
    try {
      await testIntegrationConnection({
        type: values.type,
        configuration: {
          apiUrl: values.apiURL,
          apiKey: values.apiKey,
        },
      })
      if (!mounted.current) return
      toast.success(t("connectors.form.testSuccess"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("连接器测试失败", {
        connection_id: connectionId,
        connector_type: values.type,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, [
              "type",
              "configuration.apiUrl",
              "configuration.apiKey",
            ])
          : t("connectors.form.testError"),
      )
    } finally {
      if (mounted.current) setTesting(false)
    }
  }

  const title =
    mode === "create"
      ? t("connectors.form.createTitle")
      : t("connectors.form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("connectors.loading")}
          </div>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("connectors.form.loadError")}
            </p>
            <Button className="mt-4" variant="outline" onClick={load}>
              {t("connectors.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full max-w-2xl space-y-9"
            onSubmit={form.handleSubmit(save)}
            noValidate
          >
            <FieldGroup>
              <Controller
                name="type"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="connector-type" required>
                      {t("connectors.form.type")}
                    </FieldLabel>
                    <NativeSelect
                      {...field}
                      id="connector-type"
                      required
                      aria-invalid={fieldState.invalid}
                      onChange={(event) => {
                        const previousType = field.value as IntegrationConnectionTypeId
                        const nextType = event.target
                          .value as IntegrationConnectionTypeId
                        const currentAPIURL = form.getValues("apiURL")
                        field.onChange(nextType)
                        if (
                          currentAPIURL === "" ||
                          currentAPIURL ===
                            connectorTypeConfigs[previousType].defaultAPIURL
                        ) {
                          form.setValue(
                            "apiURL",
                            connectorTypeConfigs[nextType].defaultAPIURL,
                            { shouldDirty: true, shouldValidate: true },
                          )
                        }
                      }}
                    >
                      {connectorTypeOrder.map((connectorType) => (
                        <option key={connectorType} value={connectorType}>
                          {t(connectorTypeConfigs[connectorType].nameKey)}
                        </option>
                      ))}
                    </NativeSelect>
                  </Field>
                )}
              />
              <FormInputField
                name="name"
                control={form.control}
                label={t("connectors.form.name")}
                autoFocus={mode === "create"}
                maxLength={100}
              />
              <Controller
                name="description"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="connector-description" required={false}>
                      {t("connectors.form.description")}
                    </FieldLabel>
                    <Textarea
                      {...field}
                      id="connector-description"
                      rows={4}
                      maxLength={2000}
                      aria-invalid={fieldState.invalid}
                    />
                  </Field>
                )}
              />
              <FormInputField
                name="apiURL"
                control={form.control}
                label={t(selectedTypeConfig.apiURLLabelKey)}
                inputMode="url"
              />
              <FormInputField
                name="apiKey"
                control={form.control}
                label={t("connectors.form.apiKey")}
                autoComplete="off"
                maxLength={2048}
                passwordVisibilityLabels={{
                  show: t("connectors.form.showAPIKey"),
                  hide: t("connectors.form.hideAPIKey"),
                }}
              />
            </FieldGroup>
            <div className="flex items-center gap-2">
              <Button type="submit" disabled={saving || testing}>
                {saving ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {saving
                  ? t("connectors.form.saving")
                  : t("connectors.form.save")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={testing || saving}
                onClick={form.handleSubmit(test)}
              >
                {testing ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {testing
                  ? t("connectors.form.testing")
                  : t("connectors.form.test")}
              </Button>
              <Button type="button" variant="outline" asChild>
                <Link to={listPath}>{t("connectors.form.cancel")}</Link>
              </Button>
            </div>
          </form>
        )}
      </PageContent>
    </div>
  )
}
