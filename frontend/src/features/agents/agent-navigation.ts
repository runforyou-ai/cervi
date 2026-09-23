/** AI 员工表单的返回地址。 */

/** 保留来源查询条件，并将返回目标限定为 AI 员工列表或团队页。 */
export function agentReturnPath(pathname: string, search: string) {
  const params = new URLSearchParams(search)
  const candidate = params.get("returnTo") ?? `${pathname}${search}`
  const candidatePath = candidate.split("?")[0]
  return /^\/(ai-employees|contacts\/teams\/[^/?#]+)$/.test(candidatePath)
    ? candidate
    : "/ai-employees"
}
