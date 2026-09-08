/** 移动端群成员头像预览和独立的成员搜索列表。 */
import { useState } from "react"
import { ChevronRightIcon, MinusIcon, PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useOutletContext } from "react-router"
import {
  GroupParticipantRole,
  OrganizationIdentityType,
  type GroupConversationData,
} from "@/api"
import type { MobileGroupDetailsContext } from "@/apps/mobile/mobile-group-context"
import { MobilePageHeader, MobileScrollArea } from "@/apps/mobile/mobile-page"
import { ProfileAvatar } from "@/components/profile-avatar"
import { Input } from "@/components/ui/input"

/** 展示最多两行头像、群主添加成员入口和移除占位。 */
export function MobileGroupMembersPreview({
  group,
  isOwner,
  returnDepth,
  canAdd,
}: {
  group: GroupConversationData
  isOwner: boolean
  returnDepth: number
  canAdd: boolean
}) {
  const { t } = useTranslation("mobile")
  const navigate = useNavigate()
  const visible = group.participants.slice(0, isOwner ? 8 : 9)
  return (
    <div className="border-b px-4 pt-5">
      <ul className="grid grid-cols-5 gap-x-3 gap-y-4">
        {visible.map((member) => (
          <li
            key={member.identityId}
            className="flex min-w-0 flex-col items-center gap-1.5"
          >
            <ProfileAvatar
              name={member.displayName}
              imageURL={member.avatarUrl}
              fallback={
                member.identityType ===
                OrganizationIdentityType.OrganizationIdentityTypeAgent
                  ? "agent"
                  : "person"
              }
              className="size-12 rounded-xl"
            />
            <span
              className="w-full truncate text-center text-xs"
              title={member.displayName}
            >
              {member.displayName}
            </span>
          </li>
        ))}
        {(isOwner ? (["add", "remove"] as const) : (["add"] as const)).map(
          (action) => (
            <li key={action} className="min-w-0">
              <button
                type="button"
                disabled={action !== "add" || !canAdd}
                onClick={
                  action === "add"
                    ? () => navigate("add-members", {
                        replace: returnDepth === 0,
                        state: {
                          mobileBack: returnDepth > 0,
                          groupReturnDepth: returnDepth > 0 ? returnDepth + 1 : 0,
                        },
                      })
                    : undefined
                }
                className="flex w-full flex-col items-center gap-1.5 rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:text-muted-foreground disabled:opacity-50"
              >
                <span className="flex size-12 items-center justify-center rounded-xl border border-dashed">
                  {action === "add" ? (
                    <PlusIcon className="size-6" />
                  ) : (
                    <MinusIcon className="size-6" />
                  )}
                </span>
                <span className="text-xs">
                  {t(action === "add" ? "group.add" : "group.remove")}
                </span>
              </button>
            </li>
          ),
        )}
      </ul>
      <Link
        to="members"
        replace={returnDepth === 0}
        state={{
          mobileBack: returnDepth > 0,
          groupReturnDepth: returnDepth > 0 ? returnDepth + 1 : 0,
        }}
        className="mt-4 flex min-h-14 items-center justify-between gap-3 text-sm outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span>
          {t("group.viewMembers", { count: group.participants.length })}
        </span>
        <ChevronRightIcon className="size-5 shrink-0 text-muted-foreground" />
      </Link>
    </div>
  )
}

/** 在独立页面搜索和展示全部群成员及群主身份。 */
export function MobileGroupMembersPage() {
  const { t } = useTranslation("inbox")
  const { group } = useOutletContext<MobileGroupDetailsContext>()
  const [search, setSearch] = useState("")
  const visible = group.participants.filter((member) =>
    member.displayName
      .toLocaleLowerCase()
      .includes(search.trim().toLocaleLowerCase()),
  )
  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        title={`${t("contextGroupMembersTab")} (${group.participants.length})`}
        backTo={`/inbox/group/${group.id}/details`}
      />
      <div className="space-y-2 border-b p-4">
        <label htmlFor="mobile-group-member-search" className="block text-sm">
          {t("groupMemberSearch")}
        </label>
        <Input
          id="mobile-group-member-search"
          type="search"
          className="min-h-11"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <MobileScrollArea storageKey={`group-members:${group.id}:${search}`}>
        <ul className="divide-y">
          {visible.map((member) => (
            <li
              key={member.identityId}
              className="flex min-h-16 items-center gap-3 px-4 py-2"
            >
              <ProfileAvatar
                name={member.displayName}
                imageURL={member.avatarUrl}
                fallback={
                  member.identityType ===
                  OrganizationIdentityType.OrganizationIdentityTypeAgent
                    ? "agent"
                    : "person"
                }
                className="size-10"
              />
              <span className="min-w-0 flex-1 break-words text-sm">
                {member.displayName}
              </span>
              {member.role ===
              GroupParticipantRole.GroupParticipantRoleOwner ? (
                <span className="shrink-0 text-xs text-muted-foreground">
                  {t("groupOwner")}
                </span>
              ) : null}
            </li>
          ))}
        </ul>
        {!visible.length ? (
          <p className="px-4 py-8 text-center text-sm text-muted-foreground">
            {t("groupMembersNoMatches")}
          </p>
        ) : null}
      </MobileScrollArea>
    </section>
  )
}
