/** 网站渠道挂件预览。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import type { WebsiteChannelChatInterfaceInput } from "@/api"
import { Button } from "@/components/ui/button"
import { resolveWebsiteChannelOrigin } from "@/features/channels/website/website-channel-access"

type PreviewStatus = "loading" | "ready" | "failed"

/** 按当前设置预览访客挂件与 Messenger。 */
export function WebsiteChatPreview({
  value,
}: {
  value: WebsiteChannelChatInterfaceInput
}) {
  const { t } = useTranslation("channels")
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const [previewOrigin, setPreviewOrigin] = useState("")
  const [status, setStatus] = useState<PreviewStatus>("loading")
  const [retryKey, setRetryKey] = useState(0)
  const previewURL = previewOrigin ? `${previewOrigin}/chat/preview` : ""

  useEffect(() => {
    let active = true
    setPreviewOrigin("")
    setStatus("loading")

    void resolveWebsiteChannelOrigin()
      .then((origin) => {
        if (!active) return
        const parsed = new URL(origin)
        setPreviewOrigin(parsed.origin)
      })
      .catch((error: unknown) => {
        if (!active) return
        console.warn("网站渠道 Messenger 预览地址解析失败", error)
        setStatus("failed")
      })

    return () => {
      active = false
    }
  }, [retryKey])

  /** 同步聊天界面设置到访客挂件预览。 */
  const syncPreview = useCallback(() => {
    if (!previewOrigin || !iframeRef.current?.contentWindow) return
    iframeRef.current.contentWindow.postMessage(
      { type: "cervi:preview-config", value },
      previewOrigin,
    )
  }, [previewOrigin, value])

  useEffect(() => {
    if (status === "ready") syncPreview()
  }, [status, syncPreview])

  useEffect(() => {
    if (!previewOrigin) return

    /** 只接受当前挂件预览宿主页发出的就绪消息。 */
    function handlePreviewMessage(event: MessageEvent) {
      if (
        event.origin !== previewOrigin ||
        event.source !== iframeRef.current?.contentWindow ||
        event.data?.type !== "cervi:preview-ready"
      ) {
        return
      }
      setStatus("ready")
    }

    window.addEventListener("message", handlePreviewMessage)
    return () => window.removeEventListener("message", handlePreviewMessage)
  }, [previewOrigin])

  useEffect(() => {
    if (!previewURL || status !== "loading") return
    const timeout = window.setTimeout(() => {
      console.warn("网站渠道 Messenger 预览加载超时", {
        preview_url: previewURL,
      })
      setStatus("failed")
    }, 8_000)
    return () => window.clearTimeout(timeout)
  }, [previewURL, status])

  /** 挂件预览页加载后同步当前设置。 */
  function handleLoad() {
    syncPreview()
  }

  /** 记录挂件预览页加载失败。 */
  function handleError() {
    console.warn("网站渠道 Messenger 预览页面加载失败", {
      preview_url: previewURL,
    })
    setStatus("failed")
  }

  /** 重新加载挂件预览。 */
  function retry() {
    setRetryKey((current) => current + 1)
  }

  return (
    <aside className="w-full max-w-[480px] xl:sticky xl:top-6 xl:self-start">
      <p className="mb-3 text-sm font-medium">
        {t("chatInterface.preview.title")}
      </p>

      <div className="relative h-[800px] overflow-hidden rounded-2xl border bg-muted/30 shadow-sm">
        {previewURL ? (
          <iframe
            ref={iframeRef}
            className="block size-full border-0 bg-background"
            src={previewURL}
            title={t("chatInterface.preview.frameTitle")}
            referrerPolicy="strict-origin-when-cross-origin"
            onLoad={handleLoad}
            onError={handleError}
          />
        ) : null}

        {status !== "ready" ? (
          <div className="absolute inset-0 grid place-items-center bg-background px-8 text-center">
            {status === "failed" ? (
              <div>
                <p className="text-sm text-muted-foreground">
                  {t("chatInterface.preview.loadFailed")}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="mt-4"
                  onClick={retry}
                >
                  {t("chatInterface.preview.retry")}
                </Button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("chatInterface.preview.loading")}
              </p>
            )}
          </div>
        ) : null}
      </div>
    </aside>
  )
}
