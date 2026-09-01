/** 收件箱会话选择器共用的企业身份候选读取。 */
import { listMemberOptions, type MemberOption } from "@/api"

const memberOptionPageSize = 100

/** 分页读取全部可用企业身份候选项。 */
export async function listAllMemberOptions() {
  const members: MemberOption[] = []
  let page = 1
  let pages = 1
  do {
    const output = await listMemberOptions({
      page,
      pageSize: memberOptionPageSize,
    })
    members.push(...output.members)
    pages = Math.ceil(output.page.total / memberOptionPageSize)
    page += 1
  } while (page <= pages)
  return members
}
