/** 按运行平台加载 Web、桌面端或移动端应用。 */
import { lazy, Suspense } from "react"

import { Toaster } from "@/components/ui/sonner"
import {
  SessionGate,
  SessionLoadingState,
} from "@/features/session/session-gate"
import type { AppPlatform } from "@/platform/app-platform"

const WebApp = lazy(() => import("@/apps/web/web-app"))
const DesktopApp = lazy(() => import("@/apps/desktop/desktop-app"))
const MobileApp = lazy(() => import("@/apps/mobile/mobile-app"))

/** 根应用，按平台渲染对应入口。 */
function App({ platform }: { platform: AppPlatform }) {
  return (
    <>
      <SessionGate platform={platform}>
        <Suspense fallback={<SessionLoadingState />}>
          {platform === "web" ? <WebApp /> : null}
          {platform === "desktop" ? <DesktopApp /> : null}
          {platform === "mobile" ? <MobileApp /> : null}
        </Suspense>
      </SessionGate>
      <Toaster />
    </>
  )
}

export default App
