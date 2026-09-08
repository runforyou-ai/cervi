/** 移动端群成员预览下方的群资料和个人、群主管理入口。 */
import { ChevronRightIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { GroupParticipantRole, type GroupConversationData } from "@/api"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { GroupAvatar } from "@/features/inbox/group-avatar"
import { useDateTime } from "@/hooks/use-date-time"

/** 紧凑展示群资料，资料操作与免打扰保持统一表单行布局。 */
export function MobileGroupInfo({
  group,
  isOwner,
  archived,
  busy,
  muted,
  muteBusy,
  onEdit,
  onLeave,
  onMute,
}: {
  group: GroupConversationData
  isOwner: boolean
  archived: boolean
  busy: boolean
  muted: boolean
  muteBusy: boolean
  onEdit: (field: "image" | "title" | "description") => void
  onLeave: (trigger: HTMLElement | null) => void
  onMute: (muted: boolean) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tm } = useTranslation("mobile")
  const { formatDateTime } = useDateTime()
  const owner = group.participants.find(
    (member) => member.role === GroupParticipantRole.GroupParticipantRoleOwner,
  )
  return (
    <div className="px-4 pb-4">
      <div className="divide-y text-sm">
        {(["image", "title", "description"] as const).map((field) => {
          const label = t(
            field === "image"
              ? "groupImageLabel"
              : field === "title"
                ? "groupTitleLabel"
                : "groupDescriptionLabel",
          )
          const content = (
            <>
              <span className="w-20 shrink-0 text-muted-foreground">
                {label}
              </span>
              <span className="min-w-0 flex-1 whitespace-pre-wrap break-words text-right">
                {field === "image" ? (
                  <GroupAvatar
                    imageURL={group.imageUrl}
                    className="ml-auto size-12 rounded-xl"
                  />
                ) : field === "title" ? (
                  group.title
                ) : (
                  group.description || t("groupDescriptionEmpty")
                )}
              </span>
              <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" />
            </>
          )
          return (
            <button
              key={field}
              type="button"
              disabled={busy}
              onClick={() => onEdit(field)}
              className="flex min-h-14 w-full items-center gap-3 py-3 text-left outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
            >
              {content}
            </button>
          )
        })}
        <div className="flex min-h-14 items-center gap-3 py-3">
          <span className="w-20 shrink-0 text-muted-foreground">
            {t("groupOwner")}
          </span>
          <span className="min-w-0 flex-1 break-words text-right">
            {owner?.displayName ?? "—"}
          </span>
        </div>
        <div className="flex min-h-14 items-center gap-3 py-3">
          <span className="w-20 shrink-0 text-muted-foreground">
            {t("groupCreatedAt")}
          </span>
          <span className="min-w-0 flex-1 text-right">
            {formatDateTime(group.createdAt)}
          </span>
        </div>
      </div>
      {archived ? (
        <p className="py-3 text-sm text-muted-foreground" role="status">
          {tm("group.archived")}
        </p>
      ) : null}
      <div className="flex min-h-14 items-center justify-between gap-3 border-t text-sm">
        <label
          className="flex min-h-14 flex-1 items-center"
          htmlFor="mobile-group-muted"
        >
          {tm("group.mute")}
        </label>
        <Switch
          id="mobile-group-muted"
          className="relative h-7 w-12 border-0 px-0.5 disabled:opacity-100 after:absolute after:inset-x-0 after:-inset-y-2 after:content-[''] [&_[data-slot=switch-thumb]]:size-6 [&_[data-slot=switch-thumb][data-state=checked]]:translate-x-5"
          checked={muted}
          disabled={muteBusy}
          onCheckedChange={onMute}
        />
      </div>
      <Button
        type="button"
        variant="destructive"
        className="mt-4 min-h-11 w-full"
        disabled={archived || busy}
        onClick={(event) => onLeave(event.currentTarget)}
      >
        {t(isOwner ? "groupDissolve" : "groupLeave")}
      </Button>
    </div>
  )
}
