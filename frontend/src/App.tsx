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
  // 移动端轻提示容器横跨视口，胶囊在容器内水平居中。
  const mobileToastOffset = mobile
    ? {
        top: "calc(env(safe-area-inset-top) + 0.75rem)",
        right: "0px",
        left: "0px",
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
          mobile ? ({ "--width": "100vw" } as CSSProperties) : undefined
        }
        toastOptions={
          mobile
            ? {
                classNames: {
                  toast:
                    "inset-x-0! mx-auto! w-fit! max-w-[calc(100%-2rem)]! rounded-full! border-0! bg-neutral-900/85! px-4! py-2.5! text-white! shadow-[0_4px_24px_rgb(0_0_0/0.16)]! backdrop-blur-md! dark:bg-neutral-700/90!",
                  title: "text-sm! leading-5! font-medium!",
                  description: "text-[13px]! leading-5! text-white/70!",
                  icon: "hidden!",
                },
              }
            : undefined
        }
      />
    </>
  )
}

export default App
