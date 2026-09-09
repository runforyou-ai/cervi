/** 共享表单未保存状态登记入口。 */
import { createContext, useContext, type RefObject } from "react"

export type UnsavedForm = {
  pathname: string
  dirty: RefObject<boolean>
}

export const UnsavedChangesContext = createContext<{
  register: (id: symbol, form: UnsavedForm) => () => void
  confirmTabs: (ids?: string[]) => Promise<boolean>
} | null>(null)

/** 读取当前工作台的未保存内容管理入口。 */
export function useUnsavedChangesContext() {
  return useContext(UnsavedChangesContext)
}
