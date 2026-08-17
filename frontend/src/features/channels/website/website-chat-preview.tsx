import { useTranslation } from "react-i18next"

import type { WebsiteChannelChatInterfaceInput } from "@/api/channels"
import {
  defaultWebsiteChannelThemeColor,
  isWebsiteChannelThemeColor,
} from "@/features/channels/website/website-channel-chat-interface-schema"

export function WebsiteChatPreview({
  value,
}: {
  value: Partial<WebsiteChannelChatInterfaceInput>
}) {
  const { t } = useTranslation("channels")
  const title = value.title?.trim() || t("chatInterface.form.title")
  const subtitle = value.subtitle?.trim()
  const greetingMessage = value.greetingMessage?.trim()
  const themeColor = isWebsiteChannelThemeColor(value.themeColor)
    ? value.themeColor
    : defaultWebsiteChannelThemeColor

  return (
    <aside className="xl:sticky xl:top-6 xl:self-start">
      <p className="mb-3 text-sm font-medium">
        {t("chatInterface.preview.title")}
      </p>
      <div className="flex min-h-[600px] items-end justify-center rounded-2xl border bg-muted/30 p-5 sm:p-8">
        <div className="flex h-[520px] w-full max-w-[360px] flex-col overflow-hidden rounded-2xl border bg-background shadow-lg">
          <header
            className="shrink-0 px-5 py-4 text-white"
            style={{ backgroundColor: themeColor }}
          >
            <p className="truncate text-sm font-semibold">{title}</p>
            {subtitle ? (
              <p className="mt-1 truncate text-xs text-white/80">{subtitle}</p>
            ) : null}
          </header>

          <div className="flex min-h-0 flex-1 flex-col justify-end gap-3 bg-muted/20 p-4">
            {greetingMessage ? (
              <div className="max-w-[82%] self-start rounded-2xl rounded-bl-md border bg-background px-3.5 py-2.5 text-sm leading-6 shadow-xs">
                {greetingMessage}
              </div>
            ) : null}
            <div
              className="max-w-[82%] self-end rounded-2xl rounded-br-md px-3.5 py-2.5 text-sm leading-6 text-white"
              style={{ backgroundColor: themeColor }}
            >
              {t("chatInterface.preview.visitorMessage")}
            </div>
          </div>

          <div className="shrink-0 border-t bg-background p-3">
            <div className="flex h-10 items-center rounded-xl border px-3 text-sm text-muted-foreground">
              {t("chatInterface.preview.composerPlaceholder")}
            </div>
          </div>
        </div>
      </div>
    </aside>
  )
}
