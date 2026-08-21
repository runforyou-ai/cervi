/** 网站渠道使用方式页签。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  resolveWebsiteChannelOrigin,
  websiteChannelChatURL,
  websiteChannelWidgetSnippet,
} from "@/features/channels/website/website-channel-access"
import { openExternalURL } from "@/platform/open-external-url"

export type WebsiteChannelAccess = "embed" | "link"

/** 展示网站渠道的嵌入代码和独立访问链接。 */
export function WebsiteChannelUsagePanel({
  channelId,
  access,
  onAccessChange,
}: {
  channelId: string
  access: WebsiteChannelAccess
  onAccessChange: (value: WebsiteChannelAccess) => void
}) {
  const { t } = useTranslation("channels")
  const [origin, setOrigin] = useState("")
  const [error, setError] = useState("")
  const [copied, setCopied] = useState<"snippet" | "link" | "">("")
  const [copyFailed, setCopyFailed] = useState(false)

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

  const snippet = origin ? websiteChannelWidgetSnippet(origin, channelId) : ""
  const chatUrl = origin ? websiteChannelChatURL(origin, channelId) : ""

  /** 复制渠道使用内容并反馈结果。 */
  async function copy(value: string, target: "snippet" | "link") {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(target)
      setCopyFailed(false)
      console.info("网站渠道使用内容已复制", { channel_id: channelId, target })
    } catch (error) {
      console.warn("复制网站渠道使用内容失败", error)
      setCopied("")
      setCopyFailed(true)
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

  return (
    <Tabs
      value={access}
      onValueChange={(value) => onAccessChange(value as WebsiteChannelAccess)}
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
        <FieldGroup className="mt-6 max-w-2xl">
          <Field>
            <FieldLabel>{t("usage.snippet")}</FieldLabel>
            <FieldDescription>{t("usage.snippetHelp")}</FieldDescription>
            <div className="flex items-start gap-2 rounded-md border bg-muted/30 px-3 py-2">
              <code className="min-w-0 flex-1 font-mono text-sm break-all whitespace-pre-wrap">
                {snippet}
              </code>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void copy(snippet, "snippet")}
              >
                {copied === "snippet" ? t("usage.copied") : t("usage.copy")}
              </Button>
            </div>
          </Field>
        </FieldGroup>
      </TabsContent>
      <TabsContent
        value="link"
        forceMount
        className="data-[state=inactive]:hidden"
      >
        <FieldGroup className="mt-6 max-w-2xl">
          <Field>
            <FieldLabel>{t("usage.chatUrl")}</FieldLabel>
            <FieldDescription>{t("usage.chatUrlHelp")}</FieldDescription>
            <div className="flex items-start gap-2 rounded-md border bg-muted/30 px-3 py-2">
              <button
                type="button"
                className="min-w-0 flex-1 text-left font-mono text-sm break-all hover:underline"
                onClick={() => void openExternalURL(chatUrl)}
              >
                {chatUrl}
              </button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void copy(chatUrl, "link")}
              >
                {copied === "link" ? t("usage.copied") : t("usage.copy")}
              </Button>
            </div>
          </Field>
        </FieldGroup>
      </TabsContent>
      {copyFailed ? (
        <p className="mt-3 text-sm text-destructive">{t("usage.copyFailed")}</p>
      ) : null}
    </Tabs>
  )
}
