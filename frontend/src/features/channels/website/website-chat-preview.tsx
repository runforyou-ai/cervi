import { useTranslation } from "react-i18next"

import type { WebsiteChannelChatInterfaceInput } from "@/api/channels"
import {
  defaultWebsiteChannelThemeColor,
  isWebsiteChannelThemeColor,
} from "@/features/channels/website/website-channel-chat-interface-schema"

function relativeLuminance(hexColor: string) {
  const channels = hexColor
    .slice(1)
    .match(/.{2}/g)!
    .map((channel) => Number.parseInt(channel, 16) / 255)
    .map((channel) =>
      channel <= 0.04045
        ? channel / 12.92
        : Math.pow((channel + 0.055) / 1.055, 2.4)
    )

  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722
}

function contrastingForeground(backgroundColor: string) {
  const luminance = relativeLuminance(backgroundColor)
  const whiteContrast = 1.05 / (luminance + 0.05)
  const blackContrast = (luminance + 0.05) / 0.05

  return whiteContrast >= blackContrast ? "#FFFFFF" : "#000000"
}

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
  const foregroundColor = contrastingForeground(themeColor)

  return (
    <aside className="xl:sticky xl:top-6 xl:self-start">
      <p className="mb-3 text-sm font-medium">
        {t("chatInterface.preview.title")}
      </p>
      <div className="flex min-h-[600px] items-end justify-center rounded-2xl border bg-muted/30 p-5 sm:p-8">
        <div className="flex h-[520px] w-full max-w-[360px] flex-col overflow-hidden rounded-2xl border bg-background shadow-lg">
          <header
            className="shrink-0 px-5 py-4"
            style={{
              backgroundColor: themeColor,
              color: foregroundColor,
            }}
          >
            <p className="truncate text-sm font-semibold">{title}</p>
            {subtitle ? (
              <p className="mt-1 truncate text-xs">{subtitle}</p>
            ) : null}
          </header>

          <div className="flex min-h-0 flex-1 flex-col justify-end gap-3 bg-muted/20 p-4">
            {greetingMessage ? (
              <div className="max-w-[82%] self-start rounded-2xl rounded-bl-md border bg-background px-3.5 py-2.5 text-sm leading-6 shadow-xs">
                {greetingMessage}
              </div>
            ) : null}
            <div
              className="max-w-[82%] self-end rounded-2xl rounded-br-md px-3.5 py-2.5 text-sm leading-6"
              style={{
                backgroundColor: themeColor,
                color: foregroundColor,
              }}
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
