/**
 * 为单个工作台标签构造独立的路由上下文。
 *
 * 本模块是全前端唯一允许使用 react-router UNSAFE_ 内部 API 的位置：
 * 多标签需要在一个 Router 实例下让每个标签保持各自的地址和导航行为，
 * react-router 没有公开等价能力。package.json 已将 react-router 锁定为
 * 精确版本；升级 react-router 前必须先在本模块验证这两个内部上下文
 * 的行为未变化。
 */
import { useContext, useMemo, useRef, type ReactNode } from "react"
import {
  NavigationType,
  UNSAFE_LocationContext,
  UNSAFE_NavigationContext,
  parsePath,
  type Navigator,
  type To,
} from "react-router"

/** 把隐藏标签的导航转交给宿主处理的回调。 */
export type TabBackgroundNavigate = (
  sourceId: string,
  to: To,
  replace: boolean,
) => void

/** 让子树只感知本标签的地址，并把后台标签的导航交回宿主。 */
export function TabScopedRouter({
  tabId,
  href,
  active,
  onBackgroundNavigate,
  children,
}: {
  tabId: string
  href: string
  active: boolean
  onBackgroundNavigate: TabBackgroundNavigate
  children: ReactNode
}) {
  const navigationContext = useContext(UNSAFE_NavigationContext)
  const activeRef = useRef(active)
  activeRef.current = active

  const scopedNavigator = useMemo<Navigator>(
    () => ({
      ...navigationContext.navigator,
      go: (delta) => {
        if (activeRef.current) {
          navigationContext.navigator.go(delta)
        }
      },
      push: (to, state, options) => {
        if (activeRef.current) {
          navigationContext.navigator.push(to, state, options)
        } else {
          onBackgroundNavigate(tabId, to, false)
        }
      },
      replace: (to, state, options) => {
        if (activeRef.current) {
          navigationContext.navigator.replace(to, state, options)
        } else {
          onBackgroundNavigate(tabId, to, true)
        }
      },
    }),
    [navigationContext.navigator, onBackgroundNavigate, tabId],
  )
  const scopedNavigationContext = useMemo(
    () => ({ ...navigationContext, navigator: scopedNavigator }),
    [navigationContext, scopedNavigator],
  )
  /** 面板内页面只感知本标签的地址，避免切换标签时其他页面重新请求数据。 */
  const scopedLocationContext = useMemo(() => {
    const parsed = parsePath(href)
    return {
      location: {
        pathname: parsed.pathname ?? "/",
        search: parsed.search ?? "",
        hash: parsed.hash ?? "",
        state: null,
        key: href,
      },
      navigationType: NavigationType.Pop,
    }
  }, [href])

  return (
    <UNSAFE_NavigationContext.Provider value={scopedNavigationContext}>
      <UNSAFE_LocationContext.Provider value={scopedLocationContext}>
        {children}
      </UNSAFE_LocationContext.Provider>
    </UNSAFE_NavigationContext.Provider>
  )
}
