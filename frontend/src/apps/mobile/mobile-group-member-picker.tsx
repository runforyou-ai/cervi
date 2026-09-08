/** 移动端建群和添加群成员共用的真人搜索与多选。 */
import { groupAdditionalMemberMaxCount } from "@/features/inbox/group-conversation-schema"
import { useState, type Ref } from "react"
import { useTranslation } from "react-i18next"

import { OrganizationIdentityType, type MemberOption } from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 保留跨搜索的列表勾选，建群时展示已选成员摘要。 */
export function MobileGroupMemberPicker({
  currentIdentityID,
  selected,
  onChange,
  onBlur,
  inputRef,
  disabled,
  excludedIdentityIDs = [],
  selectionLimit = groupAdditionalMemberMaxCount,
  showSelectionSummary = true,
}: {
  currentIdentityID: string
  selected: MemberOption[]
  onChange: (members: MemberOption[]) => void
  onBlur: () => void
  inputRef: Ref<HTMLInputElement>
  disabled: boolean
  excludedIdentityIDs?: string[]
  selectionLimit?: number
  showSelectionSummary?: boolean
}) {
  const { t } = useTranslation(["mobile", "common"])
  const { t: tInbox } = useTranslation("inbox")
  const [search, setSearch] = useState("")
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { staleTime: 0 },
  )
  const members = (data ?? []).filter(
    (member) =>
      member.type === OrganizationIdentityType.OrganizationIdentityTypeUser &&
      member.id !== currentIdentityID &&
      !excludedIdentityIDs.includes(member.id),
  )
  const query = search.trim().toLocaleLowerCase()
  const candidates = members.filter((member) =>
    member.displayName.toLocaleLowerCase().includes(query),
  )

  return (
    <div className="space-y-2">
      {showSelectionSummary ? (
        <>
          <div className="flex items-center justify-between gap-2">
            <FieldLabel htmlFor="mobile-group-members" required>
              {tInbox("groupMembersLabel")}
            </FieldLabel>
            <span className="text-xs text-muted-foreground" role="status">
              {tInbox("groupMembersSelected", { count: selected.length })}
            </span>
          </div>
          <div className="h-20 overflow-y-auto" aria-label={t("group.selectedMembers")}>
            {selected.length ? (
              <ul className="flex flex-wrap gap-2">
                {selected.map((member) => (
                  <li key={member.id} className="max-w-full">
                    <Button
                      type="button"
                      variant="outline"
                      className="min-h-11 max-w-full"
                      disabled={disabled}
                      aria-label={t("group.removeMember", { name: member.displayName })}
                      onClick={() =>
                        onChange(selected.filter((item) => item.id !== member.id))
                      }
                    >
                      <span className="truncate">{member.displayName}</span>
                      <span className="text-muted-foreground">
                        {t("common:actions.remove")}
                      </span>
                    </Button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="py-3 text-sm text-muted-foreground">
                {t("group.noSelection")}
              </p>
            )}
          </div>
        </>
      ) : null}
      <div className="space-y-2">
        <label htmlFor="mobile-group-members" className="text-sm">
          {t("group.searchMembers")}
        </label>
        <Input
          id="mobile-group-members"
          ref={inputRef}
          value={search}
          type="search"
          className="min-h-11 md:text-base"
          autoComplete="off"
          disabled={disabled}
          onBlur={onBlur}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>
      <div className="h-64 overflow-y-auto overscroll-contain rounded-md border">
        {loading && !data ? (
          <LoadingIndicator className="h-full justify-center">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : null}
        {error ? (
          <div className="space-y-3 p-4 text-sm">
            <p>{tInbox("groupMembersLoadError")}</p>
            <Button
              type="button"
              variant="outline"
              className="min-h-11"
              disabled={refreshing || disabled}
              onClick={() => void refresh()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : null}
        {data && !error && !candidates.length ? (
          <p className="p-4 text-sm text-muted-foreground">
            {tInbox("groupMembersEmpty")}
          </p>
        ) : null}
        <ul className="divide-y">
          {candidates.map((member) => {
            const checked = selected.some((item) => item.id === member.id)
            return (
              <li key={member.id}>
                <label className="flex min-h-16 items-center gap-3 px-3 py-2 active:bg-muted">
                  <input
                    type="checkbox"
                    checked={checked}
                    name="members"
                    className="size-5 shrink-0 accent-primary"
                    disabled={disabled || (!checked && selected.length >= selectionLimit)}
                    onBlur={onBlur}
                    onChange={(event) =>
                      onChange(
                        event.target.checked
                          ? [...selected, member]
                          : selected.filter((item) => item.id !== member.id),
                      )
                    }
                  />
                  <ProfileAvatar
                    name={member.displayName}
                    imageURL={member.avatarUrl}
                    className="size-9"
                  />
                  <span className="min-w-0 truncate text-sm">{member.displayName}</span>
                </label>
              </li>
            )
          })}
        </ul>
      </div>
    </div>
  )
}
