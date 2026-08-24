/** 按运行平台加载 Web、桌面端或移动端应用。 */
import { lazy, Suspense } from "react"
import { LoaderCircleIcon } from "lucide-react"

import { Toaster } from "@/components/ui/sonner"
import { StartupBootstrap } from "@/features/startup/startup-bootstrap"
import type { AppPlatform } from "@/platform/app-platform"

const WebApp = lazy(() => import("@/apps/web/web-app"))
const DesktopApp = lazy(() => import("@/apps/desktop/desktop-app"))
const MobileApp = lazy(() => import("@/apps/mobile/mobile-app"))

/** 平台应用加载中的占位。 */
function AppLoading() {
  return (
    <main className="flex min-h-dvh items-center justify-center">
      <LoaderCircleIcon
        aria-label="Loading"
        className="size-5 animate-spin text-muted-foreground"
      />
    </main>
  )
}

/** 根应用，按平台渲染对应入口。 */
function App({ platform }: { platform: AppPlatform }) {
  return (
    <>
      <StartupBootstrap>
        <Suspense fallback={<AppLoading />}>
          {platform === "web" ? <WebApp /> : null}
          {platform === "desktop" ? <DesktopApp /> : null}
          {platform === "mobile" ? <MobileApp /> : null}
        </Suspense>
      </StartupBootstrap>
      <Toaster />
    </>
  )
}

export default App
