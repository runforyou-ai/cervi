/** 列表查询参数中的页码解析。 */

/** 把查询参数中的页码解析为正整数，缺省或无效时为第一页。 */
export function parseListPage(value: string | null) {
  const page = Number(value ?? 1)
  return Number.isSafeInteger(page) && page > 0 ? page : 1
}
