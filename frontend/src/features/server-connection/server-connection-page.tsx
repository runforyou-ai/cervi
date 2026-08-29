/** 企业服务器连接页。 */
import { useLayoutEffect, useRef, useState, type ReactNode } from "react"
import { useLocation, useNavigate } from "react-router"

import { ServerConnectionForm } from "@/features/server-connection/server-connection-form"
import { resolveAppPlatform } from "@/platform/app-platform"

export type ServerConnectionSource = "login" | "me"

/** 按未展开高度垂直居中，内容增高时只向下延伸。 */
function AnchoredCenter({
  children,
  topAligned = false,
}: {
  children: ReactNode
  topAligned?: boolean
}) {
  const contentRef = useRef<HTMLDivElement>(null)
  const compactHeightRef = useRef<number | null>(null)
  const [offset, setOffset] = useState<number | null>(null)

  useLayoutEffect(() => {
    if (topAligned) return
    const node = contentRef.current
    if (!node) {
      return
    }

    const lockOffset = (height: number) => {
      compactHeightRef.current = height
      setOffset(Math.max(24, (window.innerHeight - height) / 2))
    }

    const sync = () => {
      const height = node.offsetHeight
      const compact = compactHeightRef.current
      if (compact == null || height <= compact + 1) {
        lockOffset(height)
      }
    }

    const onResize = () => {
      const compact = compactHeightRef.current
      if (compact == null) {
        sync()
        return
      }
      setOffset(Math.max(24, (window.innerHeight - compact) / 2))
    }

    sync()
    const observer = new ResizeObserver(sync)
    observer.observe(node)
    window.addEventListener("resize", onResize)
    return () => {
      observer.disconnect()
      window.removeEventListener("resize", onResize)
    }
  }, [topAligned])

  if (topAligned) {
    return (
      <main className="h-dvh w-full overflow-y-auto px-6">
        <div className="mx-auto w-full max-w-sm pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))]">
          {children}
        </div>
      </main>
    )
  }

  return (
    <main
      className={
        offset == null
          ? "flex h-dvh w-full items-center justify-center overflow-y-auto px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10"
          : "flex h-dvh w-full justify-center overflow-y-auto px-6 pb-[max(1.5rem,env(safe-area-inset-bottom))] md:px-10 md:pb-10"
      }
    >
      <div
        ref={contentRef}
        className="w-full max-w-sm"
        style={offset == null ? undefined : { marginTop: offset }}
      >
        {children}
      </div>
    </main>
  )
}

/** 从一次性路由状态中读取主动切换服务器的来源。 */
function connectionSource(value: unknown): ServerConnectionSource | null {
  if (typeof value !== "object" || value === null || !("from" in value)) {
    return null
  }
  const from = (value as { from?: unknown }).from
  return from === "login" || from === "me" ? from : null
}

/** 展示企业服务器地址表单。 */
export function ServerConnectionPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const source = connectionSource(location.state)
  const mobile = resolveAppPlatform() === "mobile"

  /** 取消主动切换并返回来源页。 */
  function cancelServerChange() {
    navigate(source === "me" ? "/me" : "/login", { replace: true })
  }

  return (
    <AnchoredCenter topAligned={mobile}>
      <div className="mb-6 text-center">
        <p className="text-lg font-semibold tracking-tight">Cervi</p>
      </div>
      <ServerConnectionForm
        source={source}
        onCancel={source ? cancelServerChange : undefined}
      />
    </AnchoredCenter>
  )
}
