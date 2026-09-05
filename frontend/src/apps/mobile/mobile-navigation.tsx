/** 保存当前登录会话的移动端导航、列表加载进度和滚动位置。 */
import {
  createContext,
  useContext,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { useLocation, useNavigate } from "react-router"

type MobileNavigationState = {
  inboxURL: string
  scrollPositions: Map<string, number>
  listPageCounts: Map<string, number>
}

const MobileNavigationContext = createContext<MobileNavigationState | null>(
  null,
)

/** 在登录工作区内保存导航状态，退出时一起释放。 */
export function MobileNavigationProvider({
  children,
}: {
  children: ReactNode
}) {
  const [inboxURL, setInboxURL] = useState("/inbox")
  const scrollPositions = useRef(new Map<string, number>())
  const listPageCounts = useRef(new Map<string, number>())
  const location = useLocation()
  useLayoutEffect(() => {
    if (location.pathname === "/inbox") {
      setInboxURL(location.pathname + location.search)
    }
  }, [location.pathname, location.search])
  return (
    <MobileNavigationContext
      value={{
        inboxURL,
        scrollPositions: scrollPositions.current,
        listPageCounts: listPageCounts.current,
      }}
    >
      {children}
    </MobileNavigationContext>
  )
}

/** 返回当前登录工作区的导航记录。 */
export function useMobileNavigation() {
  const state = useContext(MobileNavigationContext)
  if (!state) throw new Error("移动端导航必须位于登录工作区内")
  return state
}

/** 沿站内来源返回，直接打开子页时回到所属模块。 */
export function useMobileBack(fallback: string) {
  const navigate = useNavigate()
  const location = useLocation()
  return () => {
    if ((location.state as { mobileBack?: boolean } | null)?.mobileBack) {
      void navigate(-1)
    } else {
      void navigate(fallback, { replace: true })
    }
  }
}
