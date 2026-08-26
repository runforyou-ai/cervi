/** 管理工作台标签及长期挂载的页面实例。 */
import {
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type KeyboardEvent,
} from "react"
import { PinIcon, XIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  UNSAFE_NavigationContext,
  createPath,
  parsePath,
  useNavigate,
  useNavigationType,
  type Navigator,
  type To,
} from "react-router"

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
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { PortalContainerProvider } from "@/components/ui/portal-container"
import { WorkspaceTabLifecycleProvider } from "@/contexts/workspace-tab-lifecycle"
import type { WorkspaceOutletContext } from "@/features/workspace/workspace-context"
import { WorkspaceProvider } from "@/features/workspace/workspace-context"
import {
  WorkspacePageRoutes,
  resolveWorkspaceLocation,
  type ResolvedWorkspaceTab,
} from "@/features/workspace/workspace-page-routes"
import { cn } from "@/lib/utils"

type WorkspaceTabState = {
  tabs: ResolvedWorkspaceTab[]
  activeId: string
  pinnedIds: string[]
}

type WorkspaceTabAction =
  | {
      type: "sync"
      tab: ResolvedWorkspaceTab
      replaceCurrent: boolean
      activatingExisting: boolean
      protectActive: boolean
    }
  | { type: "activate"; id: string }
  | { type: "close"; id: string; nextActiveId: string }
  | { type: "keep"; ids: ReadonlySet<string>; activeId: string }
  | { type: "pin"; id: string; pinned: boolean }
  | {
      type: "backgroundNavigate"
      sourceId: string
      tab: ResolvedWorkspaceTab
      replaceCurrent: boolean
      protectSource: boolean
    }

/** 把固定标签按固定顺序排列在普通标签之前。 */
function orderWorkspaceTabs(
  tabs: ResolvedWorkspaceTab[],
  pinnedIds: string[],
) {
  const tabsById = new Map(tabs.map((tab) => [tab.id, tab]))
  const pinnedIdSet = new Set(pinnedIds)
  return {
    tabs: [
      ...pinnedIds.map((id) => tabsById.get(id)!),
      ...tabs.filter((tab) => !pinnedIdSet.has(tab.id)),
    ],
    pinnedIds,
  }
}

/** 页面替换时把原标签的固定位置转移给新标签。 */
function replacePinnedTabId(
  pinnedIds: string[],
  sourceId: string,
  targetId: string,
) {
  if (!pinnedIds.includes(sourceId)) {
    return pinnedIds
  }
  if (pinnedIds.includes(targetId)) {
    return pinnedIds.filter((id) => id !== sourceId)
  }
  return pinnedIds.map((id) => (id === sourceId ? targetId : id))
}

