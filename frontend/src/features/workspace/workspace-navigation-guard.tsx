/** 在页面离开、标签关闭和刷新前确认未保存内容。 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { useTranslation } from "react-i18next"
import { useBlocker } from "react-router"

import { SessionState, sessionPath } from "@/api"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  UnsavedChangesContext,
  type UnsavedForm,
} from "@/contexts/unsaved-changes-context"
import { resolveWorkspaceLocation } from "@/features/workspace/workspace-page-routes"

/** 判断导航是否卸载表单，并统一确认放弃修改。 */
export function WorkspaceNavigationGuard({
  tabsEnabled,
  children,
}: {
  tabsEnabled: boolean
  children: ReactNode
}) {
  const { t } = useTranslation("common")
  const forms = useRef(new Map<symbol, UnsavedForm>())
  const pendingRef = useRef<((confirmed: boolean) => void) | null>(null)
  const [pending, setPending] = useState(false)
  const blocker = useBlocker(
    ({ currentLocation, nextLocation, historyAction }) => {
      // 会话恢复必须到达入口，主动退出已在用户菜单中确认。
      if (
        [
          SessionState.SessionStateLogin,
          SessionState.SessionStateSetup,
          SessionState.SessionStateConnect,
        ].some((state) => sessionPath(state) === nextLocation.pathname)
      )
        return false
      const resolved = resolveWorkspaceLocation(nextLocation)
      const targetTab =
        resolved.tab ??
        resolveWorkspaceLocation({
          ...nextLocation,
          pathname: resolved.canonicalHref.split(/[?#]/)[0],
        }).tab
      return [...forms.current.values()].some((form) => {
        if (!form.dirty.current || form.pathname === nextLocation.pathname)
          return false
        if (!tabsEnabled) return true
        // 规范地址修正会替换当前标签，普通模块切换保留后台表单。
        const formTab = resolveWorkspaceLocation({
          pathname: form.pathname,
          search: "",
          hash: "",
        }).tab
        return (
          formTab?.id === targetTab?.id ||
          ((!resolved.tab || historyAction === "REPLACE") &&
            form.pathname === currentLocation.pathname)
        )
      })
    },
  )
  /** 登记表单，并在卸载时移除登记。 */
  const register = useCallback((id: symbol, form: UnsavedForm) => {
    forms.current.set(id, form)
    return () => {
      forms.current.delete(id)
    }
  }, [])
  /** 在关闭、重载标签或退出登录前确认未保存内容。 */
  const confirmTabs = useCallback(async (ids?: string[]) => {
    if (pendingRef.current) return false
    const dirty = [...forms.current.values()].some((form) => {
      const tab = resolveWorkspaceLocation({
        pathname: form.pathname,
        search: "",
        hash: "",
      }).tab
      return form.dirty.current && (!ids || (tab && ids.includes(tab.id)))
    })
    if (!dirty) return true
    return new Promise<boolean>((resolve) => {
      pendingRef.current = resolve
      setPending(true)
    })
  }, [])
  const context = useMemo(
    () => ({ register, confirmTabs }),
    [register, confirmTabs],
  )

  useEffect(() => {
    /** 浏览器刷新或关闭时使用原生离开提示。 */
    function beforeUnload(event: BeforeUnloadEvent) {
      if (![...forms.current.values()].some((form) => form.dirty.current))
        return
      event.preventDefault()
      event.returnValue = ""
    }
    window.addEventListener("beforeunload", beforeUnload)
    return () => window.removeEventListener("beforeunload", beforeUnload)
  }, [])

  /** 继续或取消会丢弃表单的操作。 */
  function finish(confirmed: boolean) {
    const resolve = pendingRef.current
    pendingRef.current = null
    setPending(false)
    // 历史导航发生时取消原操作，仅处理当前被拦截的导航。
    resolve?.(blocker.state === "blocked" ? false : confirmed)
    if (blocker.state === "blocked") {
      if (confirmed) blocker.proceed()
      else blocker.reset()
    }
  }

  return (
    <UnsavedChangesContext.Provider value={context}>
      {children}
      <AlertDialog
        open={pending || blocker.state === "blocked"}
        onOpenChange={(open) => {
          if (!open) finish(false)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("unsavedChanges.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("unsavedChanges.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("unsavedChanges.keepEditing")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(event) => {
                event.preventDefault()
                finish(true)
              }}
            >
              {t("unsavedChanges.discard")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </UnsavedChangesContext.Provider>
  )
}
