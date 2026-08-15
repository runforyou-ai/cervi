import { lazy, Suspense } from "react"
import { LoaderCircleIcon } from "lucide-react"

import { Toaster } from "@/components/ui/sonner"
import type { AppPlatform } from "@/platform/app-platform"

const WebApp = lazy(() => import("@/apps/web/web-app"))
const DesktopApp = lazy(() => import("@/apps/desktop/desktop-app"))
const MobileApp = lazy(() => import("@/apps/mobile/mobile-app"))

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

function App({ platform }: { platform: AppPlatform }) {
  return (
    <>
      <Suspense fallback={<AppLoading />}>
        {platform === "web" ? <WebApp /> : null}
        {platform === "desktop" ? <DesktopApp /> : null}
        {platform === "mobile" ? <MobileApp /> : null}
      </Suspense>
      <Toaster />
    </>
  )
}

export default App
