/** 登记表单未保存状态并标识组件生命周期。 */
import { useEffect, useRef } from "react"
import { useLocation } from "react-router"

import { useUnsavedChangesContext } from "@/contexts/unsaved-changes-context"

/** 保留未保存提示，并让提交操作忽略卸载后的界面更新。 */
export function useFormLifetime(isDirty: boolean) {
  const { pathname } = useLocation()
  const context = useUnsavedChangesContext()
  const mounted = useRef(false)
  const dirty = useRef(isDirty)
  dirty.current = isDirty
  const register = context?.register

  useEffect(() => {
    mounted.current = true
    const unregister = register?.(Symbol(), { pathname, dirty })
    return () => {
      mounted.current = false
      unregister?.()
    }
  }, [pathname, register])

  return { mounted, dirty }
}
