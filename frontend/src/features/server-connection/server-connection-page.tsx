/** 企业服务器连接页。 */
import { useLayoutEffect, useRef, useState, type ReactNode } from "react"

import { ServerConnectionForm } from "@/features/server-connection/server-connection-form"

/** 按未展开高度垂直居中，内容增高时只向下延伸。 */
function AnchoredCenter({ children }: { children: ReactNode }) {
  const contentRef = useRef<HTMLDivElement>(null)
  const compactHeightRef = useRef<number | null>(null)
  const [offset, setOffset] = useState<number | null>(null)

  useLayoutEffect(() => {
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
  }, [])

  return (
    <main
      className={
        offset == null
          ? "flex min-h-dvh w-full items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10"
          : "flex min-h-dvh w-full justify-center px-6 pb-[max(1.5rem,env(safe-area-inset-bottom))] md:px-10 md:pb-10"
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

/** 展示企业服务器地址表单。 */
export function ServerConnectionPage() {
  return (
    <AnchoredCenter>
      <div className="mb-6 text-center">
        <p className="text-lg font-semibold tracking-tight">Cervi</p>
      </div>
      <ServerConnectionForm />
    </AnchoredCenter>
  )
}
