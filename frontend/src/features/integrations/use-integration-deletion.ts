/** 管理集成配置的删除确认、请求状态和缓存失效。 */
import { useState } from "react"
import type { QueryKey } from "@tanstack/react-query"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError } from "@/api"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 串行删除选中的配置，并保留失败时的确认上下文。 */
export function useIntegrationDeletion<T extends { id: string }>({
  deleteItem,
  listKey,
  detailKey,
  entityName,
  successMessage,
  errorMessage,
  relatedKeys = [],
}: {
  deleteItem: (id: string) => Promise<unknown>
  listKey: QueryKey
  detailKey: (id: string) => QueryKey
  entityName: string
  successMessage: string
  errorMessage: string
  relatedKeys?: QueryKey[]
}) {
  const [item, setItem] = useState<T | null>(null)
  const save = useImmediateSave()
  const invalidate = useResourceInvalidator()
  const navigate = useNavigate()

  /** 删除当前配置，离开页面后仅更新共享缓存。 */
  async function confirm() {
    if (!item) return
    const request = save.begin()
    if (request === null) return
    try {
      await deleteItem(item.id)
      void invalidate(listKey)
      void invalidate(detailKey(item.id))
      for (const key of relatedKeys) void invalidate(key)
      console.info(`${entityName}已删除`, { id: item.id })
      if (!save.isCurrent(request)) return
      setItem(null)
      toast.success(successMessage)
    } catch (error) {
      if (!save.isCurrent(request) || recoverSession(error, navigate)) return
      console.warn(`${entityName}删除失败`, { id: item.id, error })
      toast.error(isApiError(error) ? apiErrorMessage(error) : errorMessage)
    } finally {
      save.finish(request)
    }
  }

  return { item, select: setItem, pending: save.saving, confirm }
}
