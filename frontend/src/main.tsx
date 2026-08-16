import React from "react"
import ReactDOM from "react-dom/client"
import { ThemeProvider } from "next-themes"
import { HashRouter } from "react-router"

import App from "@/App"
import { TooltipProvider } from "@/components/ui/tooltip"
import { initializeI18n } from "@/i18n"
import "@/index.css"
import { resolveAppPlatform } from "@/platform/app-platform"

async function bootstrap() {
  await initializeI18n()

  ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    <React.StrictMode>
      <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
        <HashRouter>
          <TooltipProvider>
            <App platform={resolveAppPlatform()} />
          </TooltipProvider>
        </HashRouter>
      </ThemeProvider>
    </React.StrictMode>,
  )
}

void bootstrap().catch((error: unknown) => {
  console.error("应用初始化失败", error)
})
