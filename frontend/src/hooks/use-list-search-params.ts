/** 列表搜索输入与 URL 查询参数同步。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { useLocation, useSearchParams } from "react-router"

import { useDebouncedValue } from "@/hooks/use-debounced-value"

const listSearchDelay = 300

/**
 * 管理列表查询参数，并把搜索输入防抖同步到地址栏。
 * 搜索同步替换当前历史记录并保留页面的导航 state，同时清空 resetParameter；
 * 检索词变化时先以新检索词调用 onQueryChange，供调用方重置该查询的缓存进度。
 */
export function useListSearchParams({
  resetParameter,
  onQueryChange,
}: {
  resetParameter?: string
  onQueryChange?: (query: string) => void
} = {}) {
  const [searchParams, setSearchParams] = useSearchParams()
  const location = useLocation()
  const query = searchParams.get("q") ?? ""
  const [search, setSearch] = useState(query)
  const latest = useRef({ state: location.state as unknown, onQueryChange })
  latest.current = { state: location.state, onQueryChange }

  /** 更新列表查询参数。 */
  const setParameters = useCallback(
    (
      changes: Record<string, string | null>,
      replace = false,
      state?: unknown,
    ) => {
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current)
          for (const [name, value] of Object.entries(changes)) {
            if (!value) {
              next.delete(name)
            } else {
              next.set(name, value)
            }
          }
          return next
        },
        { replace, state },
      )
    },
    [setSearchParams],
  )

  const settledSearch = useDebouncedValue(search, listSearchDelay)
  useEffect(() => setSearch(query), [query])
  // 输入停顿后写入地址栏；地址栏先行变化时等输入追上后再比较。
  useEffect(() => {
    if (settledSearch !== search || settledSearch === query) return
    if (settledSearch.trim() !== query.trim())
      latest.current.onQueryChange?.(settledSearch.trim())
    const changes: Record<string, string | null> = {
      q: settledSearch || null,
    }
    if (resetParameter) changes[resetParameter] = null
    setParameters(changes, true, latest.current.state)
  }, [query, resetParameter, search, settledSearch, setParameters])

  return { searchParams, setParameters, query, search, setSearch }
}