/** 更新标签页面、固定顺序和当前选中项。 */
function workspaceTabReducer(
  state: WorkspaceTabState,
  action: WorkspaceTabAction,
): WorkspaceTabState {
  if (action.type === "activate") {
    return { ...state, activeId: action.id }
  }
  if (action.type === "close") {
    return {
      tabs: state.tabs.filter((tab) => tab.id !== action.id),
      activeId: action.nextActiveId,
      pinnedIds: state.pinnedIds.filter((id) => id !== action.id),
    }
  }
  if (action.type === "keep") {
    return {
      tabs: state.tabs.filter((tab) => action.ids.has(tab.id)),
      activeId: action.activeId,
      pinnedIds: state.pinnedIds.filter((id) => action.ids.has(id)),
    }
  }
  if (action.type === "pin") {
    const pinnedIds = action.pinned
      ? [...state.pinnedIds, action.id]
      : state.pinnedIds.filter((id) => id !== action.id)
    return { ...state, ...orderWorkspaceTabs(state.tabs, pinnedIds) }
  }
  if (action.type === "backgroundNavigate") {
    const sourceTab = state.tabs.find((tab) => tab.id === action.sourceId)
    if (!sourceTab) {
      return state
    }
    if (sourceTab.id === action.tab.id) {
      return {
        ...state,
        tabs: state.tabs.map((tab) =>
          tab.id === sourceTab.id ? action.tab : tab,
        ),
      }
    }

    const replaceSource =
      !action.protectSource &&
      (action.replaceCurrent || sourceTab.transient)
    const targetExists = state.tabs.some((tab) => tab.id === action.tab.id)
    if (targetExists) {
      const pinnedIds = replaceSource
        ? replacePinnedTabId(
            state.pinnedIds,
            sourceTab.id,
            action.tab.id,
          )
        : state.pinnedIds
      const ordered = orderWorkspaceTabs(
        state.tabs
          .filter((tab) => !replaceSource || tab.id !== sourceTab.id)
          .map((tab) =>
            tab.id === action.tab.id && tab.id !== state.activeId
              ? action.tab
              : tab,
          ),
        pinnedIds,
      )
      return {
        ...state,
        ...ordered,
      }
    }
    if (replaceSource) {
      const ordered = orderWorkspaceTabs(
        state.tabs.map((tab) =>
          tab.id === sourceTab.id ? action.tab : tab,
        ),
        replacePinnedTabId(
          state.pinnedIds,
          sourceTab.id,
          action.tab.id,
        ),
      )
      return {
        ...state,
        ...ordered,
      }
    }
    return { ...state, tabs: [...state.tabs, action.tab] }
  }

  const activeTab = state.tabs.find((tab) => tab.id === state.activeId)!
  const replaceActive =
    !action.activatingExisting &&
    activeTab.id !== action.tab.id &&
    !action.protectActive &&
    (action.replaceCurrent || activeTab.transient)
  const existingIndex = state.tabs.findIndex((tab) => tab.id === action.tab.id)

  if (replaceActive) {
    if (existingIndex >= 0) {
      const ordered = orderWorkspaceTabs(
        state.tabs
          .filter((tab) => tab.id !== activeTab.id)
          .map((tab) => (tab.id === action.tab.id ? action.tab : tab)),
        replacePinnedTabId(
          state.pinnedIds,
          activeTab.id,
          action.tab.id,
        ),
      )
      return {
        ...ordered,
        activeId: action.tab.id,
      }
    }
    const ordered = orderWorkspaceTabs(
      state.tabs.map((tab) =>
        tab.id === activeTab.id ? action.tab : tab,
      ),
      replacePinnedTabId(state.pinnedIds, activeTab.id, action.tab.id),
    )
    return {
      ...ordered,
      activeId: action.tab.id,
    }
  }

  if (existingIndex >= 0) {
    return {
      tabs: state.tabs.map((tab) =>
        tab.id === action.tab.id ? action.tab : tab,
      ),
      activeId: action.tab.id,
      pinnedIds: state.pinnedIds,
    }
  }
  return {
    tabs: [...state.tabs, action.tab],
    activeId: action.tab.id,
    pinnedIds: state.pinnedIds,
  }
}

/** 在独立容器中保持一个标签页面及其浮层。 */
function WorkspaceTabPane({
  tab,
  active,
  context,
  onDirtyChange,
  onBackgroundNavigate,
}: {
  tab: ResolvedWorkspaceTab
  active: boolean
  context: WorkspaceOutletContext
  onDirtyChange: (tabId: string, source: symbol, dirty: boolean) => void
  onBackgroundNavigate: (sourceId: string, to: To, replace: boolean) => void
}) {
  const navigationContext = useContext(UNSAFE_NavigationContext)
  const activeRef = useRef(active)
  activeRef.current = active
  const [portalContainer, setPortalContainer] = useState<HTMLDivElement | null>(
    null,
  )
  const reportDirty = useCallback(
    (source: symbol, dirty: boolean) =>
      onDirtyChange(tab.id, source, dirty),
    [onDirtyChange, tab.id],
  )
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
          onBackgroundNavigate(tab.id, to, false)
        }
      },
      replace: (to, state, options) => {
        if (activeRef.current) {
          navigationContext.navigator.replace(to, state, options)
        } else {
          onBackgroundNavigate(tab.id, to, true)
        }
      },
    }),
    [navigationContext.navigator, onBackgroundNavigate, tab.id],
  )
  const scopedNavigationContext = useMemo(
    () => ({ ...navigationContext, navigator: scopedNavigator }),
    [navigationContext, scopedNavigator],
  )

  return (
    <div
      ref={setPortalContainer}
      id={`workspace-tab-panel-${encodeURIComponent(tab.id)}`}
      role="tabpanel"
      aria-labelledby={`workspace-tab-${encodeURIComponent(tab.id)}`}
      aria-hidden={!active}
      hidden={!active}
      inert={!active}
      className="flex h-full min-h-0 w-full flex-col overflow-hidden"
    >
      {portalContainer ? (
        <PortalContainerProvider container={portalContainer} active={active}>
          <WorkspaceTabLifecycleProvider
            reportDirty={reportDirty}
          >
            <WorkspaceProvider value={context}>
              <UNSAFE_NavigationContext.Provider
                value={scopedNavigationContext}
              >
                <WorkspacePageRoutes location={tab.href} />
              </UNSAFE_NavigationContext.Provider>
            </WorkspaceProvider>
          </WorkspaceTabLifecycleProvider>
        </PortalContainerProvider>
      ) : null}
    </div>
  )
}

