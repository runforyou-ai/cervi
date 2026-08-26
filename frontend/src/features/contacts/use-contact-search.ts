/** 通讯录列表的查询参数管理和搜索输入防抖。 */
import { useCallback, useEffect, useState } from "react"
import { useSearchParams } from "react-router"

/** 管理列表 URL 查询参数，并把搜索输入防抖同步到地址栏。 */
export function useContactSearch() {
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get("q") ?? ""
  const [search, setSearch] = useState(query)

  /** 更新列表查询参数。 */
  const setParameters = useCallback(
    (changes: Record<string, string | null>) => {
      setSearchParams((current) => {
        const next = new URLSearchParams(current)
        for (const [name, value] of Object.entries(changes)) {
          if (!value) {
            next.delete(name)
          } else {
            next.set(name, value)
          }
        }
        return next
      })
    },
    [setSearchParams],
  )

  useEffect(() => setSearch(query), [query])
  useEffect(() => {
    const timeout = window.setTimeout(() => {
      if (search !== query) {
        setParameters({ q: search || null, page: null, selected: null })
      }
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [query, search, setParameters])

  const currentPage = Number(searchParams.get("page") ?? "1") || 1
  const selected = searchParams.get("selected") ?? ""

  return {
    searchParams,
    setParameters,
    query,
    search,
    setSearch,
    currentPage,
    selected,
  }
}
