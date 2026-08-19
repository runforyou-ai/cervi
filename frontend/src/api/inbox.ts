import { LoadInbox } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  Conversation,
  Inbox,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"

export { MessageAuthor } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
export type {
  Message,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

export type ConversationData = Omit<Conversation, "messages"> & {
  messages: NonNullable<Conversation["messages"]>
}

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: ConversationData[]
}

export type { ConversationData as Conversation }

export async function loadInbox(): Promise<InboxData> {
  const inbox = await call((meta) => LoadInbox(meta))
  return {
    ...inbox,
    conversations: (inbox.conversations ?? []).map((conversation) => ({
      ...conversation,
      messages: conversation.messages ?? [],
    })),
  }
}
