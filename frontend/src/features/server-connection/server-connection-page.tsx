/** 企业服务器连接页。 */
import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { useLocation, useNavigate } from "react-router"

import { ServerConnectionForm } from "@/features/server-connection/server-connection-form"

export type ServerConnectionSource = "login"

/** 居中基础表单，并让检测结果保持顶部锚点向下展开。 */
function CenteredContent({
  anchorReady,
  children,
}: {
  anchorReady: boolean
  children: ReactNode
}) {
  const contentRef = useRef<HTMLDivElement>(null)
  const [anchorHeight, setAnchorHeight] = useState<number | null>(null)

  useLayoutEffect(() => {
    if (!anchorReady) {
      setAnchorHeight(null)
      return
    }
    if (!contentRef.current) return
    setAnchorHeight(contentRef.current.offsetHeight)
  }, [anchorReady])

  return (
    <main className="flex h-dvh w-full overflow-y-auto px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))] md:p-10">
      <div
        ref={contentRef}
        className="mx-auto my-auto w-full max-w-sm shrink-0"
        style={anchorHeight === null ? undefined : { height: anchorHeight }}
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
  return from === "login" ? from : null
}

/** 展示企业服务器地址表单。 */
export function ServerConnectionPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const source = connectionSource(location.state)
  const [anchorReady, setAnchorReady] = useState(false)
  const updateAnchorReady = useCallback(
    (ready: boolean) => setAnchorReady(ready),
    [],
  )

  /** 取消主动切换并返回登录页。 */
  function cancelServerChange() {
    navigate("/login", { replace: true })
  }

  return (
    <CenteredContent anchorReady={anchorReady}>
      <div className="mb-6 text-center">
        <p className="text-lg font-semibold tracking-tight">Cervi</p>
      </div>
      <ServerConnectionForm
        source={source}
        onCancel={source ? cancelServerChange : undefined}
        onEditableLayoutChange={updateAnchorReady}
      />
    </CenteredContent>
  )
}
