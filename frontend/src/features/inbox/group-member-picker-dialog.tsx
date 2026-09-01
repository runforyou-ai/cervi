/** 群聊成员的搜索、多选和批量添加对话框。 */
import { useId, useMemo, useState } from "react"
import { SearchIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  OrganizationIdentityType,
  type GroupParticipant,
  type MemberOption,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupMemberMaxCount = 100

/** 选择尚未加入群聊的企业成员。 */
export function GroupMemberPickerDialog({
  open,
  participants,
  onOpenChange,
  onAdd,
}: {
  open: boolean
  participants: GroupParticipant[]
  onOpenChange: (open: boolean) => void
  onAdd: (members: MemberOption[]) => Promise<void>
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const searchID = useId()
  const [query, setQuery] = useState("")
  const [selectedIdentityIDs, setSelectedIdentityIDs] = useState<Set<string>>(
    new Set(),
  )
  const [saving, setSaving] = useState(false)
  const resource = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { enabled: open, staleTime: 0 },
  )
  const participantIdentityIDs = useMemo(
    () =>
      new Set(participants.map((participant) => participant.identityId)),
    [participants],
  )
  const availableMembers = useMemo(
    () =>
      (resource.data ?? []).filter(
        (member) =>
          member.type ===
            OrganizationIdentityType.OrganizationIdentityTypeUser &&
          !participantIdentityIDs.has(member.id),
      ),
    [participantIdentityIDs, resource.data],
  )
  const normalizedQuery = query.trim().toLocaleLowerCase()
  const visibleMembers = availableMembers.filter(
    (member) =>
      !normalizedQuery ||
      member.displayName.toLocaleLowerCase().includes(normalizedQuery),
  )
  const remainingCount = Math.max(
    0,
    groupMemberMaxCount - participants.length,
  )

  /** 关闭时清空尚未提交的成员选择。 */
  function changeOpen(nextOpen: boolean) {
    if (!nextOpen) {
      setQuery("")
      setSelectedIdentityIDs(new Set())
    }
    onOpenChange(nextOpen)
  }

  /** 切换候选成员的选中状态。 */
  function toggleMember(identityID: string, checked: boolean) {
    setSelectedIdentityIDs((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(identityID)
      } else {
        next.delete(identityID)
      }
      return next
    })
  }

  /** 添加选中的群聊成员。 */
  async function addSelectedMembers() {
    if (selectedIdentityIDs.size === 0) return
    const selectedMembers = availableMembers.filter((member) =>
      selectedIdentityIDs.has(member.id),
    )
    setSaving(true)
    try {
      await onAdd(selectedMembers)
      changeOpen(false)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("添加群聊成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["memberIdentityIds"])
          : t("groupAddMembersError"),
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent className="max-h-[min(42rem,calc(100svh-2rem))] max-w-2xl overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("groupAddMembers")}</DialogTitle>
          <DialogDescription>
            {t("groupAddMembersDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="grid min-h-0 gap-4">
          <div className="grid gap-2">
            <label
              htmlFor={searchID}
              className="text-sm font-medium"
            >
              {t("groupMemberSearch")}
            </label>
            <div className="relative">
              <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                id={searchID}
                value={query}
                autoComplete="off"
                className="pl-9"
                onChange={(event) => setQuery(event.target.value)}
              />
            </div>
          </div>

          <ScrollArea className="h-64 rounded-md border">
            {resource.loading ? (
              <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
                {t("groupMembersLoading")}
              </div>
            ) : resource.error ? (
              <div className="flex h-64 flex-col items-center justify-center p-6 text-center">
                <p className="text-sm text-muted-foreground">
                  {t("groupMembersLoadError")}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="mt-3"
                  onClick={() => void resource.refresh()}
                >
                  {t("messagesRetry")}
                </Button>
              </div>
            ) : remainingCount === 0 || visibleMembers.length === 0 ? (
              <p className="px-6 py-12 text-center text-sm text-muted-foreground">
                {t(
                  remainingCount === 0
                    ? "groupMemberLimitReached"
                    : normalizedQuery
                      ? "groupMembersNoMatches"
                      : "groupMembersNoCandidates",
                )}
              </p>
            ) : (
              <div className="grid p-1.5">
                {visibleMembers.map((member) => {
                  const selected = selectedIdentityIDs.has(member.id)
                  const selectionFull =
                    !selected &&
                    selectedIdentityIDs.size >= remainingCount
                  return (
                    <label
                      key={member.id}
                      className="flex items-center gap-3 rounded-md px-3 py-2.5 transition-colors hover:bg-muted"
                    >
                      <input
                        type="checkbox"
                        checked={selected}
                        disabled={selectionFull || saving}
                        className="size-4 rounded border-input accent-primary disabled:cursor-not-allowed disabled:opacity-60"
                        aria-label={t("groupSelectMember", {
                          name: member.displayName,
                        })}
                        onChange={(event) =>
                          toggleMember(member.id, event.target.checked)
                        }
                      />
                      <span className="flex size-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-sm font-medium text-muted-foreground">
                        {member.avatarUrl ? (
                          <img
                            src={member.avatarUrl}
                            alt=""
                            className="size-full object-cover"
                            draggable={false}
                          />
                        ) : member.displayName ? (
                          Array.from(
                            member.displayName,
                          )[0]?.toLocaleUpperCase()
                        ) : (
                          <UserRoundIcon className="size-4" />
                        )}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-sm">
                        {member.displayName}
                      </span>
                    </label>
                  )
                })}
              </div>
            )}
          </ScrollArea>

          <div className="flex items-center justify-between gap-3">
            <span className="text-sm text-muted-foreground">
              {t("groupMembersSelected", {
                count: selectedIdentityIDs.size,
              })}
            </span>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={saving}
                onClick={() => changeOpen(false)}
              >
                {t("groupCreateCancel")}
              </Button>
              <Button
                type="button"
                disabled={selectedIdentityIDs.size === 0 || saving}
                onClick={() => void addSelectedMembers()}
              >
                {saving ? t("groupAddingMembers") : t("groupAddMembers")}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
