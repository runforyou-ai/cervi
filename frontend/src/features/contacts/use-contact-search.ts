/** 通讯录列表查询参数适配。 */
import { useListSearchParams } from "@/hooks/use-list-search-params"
import { parseListPage } from "@/lib/list-page"

/** 补充通讯录列表的分页和选中项查询参数。 */
export function useContactSearch() {
  const listSearch = useListSearchParams({ resetParameter: "selected" })
  const { searchParams } = listSearch
  const currentPage = parseListPage(searchParams.get("page"))
  const selected = searchParams.get("selected") ?? ""

  return {
    ...listSearch,
    currentPage,
    selected,
  }
}