/** 渲染一个标签并处理键盘导航。 */
function WorkspaceTabButton({
  tab,
  index,
  tabs,
  active,
  dirty,
  pinned,
  closable,
  onActivate,
  onClose,
}: {
  tab: ResolvedWorkspaceTab
  index: number
  tabs: ResolvedWorkspaceTab[]
  active: boolean
  dirty: boolean
  pinned: boolean
  closable: boolean
  onActivate: (id: string) => void
  onClose: (id: string) => void
}) {
  const { t } = useTranslation("workspace")
  const title = t(tab.titleKey)
  const statefulTitle = dirty
    ? t("tabs.unsavedTab", { title })
    : title
  const accessibleTitle = pinned
    ? t("tabs.pinnedTab", { title: statefulTitle })
    : statefulTitle

  /** 使用方向键切换标签，使用 Delete 关闭当前标签。 */
  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key === "Delete" && closable) {
      event.preventDefault()
      onClose(tab.id)
      return
    }
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") {
      return
    }
    event.preventDefault()
    const offset = event.key === "ArrowLeft" ? -1 : 1
    const targetIndex = (index + offset + tabs.length) % tabs.length
    onActivate(tabs[targetIndex].id)
  }

  return (
    <div
      data-active={active}
      className={cn(
        "cervi-workspace-tab group/tab relative isolate mt-1 flex h-10 w-40 shrink-0 items-center",
        active
          ? "z-10 rounded-t-[10px] bg-sidebar-accent text-sidebar-accent-foreground"
          : "text-foreground",
      )}
    >
      <button
        id={`workspace-tab-${encodeURIComponent(tab.id)}`}
        type="button"
        role="tab"
        aria-selected={active}
        aria-controls={`workspace-tab-panel-${encodeURIComponent(tab.id)}`}
        aria-label={accessibleTitle}
        tabIndex={active ? 0 : -1}
        title={title}
        className={cn(
          "flex h-full min-w-0 flex-1 items-center gap-2 rounded-t-[10px] px-3 text-left text-sm outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
          active && "font-medium",
        )}
        onClick={() => onActivate(tab.id)}
        onAuxClick={(event) => {
          if (event.button === 1 && closable) {
            event.preventDefault()
            onClose(tab.id)
          }
        }}
        onKeyDown={handleKeyDown}
      >
        {pinned ? (
          <PinIcon aria-hidden="true" className="size-3.5 shrink-0" />
        ) : null}
        {dirty ? (
          <span
            aria-hidden="true"
            className="size-1.5 shrink-0 rounded-full bg-primary"
          />
        ) : null}
        <span className="truncate">{title}</span>
      </button>
      {closable ? (
        <button
          type="button"
          className="mr-2 flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground opacity-70 outline-none transition-[background-color,color,opacity] hover:bg-background/70 hover:text-foreground hover:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={t("tabs.close", { title })}
          title={t("tabs.close", { title })}
          onClick={() => onClose(tab.id)}
        >
          <XIcon aria-hidden="true" className="size-3.5" />
        </button>
      ) : null}
    </div>
  )
}

