/** 集中维护企业成员变更影响的目录、组织关系和候选数据。 */
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"

/** 刷新真人或 AI 员工及其团队、角色、会话候选和知识库使用情况。 */
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
      resourceKeys.serviceQueueTeams(),
      resourceKeys.memberOptions(),
    ]
    // AI 员工的名称、状态和知识库绑定变化后刷新知识库的员工列表。
    if (kind === "agent") keys.push(resourceKeys.knowledgeBaseAgents())
    if (id) keys.push(kind === "user" ? resourceKeys.user(id) : resourceKeys.agent(id))
    return Promise.all(keys.map((key) => invalidate(key)))
  }
}
