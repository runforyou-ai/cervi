/** 企业服务器地址表单。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon, SearchIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { connectServer, getServerURL, isApiError, probeServer } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { FieldGroup } from "@/components/ui/field"
import {
  createServerConnectionSchema,
  type ServerConnectionFormValues,
} from "@/features/server-connection/server-connection-schema"
import { useStartup } from "@/features/startup/startup-context"
import { apiErrorMessage } from "@/lib/form-errors"

type DetectedServer = {
  serverUrl: string
  organizationName: string
}

/** 检测企业服务器后确认连接。 */
export function ServerConnectionForm() {
  const { t } = useTranslation("connection")
  const navigate = useNavigate()
  const { completeStartup } = useStartup()
  const [detected, setDetected] = useState<DetectedServer | null>(null)
  const [detecting, setDetecting] = useState(false)
  const [connecting, setConnecting] = useState(false)
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
    void getServerURL()
      .then((savedUrl) => {
        if (
          stale ||
          savedUrl === "" ||
          getValues("serverUrl").trim() !== ""
        ) {
          return
        }
        reset({ serverUrl: savedUrl })
      })
      .catch((error: unknown) => {
        if (!stale) console.warn("读取企业服务器地址失败", error)
      })
    return () => {
      stale = true
    }
  }, [getValues, reset])

  /** 检测企业服务器并展示企业名称。 */
  async function detectServer(values: ServerConnectionFormValues) {
    setDetecting(true)
    try {
      const status = await probeServer(values.serverUrl)
      const organizationName = status.organizationName.trim()
      if (!status.installed || organizationName === "") {
        setDetected(null)
        toast.error(t("connectionError"))
        return
      }
      setDetected({
        serverUrl: values.serverUrl.trim(),
        organizationName,
      })
    } catch (error) {
      setDetected(null)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["serverUrl"]))
        return
      }
      toast.error(t("connectionError"))
    } finally {
      setDetecting(false)
    }
  }

  /** 保存已检测的企业服务器并进入身份检查。 */
  async function connectDetectedServer() {
    if (!detected) {
      return
    }
    setConnecting(true)
    try {
      await connectServer(detected.serverUrl)
      completeStartup(detected.organizationName)
      navigate("/inbox", { replace: true })
    } catch (error) {
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["serverUrl"]))
        return
      }
      toast.error(t("connectionError"))
    } finally {
      setConnecting(false)
    }
  }

  const busy = detecting || connecting

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("title")}</CardTitle>
        <CardDescription>{t("description")}</CardDescription>
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