/** 管理已打开标签的激活、关闭和永久挂载。 */
export function WorkspaceTabs({
  currentTab,
  context,
}: {
  currentTab: ResolvedWorkspaceTab
  context: WorkspaceOutletContext
}) {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const navigationType = useNavigationType()
  const pendingActivationRef = useRef<string | null>(null)
  const tabButtonRefs = useRef(new Map<string, HTMLButtonElement>())
  const dirtySourcesRef = useRef(new Map<string, Set<symbol>>())
  const dirtyTabIdsRef = useRef<ReadonlySet<string>>(new Set())
  const [dirtyTabIds, setDirtyTabIds] = useState<ReadonlySet<string>>(
    () => new Set(),
  )
  const [reloadRevisionById, setReloadRevisionById] = useState<
    Readonly<Record<string, number>>
  >({})
  const [pendingTabAction, setPendingTabAction] = useState<
    | { kind: "one"; id: string }
    | { kind: "others"; id: string }
    | { kind: "right"; id: string }
    | { kind: "reload"; id: string }
    | null
  >(null)
  const [state, dispatch] = useReducer(workspaceTabReducer, {
    tabs: [currentTab],
    activeId: currentTab.id,
    pinnedIds: [],
  })
  const activeTabIdRef = useRef(state.activeId)
  dirtyTabIdsRef.current = dirtyTabIds
  activeTabIdRef.current = state.activeId

  /** 仅在地址变化时同步标签，避免关闭操作被旧地址反向恢复。 */
  useLayoutEffect(() => {
    const activatingExisting = pendingActivationRef.current === currentTab.id
    if (activatingExisting) {
      pendingActivationRef.current = null
    }
    dispatch({
      type: "sync",
      tab: currentTab,
      replaceCurrent: navigationType === "REPLACE",
      activatingExisting,
      protectActive: dirtyTabIdsRef.current.has(activeTabIdRef.current),
    })
  }, [
    currentTab.href,
    currentTab.id,
    currentTab.titleKey,
    currentTab.transient,
    navigationType,
  ])

  useLayoutEffect(() => {
    tabButtonRefs.current
      .get(state.activeId)
      ?.scrollIntoView({ block: "nearest", inline: "nearest" })
  }, [state.activeId])

  useEffect(() => {
    if (dirtyTabIds.size === 0) {
      return
    }

    /** 刷新或关闭窗口前交给运行平台确认未保存内容。 */
    function preventUnsavedUnload(event: BeforeUnloadEvent) {
      event.preventDefault()
      event.returnValue = ""
    }

    window.addEventListener("beforeunload", preventUnsavedUnload)
    return () => window.removeEventListener("beforeunload", preventUnsavedUnload)
  }, [dirtyTabIds])

  /** 汇总标签内各表单的未保存状态。 */
  const reportTabDirty = useCallback(
    (tabId: string, source: symbol, dirty: boolean) => {
      const sources = dirtySourcesRef.current.get(tabId) ?? new Set<symbol>()
      if (dirty) {
        sources.add(source)
        dirtySourcesRef.current.set(tabId, sources)
      } else {
        sources.delete(source)
        if (sources.size === 0) {
          dirtySourcesRef.current.delete(tabId)
        }
      }
      setDirtyTabIds((current) => {
        const next = new Set(current)
        if (sources.size > 0) {
          next.add(tabId)
        } else {
          next.delete(tabId)
        }
        if (
          next.size === current.size &&
          [...next].every((id) => current.has(id))
        ) {
          return current
        }
        return next
      })
    },
    [],
  )

  /** 把隐藏页面的后续导航保留在原标签中。 */
  const navigateInBackgroundTab = useCallback(
    (sourceId: string, to: To, replaceCurrent: boolean) => {
      const href = typeof to === "string" ? to : createPath(to)
      const parsed = parsePath(href)
      if (
        parsed.pathname === "/login" ||
        parsed.pathname === "/connect" ||
        parsed.pathname === "/setup"
      ) {
        navigate(href, { replace: replaceCurrent })
        return
      }
      const resolved = resolveWorkspaceLocation({
        pathname: parsed.pathname ?? "/",
        search: parsed.search ?? "",
        hash: parsed.hash ?? "",
      })
      if (!resolved.tab) {
        navigate(resolved.canonicalHref, { replace: replaceCurrent })
        return
      }
      dispatch({
        type: "backgroundNavigate",
        sourceId,
        tab: resolved.tab,
        replaceCurrent,
        protectSource: dirtyTabIdsRef.current.has(sourceId),
      })
    },
    [navigate],
  )

  /** 清除指定标签登记的未保存状态。 */
  function discardDirtyState(ids: ReadonlySet<string>) {
    for (const id of ids) {
      dirtySourcesRef.current.delete(id)
    }
    setDirtyTabIds((current) => {
      const next = new Set(current)
      let changed = false
      for (const id of ids) {
        changed = next.delete(id) || changed
      }
      return changed ? next : current
    })
  }

  /** 激活现有标签并恢复其独立地址。 */
  function activateTab(id: string) {
    if (id === state.activeId) {
      return
    }
    const tab = state.tabs.find((candidate) => candidate.id === id)!
    pendingActivationRef.current = id
    dispatch({ type: "activate", id })
    navigate(tab.href)
  }

  /** 关闭一个标签，当前标签关闭后激活相邻页面。 */
  function closeTab(id: string) {
    const index = state.tabs.findIndex((tab) => tab.id === id)
    const nextTab =
      id === state.activeId
        ? (state.tabs[index + 1] ?? state.tabs[index - 1])!
        : state.tabs.find((tab) => tab.id === state.activeId)!
    discardDirtyState(new Set([id]))
    dispatch({ type: "close", id, nextActiveId: nextTab.id })
    if (id === state.activeId) {
      pendingActivationRef.current = nextTab.id
      navigate(nextTab.href)
    }
  }

  /** 关闭标签前确认是否放弃未保存内容。 */
  function requestCloseTab(id: string) {
    if (dirtyTabIds.has(id)) {
      setPendingTabAction({ kind: "one", id })
      return
    }
    closeTab(id)
  }

  /** 切换标签的固定状态并整理标签顺序。 */
  function setTabPinned(id: string, pinned: boolean) {
    dispatch({ type: "pin", id, pinned })
  }

  /** 重新挂载指定标签，保留其他标签的页面实例。 */
  function reloadTab(id: string) {
    discardDirtyState(new Set([id]))
    setReloadRevisionById((current) => ({
      ...current,
      [id]: (current[id] ?? 0) + 1,
    }))
    console.info("工作台标签已重新加载", { tab_id: id })
  }

  /** 重新加载前确认是否放弃未保存内容。 */
  function requestReloadTab(id: string) {
    if (dirtyTabIds.has(id)) {
      setPendingTabAction({ kind: "reload", id })
      return
    }
    reloadTab(id)
  }

  /** 只保留指定标签，并在需要时将它激活。 */
  function closeOtherTabs(id: string) {
    const targetTab = state.tabs.find((tab) => tab.id === id)!
    const removedIds = new Set(
      state.tabs.filter((tab) => tab.id !== id).map((tab) => tab.id),
    )
    discardDirtyState(removedIds)
    dispatch({
      type: "keep",
      ids: new Set([id]),
      activeId: id,
    })
    if (state.activeId !== id) {
      pendingActivationRef.current = id
      navigate(targetTab.href)
    }
  }

  /** 关闭指定标签右侧的所有标签。 */
  function closeTabsToRight(id: string) {
    const targetIndex = state.tabs.findIndex((tab) => tab.id === id)
    const targetTab = state.tabs[targetIndex]
    const removedIds = new Set(
      state.tabs.slice(targetIndex + 1).map((tab) => tab.id),
    )
    const activeId = removedIds.has(state.activeId) ? id : state.activeId
    discardDirtyState(removedIds)
    dispatch({
      type: "keep",
      ids: new Set(state.tabs.slice(0, targetIndex + 1).map((tab) => tab.id)),
      activeId,
    })
    if (activeId !== state.activeId) {
      pendingActivationRef.current = activeId
      navigate(targetTab.href)
    }
  }

  /** 需要时确认并关闭其他标签。 */
  function requestCloseOtherTabs(id: string) {
    const hasDirtyTab = state.tabs.some(
      (tab) => tab.id !== id && dirtyTabIds.has(tab.id),
    )
    if (hasDirtyTab) {
      setPendingTabAction({ kind: "others", id })
      return
    }
    closeOtherTabs(id)
  }

  /** 需要时确认并关闭右侧标签。 */
  function requestCloseTabsToRight(id: string) {
    const targetIndex = state.tabs.findIndex((tab) => tab.id === id)
    const hasDirtyTab = state.tabs
      .slice(targetIndex + 1)
      .some((tab) => dirtyTabIds.has(tab.id))
    if (hasDirtyTab) {
      setPendingTabAction({ kind: "right", id })
      return
    }
    closeTabsToRight(id)
  }

  /** 放弃未保存内容并执行等待中的操作。 */
  function confirmPendingTabAction() {
    const action = pendingTabAction
    setPendingTabAction(null)
    if (!action) {
      return
    }
    if (action.kind === "one") {
      closeTab(action.id)
    } else if (action.kind === "others") {
      closeOtherTabs(action.id)
    } else if (action.kind === "right") {
      closeTabsToRight(action.id)
    } else {
      reloadTab(action.id)
    }
  }

  const reloadingDirtyTab = pendingTabAction?.kind === "reload"

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
      <div className="cervi-workspace-tabs flex h-11 shrink-0 border-b bg-background px-2">
        <div
          role="tablist"
          aria-label={t("tabs.label")}
          className="flex min-w-0 flex-1 items-end overflow-x-auto px-2.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {state.tabs.map((tab, index) => (
            <ContextMenu key={tab.id}>
              <ContextMenuTrigger asChild>
                <div
                  className="h-full shrink-0"
                  ref={(element) => {
                    const button = element?.querySelector<HTMLButtonElement>(
                      '[role="tab"]',
                    )
                    if (button) {
                      tabButtonRefs.current.set(tab.id, button)
                    } else {
                      tabButtonRefs.current.delete(tab.id)
                    }
                  }}
                >
                  <WorkspaceTabButton
                    tab={tab}
                    index={index}
                    tabs={state.tabs}
                    active={tab.id === state.activeId}
                    dirty={dirtyTabIds.has(tab.id)}
                    pinned={state.pinnedIds.includes(tab.id)}
                    closable={state.tabs.length > 1}
                    onActivate={activateTab}
                    onClose={requestCloseTab}
                  />
                </div>
              </ContextMenuTrigger>
              <ContextMenuContent>
                <ContextMenuItem onSelect={() => requestReloadTab(tab.id)}>
                  {t("tabs.reload")}
                </ContextMenuItem>
                <ContextMenuItem
                  onSelect={() =>
                    setTabPinned(tab.id, !state.pinnedIds.includes(tab.id))
                  }
                >
                  {state.pinnedIds.includes(tab.id)
                    ? t("tabs.unpin")
                    : t("tabs.pin")}
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  disabled={state.tabs.length <= 1}
                  onSelect={() => requestCloseTab(tab.id)}
                >
                  {t("tabs.closeTab")}
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  disabled={state.tabs.length <= 1}
                  onSelect={() => requestCloseOtherTabs(tab.id)}
                >
                  {t("tabs.closeOthers")}
                </ContextMenuItem>
                <ContextMenuItem
                  disabled={index >= state.tabs.length - 1}
                  onSelect={() => requestCloseTabsToRight(tab.id)}
                >
                  {t("tabs.closeRight")}
                </ContextMenuItem>
              </ContextMenuContent>
            </ContextMenu>
          ))}
        </div>
      </div>
      <div className="relative flex min-h-0 min-w-0 flex-1 overflow-hidden">
        {state.tabs.map((tab) => (
          <WorkspaceTabPane
            key={`${tab.id}:${reloadRevisionById[tab.id] ?? 0}`}
            tab={tab}
            active={tab.id === state.activeId}
            context={context}
            onDirtyChange={reportTabDirty}
            onBackgroundNavigate={navigateInBackgroundTab}
          />
        ))}
      </div>
      <AlertDialog
        open={pendingTabAction !== null}
        onOpenChange={(open) => !open && setPendingTabAction(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t(
                reloadingDirtyTab
                  ? "tabs.reloadUnsavedTitle"
                  : "tabs.unsavedTitle",
              )}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                reloadingDirtyTab
                  ? "tabs.reloadUnsavedDescription"
                  : "tabs.unsavedDescription",
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("tabs.keepEditing")}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmPendingTabAction}>
              {t(
                reloadingDirtyTab
                  ? "tabs.discardAndReload"
                  : "tabs.discardAndClose",
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
