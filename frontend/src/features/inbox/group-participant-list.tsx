/** 群聊资料栏中的当前成员列表。 */
import { CrownIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { getGroupConversation, GroupParticipantRole } from "@/api"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 读取并展示群聊当前成员。 */
export function GroupParticipantList({
  conversationID,
}: {
  conversationID: string
}) {
  const { t } = useTranslation("inbox")
  const { data, loading, error, refresh } = useResource(
    resourceKeys.groupConversation(conversationID),
    () => getGroupConversation(conversationID),
  )

  if (loading) {
    return (
      <div className="flex min-h-48 items-center justify-center text-sm text-muted-foreground">
        {t("groupMembersLoading")}
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center p-6 text-center">
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
    )
  }

  return (
    <div className="grid p-1.5">
      {data.participants.map((participant) => (
        <div
          key={participant.identityId}
          className="flex items-center gap-3 rounded-md px-2 py-2.5"
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
  )
}
