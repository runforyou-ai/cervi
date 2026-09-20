/** MCP 服务新增与编辑页。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  MCPServerType,
  createMCPServer,
  getMCPServer,
  isApiError,
  updateMCPServer,
  testMCPServerConnection,
} from "@/api"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { ResourceContent } from "@/components/resource-content"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import {
  createMCPServerSchema,
  type MCPServerFormValues,
} from "@/features/integrations/mcp-servers/mcp-server-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const listPath = "/settings/mcp-servers"

/** 编辑 MCP 服务名称、地址、服务器类型和认证令牌。 */
export function MCPServerFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const { mcpServerId = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const mounted = useRef(true)
  const [testing, setTesting] = useState(false)
  const schema = useMemo(
    () =>
      createMCPServerSchema({
        nameRequired: t("mcpServer.validation.nameRequired"),
        nameTooLong: t("mcpServer.validation.nameTooLong"),
        serverTypeInvalid: t("mcpServer.validation.serverTypeInvalid"),
        urlRequired: t("mcpServer.validation.urlRequired"),
        urlTooLong: t("mcpServer.validation.urlTooLong"),
        urlInvalid: t("mcpServer.validation.urlInvalid"),
      }),
    [t],
  )
  const form = useForm<MCPServerFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      name: "",
      url: "",
      serverType: MCPServerType.MCPServerTypeStreamableHTTP,
      authorizationToken: "",
    },
  })
  const {
    data: mcpServer,
    loading: detailLoading,
    refreshing: detailRefreshing,
    error: detailError,
    refresh,
  } = useResource(resourceKeys.mcpServer(mcpServerId), () => getMCPServer(mcpServerId), {
    enabled: mode === "edit",
  })
  const loading =
    mode === "edit" && (detailLoading || (Boolean(detailError) && detailRefreshing))
  const loadError = mode === "edit" && Boolean(detailError) && !loading

  /** 详情就绪后回填 MCP 服务表单。 */
  useEffect(() => {
    if (!mcpServer) return
    form.reset({
      name: mcpServer.name,
      url: mcpServer.url,
      serverType: mcpServer.serverType,
      authorizationToken: mcpServer.authorizationToken,
    })
  }, [mcpServer, form])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 测试当前未保存的连接配置。 */
  async function testConnection() {
    if (testing || form.formState.isSubmitting) return
    if (!(await form.trigger(["url", "serverType", "authorizationToken"], { shouldFocus: true }))) return
    setTesting(true)
    try {
      const { url, serverType, authorizationToken } = form.getValues()
      await testMCPServerConnection({ url, serverType, authorizationToken })
      if (mounted.current) toast.success(t("mcpServer.connection.success"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("mcpServer.connection.error"))
    } finally {
      if (mounted.current) setTesting(false)
    }
  }

  /** 创建或保存 MCP 服务，并由服务端提交工具更新任务。 */
  // 编辑已有服务时边改边存，新建仍由底部按钮提交并跳回列表。
  const markSaved = useAutoSave({
    form,
    schema,
    enabled: mode === "edit",
    save: (values) => save(values, true),
  })

  async function save(values: MCPServerFormValues, autoSaved = false) {
    if (testing) return
    try {
      await (mode === "create"
        ? createMCPServer(values)
        : updateMCPServer(mcpServerId, values))
      if (mode === "edit") {
        void invalidateResource(resourceKeys.mcpServer(mcpServerId))
      }
      void invalidateResource(resourceKeys.mcpServers())
      void invalidateResource(resourceKeys.agentMCPServerOptions())
      if (!mounted.current) return
      if (autoSaved) {
        markSaved(values)
        return
      }
      form.reset(values)
      toast.success(
        mode === "create"
          ? t("mcpServer.form.createSuccess")
          : t("mcpServer.form.updateSuccess"),
      )
      navigate(listPath)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("MCP 服务保存失败", {
        mcp_server_id: mcpServerId,
        mode,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, [
              "name",
              "url",
              "serverType",
              "authorizationToken",
            ])
          : t("mcpServer.form.saveError"),
      )
    }
  }

  const title =
    mode === "create" ? t("mcpServer.form.createTitle") : t("mcpServer.form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        <ResourceContent
          loading={loading}
          error={Boolean(loadError)}
          errorMessage={t("mcpServer.form.loadError")}
          onRetry={() => void refresh()}
        >
          <form
            className="w-full max-w-2xl space-y-9"
            onSubmit={form.handleSubmit((values) => save(values))}
            noValidate
          >
            <FieldGroup>
              <FormInputField
                name="name"
                control={form.control}
                disabled={testing || form.formState.isSubmitting}
                label={t("mcpServer.form.name")}
                autoFocus={mode === "create"}
              />
              <FormInputField
                name="url"
                control={form.control}
                disabled={testing || form.formState.isSubmitting}
                label={t("mcpServer.form.url")}
                inputMode="url"
              />
              <Controller
                name="serverType"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="serverType" required>
                      {t("mcpServer.form.serverType")}
                    </FieldLabel>
                    <NativeSelect
                      {...field}
                      id="serverType"
                      disabled={testing || form.formState.isSubmitting}
                      required
                      aria-invalid={fieldState.invalid}
                    >
                      {[
                        MCPServerType.MCPServerTypeSSE,
                        MCPServerType.MCPServerTypeStreamableHTTP,
                      ].map((type) => (
                        <option key={type} value={type}>
                          {type}
                        </option>
                      ))}
                    </NativeSelect>
                  </Field>
                )}
              />
              <FormInputField
                name="authorizationToken"
                control={form.control}
                disabled={testing || form.formState.isSubmitting}
                label={t("mcpServer.form.authorizationToken")}
                required={false}
                autoComplete="new-password"
                passwordVisibilityLabels={{
                  show: t("mcpServer.form.showToken"),
                  hide: t("mcpServer.form.hideToken"),
                }}
              />
            </FieldGroup>
            <FormActions
              saving={form.formState.isSubmitting}
              disabled={testing}
              cancelTo={listPath}
              submit={mode === "create"}
            >
              <Button
                type="button"
                variant="outline"
                disabled={testing || form.formState.isSubmitting}
                onClick={() => void testConnection()}
              >
                {testing ? t("mcpServer.connection.testing") : t("mcpServer.connection.test")}
              </Button>
            </FormActions>
          </form>
        </ResourceContent>
      </PageContent>
    </div>
  )
}
