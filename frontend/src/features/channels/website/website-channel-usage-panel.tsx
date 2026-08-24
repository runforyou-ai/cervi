/** 网站渠道接入方式页签。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { LoaderCircleIcon } from "lucide-react"
import * as QRCode from "qrcode"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  updateWebsiteChannelAccess,
  type WebsiteChannelAccessData,
  type WebsiteChannelData,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import {
  resolveWebsiteChannelOrigin,
  websiteChannelChatURL,
  websiteChannelWidgetSnippet,
} from "@/features/channels/website/website-channel-access"
import {
  allowedHostLines,
  createWebsiteChannelAccessSchema,
  type WebsiteChannelAccessFormValues,
} from "@/features/channels/website/website-channel-access-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { openExternalURL } from "@/platform/open-external-url"

/** 网站渠道接入方式子页签。 */
export type WebsiteChannelAccessTab = "embed" | "link"

/** 展示安装代码或聊天链接的使用说明。 */
function WebsiteChannelUsageInstructions({
  kind,
  snippet,
  chatUrl,
  channelId,
  onOpenChange,
}: {
  kind: WebsiteChannelAccessTab | ""
  snippet: string
  chatUrl: string
  channelId: string
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("channels")
  const customButton = `<button type="button" data-cervi-open="${channelId}">${t("usage.instructions.contactButton")}</button>`

  return (
    <Dialog open={kind !== ""} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        {kind === "embed" ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("usage.instructions.embedTitle")}</DialogTitle>
              <DialogDescription>
                {t("usage.instructions.embedDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-5 text-sm">
              <section className="space-y-2">
                <p className="font-medium">
                  {t("usage.instructions.addCode")}
                </p>
                <p className="text-muted-foreground">
                  {t("usage.instructions.addCodeHelp")}
                </p>
                <pre className="rounded-md border bg-muted/30 p-3 break-all whitespace-pre-wrap">
                  {snippet}
                </pre>
              </section>
              <section className="space-y-2">
                <p className="font-medium">
                  {t("usage.instructions.customButton")}
                </p>
                <p className="text-muted-foreground">
                  {t("usage.instructions.customButtonHelp")}
                </p>
                <pre className="rounded-md border bg-muted/30 p-3 break-all whitespace-pre-wrap">
                  {customButton}
                </pre>
              </section>
              <section className="space-y-2">
                <p className="font-medium">
                  {t("usage.instructions.verifyEmbed")}
                </p>
                <p className="text-muted-foreground">
                  {t("usage.instructions.verifyEmbedHelp")}
                </p>
              </section>
            </div>
          </>
        ) : kind === "link" ? (
          <>
            <DialogHeader>
              <DialogTitle>{t("usage.instructions.linkTitle")}</DialogTitle>
              <DialogDescription>
                {t("usage.instructions.linkDescription")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-5 text-sm">
              <section className="space-y-2">
                <p className="font-medium">
                  {t("usage.instructions.shareLink")}
                </p>
                <p className="text-muted-foreground">
                  {t("usage.instructions.shareLinkHelp")}
                </p>
                <pre className="rounded-md border bg-muted/30 p-3 break-all whitespace-pre-wrap">
                  {chatUrl}
                </pre>
              </section>
              <section className="space-y-2">
                <p className="font-medium">
                  {t("usage.instructions.useQrCode")}
                </p>
                <p className="text-muted-foreground">
                  {t("usage.instructions.useQrCodeHelp")}
                </p>
              </section>
            </div>
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

/** 展示网站渠道的嵌入代码和独立访问链接。 */
export function WebsiteChannelUsagePanel({
  channel,
  access,
  onAccessChange,
  onUpdated,
}: {
  channel: WebsiteChannelData
  access: WebsiteChannelAccessTab
  onAccessChange: (value: WebsiteChannelAccessTab) => void
  onUpdated: (value: WebsiteChannelAccessData) => void
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const [origin, setOrigin] = useState("")
  const [error, setError] = useState("")
  const [copied, setCopied] = useState<"snippet" | "link" | "">("")
  const [copyFailed, setCopyFailed] = useState(false)
  const [instructions, setInstructions] = useState<WebsiteChannelAccessTab | "">("")
  const [qrCode, setQrCode] = useState("")
  const [qrCodeFailed, setQrCodeFailed] = useState(false)
  const [qrRetryKey, setQrRetryKey] = useState(0)
  const schema = useMemo(
    () =>
      createWebsiteChannelAccessSchema({
        tooMany: t("usage.validation.allowedHostsTooMany"),
        invalid: t("usage.validation.allowedHostInvalid"),
      }),
    [t],
  )
  const form = useForm<WebsiteChannelAccessFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      allowedHosts: channel.access.allowedHosts.join("\n"),
    },
  })

  useEffect(() => {
    let active = true
    setError("")
    void resolveWebsiteChannelOrigin()
      .then((value) => {
        if (!active) {
          return
        }
        if (value === "") {
          console.warn("网站渠道访问地址为空")
          setError(t("usage.originError"))
          return
        }
        setOrigin(value)
      })
      .catch((requestError: unknown) => {
        if (active) {
          console.warn("网站渠道访问地址解析失败", requestError)
          setError(t("usage.originError"))
        }
      })
    return () => {
      active = false
    }
  }, [t])

  useEffect(() => {
    if (copied === "") {
      return
    }
    const timeout = window.setTimeout(() => setCopied(""), 2000)
    return () => window.clearTimeout(timeout)
  }, [copied])

  const snippet = origin
    ? websiteChannelWidgetSnippet(origin, channel.id)
    : ""
  const chatUrl = origin ? websiteChannelChatURL(origin, channel.id) : ""

  useEffect(() => {
    if (!chatUrl) {
      return
    }
    let active = true
    setQrCodeFailed(false)
    void QRCode.toDataURL(chatUrl, {
      width: 160,
      margin: 1,
      errorCorrectionLevel: "M",
      color: { dark: "#111827", light: "#FFFFFF" },
    })
      .then((value) => {
        if (active) {
          setQrCode(value)
        }
      })
      .catch((requestError: unknown) => {
        if (active) {
          console.warn("聊天链接二维码生成失败", requestError)
          setQrCode("")
          setQrCodeFailed(true)
        }
      })
    return () => {
      active = false
    }
  }, [chatUrl, qrRetryKey])

  /** 复制渠道使用内容并反馈结果。 */
  async function copy(value: string, target: "snippet" | "link") {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(target)
      setCopyFailed(false)
      console.info("网站渠道使用内容已复制", {
        channel_id: channel.id,
        target,
      })
    } catch (copyError) {
      console.warn("复制网站渠道使用内容失败", copyError)
      setCopied("")
      setCopyFailed(true)
    }
  }

  /** 保存允许使用的网站。 */
  async function submit(values: WebsiteChannelAccessFormValues) {
    try {
      const updated = await updateWebsiteChannelAccess(channel.id, {
        allowedHosts: allowedHostLines(values.allowedHosts),
      })
      form.reset({ allowedHosts: updated.allowedHosts.join("\n") })
      onUpdated(updated)
      console.info("网站渠道允许使用的网站已保存", {
        channel_id: channel.id,
      })
      toast.success(t("usage.saved"))
    } catch (submitError) {
      if (recoverSession(submitError, navigate)) {
        return
      }
      if (isNotFoundApiError(submitError)) {
        console.warn("网站渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
        return
      }
      if (isApiError(submitError)) {
        console.warn("保存网站渠道允许使用的网站失败", submitError)
        toast.error(apiErrorMessage(submitError, ["allowedHosts"]))
        return
      }
      console.warn("保存网站渠道允许使用的网站失败", submitError)
      toast.error(t("form.networkError"))
    }
  }

  if (!origin && !error) {
    return (
      <div className="flex items-center gap-2 py-6 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </div>
    )
  }

  if (error) {
    return <p className="py-6 text-sm text-muted-foreground">{error}</p>
  }

  const { isSubmitting } = form.formState

  return (
    <>
      <Tabs
        value={access}
        onValueChange={(value) =>
          onAccessChange(value as WebsiteChannelAccessTab)
        }
      >
        <TabsList>
          <TabsTrigger value="embed">{t("usage.embed")}</TabsTrigger>
          <TabsTrigger value="link">{t("usage.link")}</TabsTrigger>
        </TabsList>
        <TabsContent
          value="embed"
          forceMount
          className="data-[state=inactive]:hidden"
        >
          <FieldGroup className="mt-6 max-w-2xl gap-8">
            <Field>
              <div className="flex items-center gap-2">
                <FieldLabel>{t("usage.snippet")}</FieldLabel>
                <Button
                  type="button"
                  variant="link"
                  size="xs"
                  className="h-auto px-0 py-0 text-xs font-normal text-muted-foreground"
                  onClick={() => setInstructions("embed")}
                >
                  {t("usage.instructions.open")}
                </Button>
              </div>
              <FieldDescription>{t("usage.snippetHelp")}</FieldDescription>
              <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
                <code className="flex min-h-8 min-w-0 flex-1 items-center font-mono text-sm break-all whitespace-pre-wrap">
                  {snippet}
                </code>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0"
                  onClick={() => void copy(snippet, "snippet")}
                >
                  {copied === "snippet" ? t("usage.copied") : t("usage.copy")}
                </Button>
              </div>
            </Field>

            <form onSubmit={form.handleSubmit(submit)} noValidate>
              <FieldGroup>
                <Controller
                  name="allowedHosts"
                  control={form.control}
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor={field.name}>
                        {t("usage.allowedHosts")}
                      </FieldLabel>
                      <FieldDescription>
                        {t("usage.allowedHostsHelp")}
                      </FieldDescription>
                      <Textarea
                        {...field}
                        id={field.name}
                        rows={4}
                        disabled={isSubmitting}
                        aria-invalid={fieldState.invalid}
                      />
                    </Field>
                  )}
                />
                <div>
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? t("form.saving") : t("form.save")}
                  </Button>
                </div>
              </FieldGroup>
            </form>
          </FieldGroup>
        </TabsContent>
        <TabsContent
          value="link"
          forceMount
          className="data-[state=inactive]:hidden"
        >
          <FieldGroup className="mt-6 max-w-2xl gap-8">
            <Field>
              <div className="flex items-center gap-2">
                <FieldLabel>{t("usage.chatUrl")}</FieldLabel>
                <Button
                  type="button"
                  variant="link"
                  size="xs"
                  className="h-auto px-0 py-0 text-xs font-normal text-muted-foreground"
                  onClick={() => setInstructions("link")}
                >
                  {t("usage.instructions.open")}
                </Button>
              </div>
              <FieldDescription>{t("usage.chatUrlHelp")}</FieldDescription>
              <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
                <button
                  type="button"
                  className="flex min-h-8 min-w-0 flex-1 items-center text-left font-mono text-sm break-all hover:underline"
                  onClick={() => void openExternalURL(chatUrl)}
                >
                  {chatUrl}
                </button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0"
                  onClick={() => void copy(chatUrl, "link")}
                >
                  {copied === "link" ? t("usage.copied") : t("usage.copy")}
                </Button>
              </div>
            </Field>

            <Field>
              <FieldLabel>{t("usage.qrCode")}</FieldLabel>
              <FieldDescription>{t("usage.qrCodeHelp")}</FieldDescription>
              <div>
                <div className="flex aspect-square w-40 items-center justify-center rounded-md border bg-white p-2">
                  {qrCode ? (
                    <img
                      src={qrCode}
                      alt={t("usage.qrCodeAlt")}
                      className="aspect-square w-full object-contain"
                    />
                  ) : (
                    <div className="space-y-2 px-3 text-center text-xs text-muted-foreground">
                      <p>
                        {qrCodeFailed
                          ? t("usage.qrCodeFailed")
                          : t("usage.qrCodeLoading")}
                      </p>
                      {qrCodeFailed ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => setQrRetryKey((value) => value + 1)}
                        >
                          {t("retry")}
                        </Button>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
            </Field>
          </FieldGroup>
        </TabsContent>
        {copyFailed ? (
          <p className="mt-3 text-sm text-destructive">
            {t("usage.copyFailed")}
          </p>
        ) : null}
      </Tabs>

      <WebsiteChannelUsageInstructions
        kind={instructions}
        snippet={snippet}
        chatUrl={chatUrl}
        channelId={channel.id}
        onOpenChange={(open) => {
          if (!open) {
            setInstructions("")
          }
        }}
      />
    </>
  )
}
