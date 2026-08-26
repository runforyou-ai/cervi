/** 提供工作台标签的未保存状态登记。 */
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react"

type WorkspaceTabLifecycleContextValue = {
  reportDirty: (source: symbol, dirty: boolean) => void
}

const WorkspaceTabLifecycleContext =
  createContext<WorkspaceTabLifecycleContextValue | null>(null)

/** 返回当前标签的生命周期状态。 */
function useWorkspaceTabLifecycle() {
  const context = useContext(WorkspaceTabLifecycleContext)
  if (!context) {
    throw new Error("工作台标签生命周期不可用")
  }
  return context
}

/** 向标签页面提供未保存状态登记入口。 */
export function WorkspaceTabLifecycleProvider({
  reportDirty,
  children,
}: WorkspaceTabLifecycleContextValue & { children: ReactNode }) {
  const value = useMemo(() => ({ reportDirty }), [reportDirty])
  return (
    <WorkspaceTabLifecycleContext.Provider value={value}>
      {children}
    </WorkspaceTabLifecycleContext.Provider>
  )
}

/** 登记表单是否存在未保存修改。 */
export function useWorkspaceTabDirty(dirty: boolean) {
  const { reportDirty } = useWorkspaceTabLifecycle()
  const sourceRef = useRef(Symbol("workspace-tab-dirty-source"))

  useEffect(() => {
    reportDirty(sourceRef.current, dirty)
  }, [dirty, reportDirty])

  useEffect(
    () => () => reportDirty(sourceRef.current, false),
    [reportDirty],
  )
}
