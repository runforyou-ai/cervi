/** 集中维护企业成员变更影响的目录、组织关系和候选数据。 */
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 刷新真人或 AI 员工及其团队、角色和会话候选。 */
export function useContactInvalidator() {
  const invalidate = useResourceInvalidator()
  return (kind: "user" | "agent", id?: string) => {
    const keys = [
      kind === "user" ? resourceKeys.users() : resourceKeys.agents(),
      resourceKeys.teams(),
      resourceKeys.teamMembers(),
      resourceKeys.teamMemberCandidates(),
      resourceKeys.roles(),
      resourceKeys.roleMembers(),
      resourceKeys.customerServiceAssignees(),
      resourceKeys.memberOptions(),
    ]
    if (id) keys.push(kind === "user" ? resourceKeys.user(id) : resourceKeys.agent(id))
    return Promise.all(keys.map((key) => invalidate(key)))
  }
}
