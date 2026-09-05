/** 统一的页面数据读取 hook，封装 TanStack Query 并约束项目取数行为。 */
import { useCallback, useEffect } from "react"
import { useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query"
import { useNavigate } from "react-router"

import { recoverSession } from "@/lib/session-navigation"

/**
 * 读取一份以 key 标识的页面数据。
 * key 相同的调用在多标签间共享缓存；load 收到的 signal 只用于忽略过期结果，
 * 不取消 Wails 绑定调用。带会话状态的读取错误统一导航回对应入口。
 * 跨业务域的选项类数据用 staleTime: 0 让每次挂载都重新读取。
 */
export function useResource<T>(
  key: QueryKey,
  load: (signal: AbortSignal) => Promise<T>,
  options: {
    enabled?: boolean
    staleTime?: number
    refetchInterval?: number | false
    refetchOnWindowFocus?: boolean
  } = {},
) {
  const navigate = useNavigate()
  const client = useQueryClient()
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => load(signal),
    enabled: options.enabled,
    staleTime: options.staleTime,
    refetchInterval: options.refetchInterval,
    refetchOnWindowFocus: options.refetchOnWindowFocus,
  })

  const sessionError = query.error
  useEffect(() => {
    if (sessionError) {
      recoverSession(sessionError, navigate)
    }
  }, [sessionError, navigate])

  /** 按统一资源 key 执行交互触发的读取，共享缓存及会话错误恢复。 */
  const read = useCallback(
    async <R>(
      resourceKey: QueryKey,
      loader: (signal: AbortSignal) => Promise<R>,
    ) => {
      try {
        return await client.fetchQuery({
          queryKey: resourceKey,
          queryFn: ({ signal }) => loader(signal),
          staleTime: 0,
        })
      } catch (error) {
        recoverSession(error, navigate)
        throw error
      }
    },
    [client, navigate],
  )

  return {
    data: query.data,
    loading: query.isPending && query.isFetching,
    refreshing: query.isFetching && !query.isPending,
    error: query.error,
    refresh: query.refetch,
    read,
  }
}

type ResourceInvalidationOptions = {
  exact?: boolean
  refetchType?: "active" | "inactive" | "all" | "none"
}

/** 返回按 key 失效缓存的函数，供数据变更后刷新相关读取。 */
export function useResourceInvalidator() {
  const client = useQueryClient()
  return (key: QueryKey, options: ResourceInvalidationOptions = {}) =>
    client.invalidateQueries({ queryKey: key, ...options })
}
