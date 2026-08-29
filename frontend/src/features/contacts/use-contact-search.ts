/** 通讯录列表查询参数适配。 */
import { useListSearchParams } from "@/hooks/use-list-search-params"

/** 补充通讯录列表的分页和选中项查询参数。 */
export function useContactSearch() {
  const listSearch = useListSearchParams("selected")
  const { searchParams } = listSearch
  const currentPage = Number(searchParams.get("page") ?? "1") || 1
  const selected = searchParams.get("selected") ?? ""

  return {
    ...listSearch,
    currentPage,
    selected,
  }
}
