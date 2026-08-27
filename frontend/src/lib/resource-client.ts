/** 全局查询缓存客户端，缓存生命周期与登录会话绑定。 */
import { QueryClient } from "@tanstack/react-query"

/**
 * 进程级查询客户端。
 * 业务错误不重试；窗口聚焦不自动刷新；结果默认在页面存活期间常驻，
 * 由调用方通过 refresh、invalidateResource 或按 key 指定 staleTime 主动失效。
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

/** 清空全部查询缓存；登录、登出和企业初始化等会话边界必须调用，避免跨账号复用数据。 */
export function resetResourceCache() {
  resourceClient.clear()
}
