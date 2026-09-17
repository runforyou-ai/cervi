/** 按运行平台加载 Web、桌面端或移动端应用。 */
import { lazy, Suspense, type CSSProperties } from "react"

import { LoadingIndicator } from "@/components/loading-indicator"
import { Toaster } from "@/components/ui/sonner"
import { StartupBootstrap } from "@/features/startup/startup-bootstrap"
import type { AppPlatform } from "@/platform/app-platform"

const WebApp = lazy(() => import("@/apps/web/web-app"))
const DesktopApp = lazy(() => import("@/apps/desktop/desktop-app"))
const MobileApp = lazy(() => import("@/apps/mobile/mobile-app"))

/** 根应用，按平台渲染对应入口。 */
function App({ platform }: { platform: AppPlatform }) {
  const mobile = platform === "mobile"
  const mobileToastOffset = mobile
    ? {
        top: "calc(env(safe-area-inset-top) + 1rem)",
        right: "1rem",
        left: "1rem",
      }
    : undefined

  return (
    <>
      <StartupBootstrap>
        <Suspense
          fallback={
            <main className="flex min-h-dvh items-center justify-center">
              {/* 平台应用加载中的占位。 */}
              <LoadingIndicator>
                <span className="sr-only">Loading</span>
              </LoadingIndicator>
            </main>
          }
        >
          {platform === "web" ? <WebApp /> : null}
          {platform === "desktop" ? <DesktopApp /> : null}
          {mobile ? <MobileApp /> : null}
        </Suspense>
      </StartupBootstrap>
      <Toaster
        position={mobile ? "top-center" : undefined}
        offset={mobileToastOffset}
        mobileOffset={mobileToastOffset}
        closeButton={!mobile}
        style={
          mobile
            ? ({
                "--width": "100%",
                "--border-radius": "calc(var(--radius) + 6px)",
              } as CSSProperties)
            : undefined
        }
        toastOptions={
          mobile
            ? {
                classNames: {
                  toast: "gap-3! px-4! py-3.5! shadow-lg",
                  title: "text-[15px]! leading-6!",
                  description: "text-sm! leading-5!",
                  icon: "[&>svg]:size-5!",
                },
              }
            : undefined
        }
      />
    </>
  )
}

export default App
