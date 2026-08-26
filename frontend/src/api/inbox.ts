/** 成员收件箱调用与归一化。 */
import { LoadInbox } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  Inbox,
  InboxConversation,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: InboxConversation[]
}

const loadInboxBound = bind(LoadInbox)

/** 读取成员收件箱的客户会话列表。 */
export async function loadInbox(): Promise<InboxData> {
  const inbox = await loadInboxBound()
  return {
    ...inbox,
    conversations: asList(inbox.conversations),
  }
}
