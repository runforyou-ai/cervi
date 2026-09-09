/** AI 员工表单的通讯录返回地址。 */

/** 保留来源查询条件，并将返回目标限定为通讯录列表或团队页。 */
export function agentReturnPath(pathname: string, search: string) {
  const params = new URLSearchParams(search)
  const candidate = params.get("returnTo") ?? `${pathname}${search}`
  const candidatePath = candidate.split("?")[0]
  return /^\/contacts\/(employees|ai-employees|external|teams\/[^/?#]+)$/.test(
    candidatePath,
  )
    ? candidate
    : "/contacts/ai-employees"
}
