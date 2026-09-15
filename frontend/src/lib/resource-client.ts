/** 全局查询缓存客户端，缓存生命周期与登录会话绑定。 */
import { QueryClient } from "@tanstack/react-query"

import { advanceSessionGeneration } from "@/api/session-scope"

/**
 * 进程级查询客户端。
 * 默认单次请求，结果在页面存活期间保持新鲜。
 * 调用方通过 refresh、失效资源或按 key 指定 staleTime 控制刷新。
 */
export const resourceClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: Infinity,
      gcTime: 5 * 60 * 1000,
    },
  },
})

/** 进入新的登录会话：先提升会话代次让实时连接等订阅方清理，再清空全部查询缓存。 */
export function beginSessionBoundary() {
  advanceSessionGeneration()
  resourceClient.clear()
}
