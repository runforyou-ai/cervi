/** 群聊当前成员的只读列表。 */
import { useState } from "react"
import { CrownIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { getGroupConversation, GroupParticipantRole } from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 展示群聊成员数量入口和只读成员弹窗。 */
export function GroupParticipantDialog({
  conversationID,
  title,
  memberCount,
}: {
  conversationID: string
  title: string
  memberCount: number
}) {
  const { t } = useTranslation("inbox")
  const [open, setOpen] = useState(false)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.groupConversation(conversationID),
    () => getGroupConversation(conversationID),
    { enabled: open },
  )

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button
        type="button"
        variant="link"
        size="sm"
        className="h-auto justify-start p-0 text-xs font-normal text-muted-foreground"
        onClick={() => setOpen(true)}
      >
        {t("groupMemberCount", { count: memberCount })}
      </Button>
      <DialogContent className="grid max-h-[min(38rem,calc(100svh-2rem))] grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{t("groupMembersDescription")}</DialogDescription>
        </DialogHeader>
        <ScrollArea className="min-h-64 rounded-md border">
          {loading ? (
            <div className="flex min-h-64 items-center justify-center text-sm text-muted-foreground">
              {t("groupMembersLoading")}
            </div>
          ) : error || !data ? (
            <div className="flex min-h-64 flex-col items-center justify-center p-6 text-center">
              <p className="text-sm text-muted-foreground">
                {t("groupMembersLoadError")}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-3"
                onClick={() => void refresh()}
              >
                {t("messagesRetry")}
              </Button>
            </div>
          ) : (
            <div className="grid p-1.5">
              {data.participants.map((participant) => (
                <div
                  key={participant.identityId}
                  className="flex items-center gap-3 rounded-md px-3 py-2.5"
                >
                  <span className="flex size-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-sm font-medium text-muted-foreground">
                    {participant.avatarUrl ? (
                      <img
                        src={participant.avatarUrl}
                        alt=""
                        className="size-full object-cover"
                        draggable={false}
                      />
                    ) : participant.displayName ? (
                      Array.from(participant.displayName)[0]?.toLocaleUpperCase()
                    ) : (
                      <UserRoundIcon className="size-4" />
                    )}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm">
                    {participant.displayName}
                  </span>
                  {participant.role ===
                  GroupParticipantRole.GroupParticipantRoleOwner ? (
                    <span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
                      <CrownIcon className="size-3.5" />
                      {t("groupOwner")}
                    </span>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
