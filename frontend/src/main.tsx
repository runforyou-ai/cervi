/** 初始化国际化并挂载前端应用。 */
import React from "react"
import ReactDOM from "react-dom/client"
import { QueryClientProvider } from "@tanstack/react-query"
import { ThemeProvider } from "next-themes"
import { HashRouter } from "react-router"

import App from "@/App"
import { TooltipProvider } from "@/components/ui/tooltip"
import { initializeI18n } from "@/i18n"
import "@/index.css"
import { resourceClient } from "@/lib/resource-client"
import { resolveAppPlatform, type AppPlatform } from "@/platform/app-platform"

/** Web 和桌面端禁用原生右键菜单。 */
function applyPlatformContextMenu(platform: AppPlatform) {
  if (platform === "mobile") {
    return
  }

  document.addEventListener("contextmenu", (event) => {
    event.preventDefault()
  })
}

/** 启动前端应用。 */
async function bootstrap() {
  await initializeI18n()
  const platform = resolveAppPlatform()
  applyPlatformContextMenu(platform)

  ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    <React.StrictMode>
      <QueryClientProvider client={resourceClient}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <HashRouter>
            <TooltipProvider>
              <App platform={platform} />
            </TooltipProvider>
          </HashRouter>
        </ThemeProvider>
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

void bootstrap().catch((error: unknown) => {
  console.error("应用初始化失败", error)
})
