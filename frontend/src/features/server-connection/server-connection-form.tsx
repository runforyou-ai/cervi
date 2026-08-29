/** 企业服务器地址表单与已保存连接恢复。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon, RefreshCwIcon, SearchIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { connectServer, getServerURL, isApiError, probeServer } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { FieldGroup } from "@/components/ui/field"
import { useStartup } from "@/contexts/startup-context"
import type { ServerConnectionSource } from "@/features/server-connection/server-connection-page"
import {
  createServerConnectionSchema,
  type ServerConnectionFormValues,
} from "@/features/server-connection/server-connection-schema"
import { apiErrorMessage } from "@/lib/form-errors"

type DetectedServer = {
  serverUrl: string
  organizationName: string
}

type SavedServerState =
  | { status: "loading"; serverUrl: "" }
  | { status: "loaded"; serverUrl: string }
  | { status: "failed"; serverUrl: "" }

/** 检测企业服务器后确认连接，或恢复已保存的服务器连接。 */
export function ServerConnectionForm({
  source,
  onCancel,
}: {
  source: ServerConnectionSource | null
  onCancel?: () => void
}) {
  const { t } = useTranslation("connection")
  const navigate = useNavigate()
  const { completeStartup } = useStartup()
  const [savedServer, setSavedServer] = useState<SavedServerState>({
    status: "loading",
    serverUrl: "",
  })
  const [savedServerRevision, setSavedServerRevision] = useState(0)
  const [editing, setEditing] = useState(source !== null)
  const [detected, setDetected] = useState<DetectedServer | null>(null)
  const [detecting, setDetecting] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [recovering, setRecovering] = useState(false)
  const schema = useMemo(() => createServerConnectionSchema(t), [t])
  const form = useForm<ServerConnectionFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { serverUrl: "" },
  })
  const { getValues, reset, watch } = form
  const serverUrl = watch("serverUrl").trim()

  useEffect(() => {
    if (detected && detected.serverUrl !== serverUrl) {
      setDetected(null)
    }
  }, [detected, serverUrl])

  useEffect(() => {
    let stale = false
    setSavedServer({ status: "loading", serverUrl: "" })
    void getServerURL().then(
      (savedUrl) => {
        if (stale) return
        const normalizedURL = savedUrl.trim()
        setSavedServer({ status: "loaded", serverUrl: normalizedURL })
        if (getValues("serverUrl").trim() === "") {
          reset({ serverUrl: normalizedURL })
        }
      },
      (error: unknown) => {
        if (stale) return
        console.warn("读取企业服务器地址失败", error)
        setSavedServer({ status: "failed", serverUrl: "" })
      },
    )
    return () => {
      stale = true
    }
  }, [getValues, reset, savedServerRevision])

  /** 展示服务器检测或连接错误。 */
  function showConnectionError(error: unknown) {
    if (isApiError(error)) {
      toast.error(apiErrorMessage(error, ["serverUrl"]))
      return
    }
    toast.error(t("connectionError"))
  }

  /** 检测服务器状态并返回已初始化的企业信息。 */
  async function inspectServer(value: string): Promise<DetectedServer | null> {
    const status = await probeServer(value)
    const organizationName = status.organizationName.trim()
    if (!status.installed || organizationName === "") {
      toast.error(t("initializationRequired"))
      return null
    }
    return { serverUrl: value.trim(), organizationName }
  }

  /** 根据连接来源选择成功后的页面。 */
  function connectionDestination(changed: boolean) {
    if (source === "login") return "/login"
    if (source === "me") return changed ? "/login" : "/me"
    return "/inbox"
  }

  /** 完成启动状态并进入当前连接对应的页面。 */
  function finishConnection(
    organizationName: string,
    changed: boolean,
  ) {
    completeStartup(organizationName)
    navigate(connectionDestination(changed), { replace: true })
  }

  /** 检测企业服务器并展示企业名称。 */
  async function detectServer(values: ServerConnectionFormValues) {
    setDetecting(true)
    try {
      const result = await inspectServer(values.serverUrl)
      setDetected(result)
    } catch (error) {
      setDetected(null)
      showConnectionError(error)
    } finally {
      setDetecting(false)
    }
  }

  /** 保存已检测的企业服务器并进入身份检查。 */
  async function connectDetectedServer() {
    if (!detected) return
    setConnecting(true)
    try {
      const result = await connectServer(detected.serverUrl)
      finishConnection(detected.organizationName, result.changed)
    } catch (error) {
      showConnectionError(error)
    } finally {
      setConnecting(false)
    }
  }

  /** 重新检测并恢复已保存的企业服务器。 */
  async function recoverSavedServer() {
    if (savedServer.status !== "loaded" || savedServer.serverUrl === "") return
    setRecovering(true)
    try {
      const result = await inspectServer(savedServer.serverUrl)
      if (!result) return
      const connection = await connectServer(result.serverUrl)
      finishConnection(result.organizationName, connection.changed)
    } catch (error) {
      showConnectionError(error)
    } finally {
      setRecovering(false)
    }
  }

  /** 重新读取本机保存的企业服务器地址。 */
  function retrySavedServerLoad() {
    setSavedServerRevision((current) => current + 1)
  }

  /** 从地址编辑返回已保存服务器的恢复状态。 */
  function returnToRecovery() {
    setDetected(null)
    reset({ serverUrl: savedServer.serverUrl })
    setEditing(false)
  }

  if (savedServer.status === "loading") {
    return (
      <Card>
        <CardContent className="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
          <LoaderCircleIcon className="size-4 animate-spin" />
          {t("loading")}
        </CardContent>
      </Card>
    )
  }

  if (savedServer.status === "failed") {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("readErrorTitle")}</CardTitle>
          <CardDescription>{t("readErrorDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            className="w-full"
            variant="outline"
            onClick={retrySavedServerLoad}
          >
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </CardContent>
      </Card>
    )
  }

  const recoveryMode =
    source === null && !editing && savedServer.serverUrl !== ""
  if (recoveryMode) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("recoveryTitle")}</CardTitle>
          <CardDescription>{t("recoveryDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div>
            <p className="text-xs text-muted-foreground">
              {t("savedServerLabel")}
            </p>
            <p className="mt-1 break-all text-sm font-medium">
              {savedServer.serverUrl}
            </p>
          </div>
          <div className="grid gap-3">
            <Button disabled={recovering} onClick={recoverSavedServer}>
              {recovering ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : (
                <RefreshCwIcon />
              )}
              {recovering ? t("retrying") : t("retry")}
            </Button>
            <Button
              variant="outline"
              disabled={recovering}
              onClick={() => setEditing(true)}
            >
              {t("editServer")}
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  const busy = detecting || connecting
  const canReturnToRecovery =
    source === null && savedServer.serverUrl !== "" && editing
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {source === null ? t("title") : t("changeTitle")}
        </CardTitle>
        <CardDescription>
          {source === null ? t("description") : t("changeDescription")}
        </CardDescription>
        {onCancel || canReturnToRecovery ? (
          <CardAction>
            <Button
              className="h-11"
              variant="ghost"
              disabled={busy}
              onClick={onCancel ?? returnToRecovery}
            >
              {canReturnToRecovery ? t("back") : t("cancel")}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent>
        <form
          onSubmit={form.handleSubmit((values) => {
            if (detected) {
              void connectDetectedServer()
              return
            }
            void detectServer(values)
          })}
          noValidate
        >
          <FieldGroup>
            <div>
              <FormInputField
                name="serverUrl"
                control={form.control}
                label={t("serverUrlLabel")}
                type="url"
                autoCapitalize="none"
                autoCorrect="off"
                autoFocus
                endAction={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="absolute top-1/2 right-1 -translate-y-1/2 text-muted-foreground"
                    disabled={busy}
                    aria-label={detecting ? t("detecting") : t("detect")}
                    onClick={() => void form.handleSubmit(detectServer)()}
                  >
                    {detecting ? (
                      <LoaderCircleIcon className="animate-spin" />
                    ) : (
                      <SearchIcon />
                    )}
                  </Button>
                }
              />
              <div
                className={
                  detected
                    ? "grid grid-rows-[1fr] transition-[grid-template-rows] duration-300 ease-out"
                    : "grid grid-rows-[0fr] transition-[grid-template-rows] duration-200 ease-out"
                }
              >
                <div className="min-h-0 overflow-hidden">
                  {detected ? (
                    <div
                      className="mt-3 flex items-center justify-between gap-4 rounded-lg bg-muted/50 px-3.5 py-3 animate-in fade-in-0 slide-in-from-top-1 duration-300"
                      aria-live="polite"
                    >
                      <p
                        className="min-w-0 truncate text-[15px] leading-none font-medium tracking-[-0.02em]"
                        title={detected.organizationName}
                      >
                        {detected.organizationName}
                      </p>
                      <Button
                        type="button"
                        size="sm"
                        className="h-7 shrink-0 px-3"
                        disabled={busy}
                        onClick={connectDetectedServer}
                      >
                        {connecting ? (
                          <LoaderCircleIcon className="animate-spin" />
                        ) : null}
                        {connecting ? t("connecting") : t("connect")}
                      </Button>
                    </div>
                  ) : null}
                </div>
              </div>
            </div>
          </FieldGroup>
        </form>
      </CardContent>
    </Card>
  )
}
