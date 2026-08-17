import { resolveAppPlatform } from "@/platform/app-platform"

export async function openExternalURL(url: string) {
  if (resolveAppPlatform() === "web") {
    window.open(url, "_blank", "noopener,noreferrer")
    return
  }

  const { Browser } = await import("@wailsio/runtime")
  await Browser.OpenURL(url)
}
