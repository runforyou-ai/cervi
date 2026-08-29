/** 列表搜索输入与 URL 查询参数同步。 */
import { useCallback, useEffect, useState } from "react"
import { useSearchParams } from "react-router"

const listSearchDelay = 300

/** 管理列表查询参数，并把搜索输入防抖同步到地址栏。 */
export function useListSearchParams(resetParameter?: string) {
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""
  const [search, setSearch] = useState(query)

  /** 更新列表查询参数。 */
  const setParameters = useCallback(
    (changes: Record<string, string | null>, replace = false) => {
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
        { replace },
      )
    },
    [setSearchParams],
  )

  useEffect(() => setSearch(query), [query])
  useEffect(() => {
    if (search === query) return
    const timeout = window.setTimeout(() => {
      const changes: Record<string, string | null> = {
        q: search || null,
        page: null,
      }
      if (resetParameter) changes[resetParameter] = null
      setParameters(changes)
    }, listSearchDelay)
    return () => window.clearTimeout(timeout)
  }, [query, resetParameter, search, setParameters])

  return { searchParams, setParameters, query, search, setSearch }
}
