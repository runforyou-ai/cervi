/** 移动端企业成员搜索、滚动加载和资料入口。 */
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react"
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useSearchParams } from "react-router"

import { listUsers, UserStatus } from "@/api"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import {
  MobilePageHeader,
  MobilePageState,
  MobileScrollArea,
} from "@/apps/mobile/mobile-page"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 防抖同步搜索条件，列表滚动加载后续成员。 */
export function MobileEmployeesPage() {
  const { t } = useTranslation("mobile")
  const location = useLocation()
  const { listPageCounts, scrollPositions } = useMobileNavigation()
  const [params, setParams] = useSearchParams()
  const queryText = params.get("q") ?? ""
  const [search, setSearch] = useState(queryText)

  useEffect(() => setSearch(queryText), [queryText])
  useEffect(() => {
    if (search === queryText) return
    // 停止输入后再查询，替换当前历史并保留通讯录返回来源。
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams()
      if (search) next.set("q", search)
      if (search.trim() !== queryText.trim()) {
        const storageKey = `employees:${search.trim()}`
        listPageCounts.delete(storageKey)
        scrollPositions.delete(storageKey)
      }
      setParams(next, { replace: true, state: location.state })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [search, queryText, setParams, location.state, listPageCounts, scrollPositions])

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("contacts.employees")} backTo="/contacts" />
      <div className="shrink-0 space-y-2 border-b px-4 py-3">
        <Label htmlFor="mobile-employee-search">{t("contacts.search")}</Label>
        <Input
          id="mobile-employee-search"
          type="search"
          className="min-h-11 md:text-base"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <MobileEmployeeList
        key={queryText.trim()}
        queryText={queryText.trim()}
        searching={search !== queryText}
      />
    </section>
  )
}

/** 保留各搜索结果已加载的页数，恢复全部原有页面后再恢复滚动位置。 */
function MobileEmployeeList({
  queryText,
  searching,
}: {
  queryText: string
  searching: boolean
}) {
  const { listPageCounts } = useMobileNavigation()
  const storageKey = `employees:${queryText}`
  const [pageCount, setPageCount] = useState(
    () => listPageCounts.get(storageKey) ?? 1,
  )
  const initialPageCount = useRef(pageCount)
  const loadedPages = useRef(new Set<number>())
  const [ready, setReady] = useState(false)

  useLayoutEffect(() => {
    listPageCounts.set(storageKey, pageCount)
  }, [listPageCounts, storageKey, pageCount])

  /** 初始页面全部就绪后开放位置记录，后续追加不会重新定位。 */
  const onReady = useCallback((page: number) => {
    loadedPages.current.add(page)
    if (loadedPages.current.size >= initialPageCount.current) setReady(true)
  }, [])

  /** 一次只追加当前尾页的下一页，忽略旧观察回调。 */
  const loadMore = useCallback((page: number) => {
    setPageCount((current) => (current === page ? current + 1 : current))
  }, [])

  return (
    <MobileScrollArea storageKey={storageKey} ready={ready}>
      {Array.from({ length: pageCount }, (_, index) => (
        <MobileEmployeePage
          key={index + 1}
          page={index + 1}
          queryText={queryText}
          last={index + 1 === pageCount}
          autoLoad={ready && !searching}
          onReady={onReady}
          onLoadMore={loadMore}
        />
      ))}
    </MobileScrollArea>
  )
}

/** 按统一资源 key 缓存一页成员，尾部进入滚动视口时加载下一页。 */
function MobileEmployeePage({
  page,
  queryText,
  last,
  autoLoad,
  onReady,
  onLoadMore,
}: {
  page: number
  queryText: string
  last: boolean
  autoLoad: boolean
  onReady: (page: number) => void
  onLoadMore: (page: number) => void
}) {
  const { t } = useTranslation("mobile")
  const sentinel = useRef<HTMLDivElement>(null)
  const query = {
    query: queryText,
    status: UserStatus.UserStatusActive,
    page,
    pageSize: 50,
  }
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.users(query),
    () => listUsers(query),
  )
  const hasMore = Boolean(
    data &&
    data.users.length > 0 &&
    data.page.number * data.page.size < data.page.total,
  )

  useLayoutEffect(() => {
    if (data) onReady(page)
  }, [data, onReady, page])
  useEffect(() => {
    const element = sentinel.current
    if (!element || !last || !autoLoad || !hasMore || error || refreshing)
      return
    // 各页使用 Fragment，哨兵始终是列表滚动容器的直接子节点。
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) onLoadMore(page)
      },
      { root: element.parentElement, rootMargin: "0px 0px 160px 0px" },
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [last, autoLoad, hasMore, error, refreshing, page, onLoadMore])

  return (
    <>
      {loading && !data ? (
        <LoadingIndicator
          className={
            page === 1 ? "min-h-64 justify-center" : "h-14 justify-center"
          }
        >
          {t(page === 1 ? "loading" : "contacts.loadingMore")}
        </LoadingIndicator>
      ) : null}
      {error && !data && page === 1 ? (
        <MobilePageState
          title={t("contacts.loadError")}
          onRetry={() => void refresh()}
        />
      ) : error ? (
        <div className="flex h-14 items-center justify-center">
          <Button
            variant="outline"
            className="min-h-11"
            disabled={loading || refreshing}
            onClick={() => void refresh()}
          >
            {t("contacts.loadMoreError")} · {t("retry")}
          </Button>
        </div>
      ) : null}
      {data && page === 1 && data.users.length === 0 && !error ? (
        <MobilePageState title={t("contacts.empty")} />
      ) : null}
      {data && data.users.length > 0 ? (
        <ul className="divide-y border-b">
          {data.users.map((user) => (
            <li key={user.id}>
              <Link
                to={`/contacts/employees/${user.id}`}
                state={{ mobileBack: true }}
                className="flex min-h-18 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              >
                <span
                  className="flex size-10 shrink-0 items-center justify-center rounded-full bg-primary/10 font-medium text-primary"
                  aria-hidden="true"
                >
                  {Array.from(user.displayName)[0]?.toLocaleUpperCase()}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[15px] font-medium">
                    {user.displayName}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    {user.email}
                  </span>
                </span>
                <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
      {last && data && !error && (page > 1 || data.users.length > 0) ? (
        <div
          ref={sentinel}
          className="flex h-14 items-center justify-center text-sm text-muted-foreground"
          role="status"
        >
          {!hasMore ? t("contacts.allLoaded") : null}
        </div>
      ) : null}
    </>
  )
}
