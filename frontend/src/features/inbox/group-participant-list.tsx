/** 群聊资料栏中的成员列表和成员管理交互。 */
import { useId, useMemo, useState } from "react"
import {
  CrownIcon,
  MoreHorizontalIcon,
  SearchIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  GroupParticipantRole,
  isApiError,
  type GroupParticipant,
  type MemberOption,
} from "@/api"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button, buttonVariants } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { GroupMemberPickerDialog } from "@/features/inbox/group-member-picker-dialog"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 展示群聊成员头像。 */
function GroupParticipantAvatar({
  participant,
}: {
  participant: GroupParticipant
}) {
  return (
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
  )
}

/** 展示当前成员并提供群主和本人可用的成员操作。 */
export function GroupParticipantList({
  participants,
  currentIdentityID,
  canManage,
  readOnly,
  onAdd,
  onTransferOwner,
  onRemove,
  onLeave,
}: {
  participants: GroupParticipant[]
  currentIdentityID: string
  canManage: boolean
  readOnly: boolean
  onAdd: (members: MemberOption[]) => Promise<void>
  onTransferOwner: (identityID: string) => Promise<void>
  onRemove: (identityID: string) => Promise<void>
  onLeave: (successorIdentityID?: string) => Promise<void>
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const memberSearchID = useId()
  const successorRadioName = useId()
  const [query, setQuery] = useState("")
  const [addOpen, setAddOpen] = useState(false)
  const [transferring, setTransferring] =
    useState<GroupParticipant | null>(null)
  const [removing, setRemoving] = useState<GroupParticipant | null>(null)
  const [leaving, setLeaving] = useState<GroupParticipant | null>(null)
  const [successorIdentityID, setSuccessorIdentityID] = useState("")
  const [acting, setActing] = useState(false)
  const normalizedQuery = query.trim().toLocaleLowerCase()
  const visibleParticipants = useMemo(
    () =>
      participants.filter(
        (participant) =>
          !normalizedQuery ||
          participant.displayName
            .toLocaleLowerCase()
            .includes(normalizedQuery),
      ),
    [normalizedQuery, participants],
  )
  const successorCandidates = participants.filter(
    (participant) => participant.identityId !== currentIdentityID,
  )
  const leavingAsOwner =
    leaving?.role === GroupParticipantRole.GroupParticipantRoleOwner
  const dissolvingGroup = leavingAsOwner && successorCandidates.length === 0

  /** 转让群主并关闭确认框。 */
  async function transferOwner() {
    if (!transferring) return
    setActing(true)
    try {
      await onTransferOwner(transferring.identityId)
      setTransferring(null)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("转让群主失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["ownerIdentityId"])
          : t("groupTransferOwnerError"),
      )
    } finally {
      setActing(false)
    }
  }

  /** 移除成员并关闭确认框。 */
  async function removeMember() {
    if (!removing) return
    setActing(true)
    try {
      await onRemove(removing.identityId)
      setRemoving(null)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("移出群聊成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["memberIdentityId"])
          : t("groupRemoveMemberError"),
      )
    } finally {
      setActing(false)
    }
  }

  /** 退出或解散群聊并关闭对应对话框。 */
  async function leaveGroup() {
    if (!leaving) return
    setActing(true)
    try {
      await onLeave(
        leavingAsOwner ? successorIdentityID || undefined : undefined,
      )
      setLeaving(null)
      setSuccessorIdentityID("")
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn(dissolvingGroup ? "解散群聊失败" : "退出群聊失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["successorIdentityId"])
          : t(dissolvingGroup ? "groupDissolveError" : "groupLeaveError"),
      )
    } finally {
      setActing(false)
    }
  }

  /** 关闭退出交互并清空继任者选择。 */
  function closeLeaveDialog() {
    setLeaving(null)
    setSuccessorIdentityID("")
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 items-end gap-2 border-b px-3 py-3">
        <div className="min-w-0 flex-1">
          <label
            htmlFor={memberSearchID}
            className="sr-only"
          >
            {t("groupMemberSearch")}
          </label>
          <div className="relative">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id={memberSearchID}
              value={query}
              autoComplete="off"
              className="pl-9"
              aria-label={t("groupMemberSearch")}
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
        </div>
        {canManage ? (
          <Button type="button" size="sm" onClick={() => setAddOpen(true)}>
            {t("groupAddMembers")}
          </Button>
        ) : null}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-1.5">
        {visibleParticipants.length === 0 ? (
          <p className="px-6 py-12 text-center text-sm text-muted-foreground">
            {t("groupMembersNoMatches")}
          </p>
        ) : (
          <div className="grid">
            {visibleParticipants.map((participant) => {
              const isCurrent =
                participant.identityId === currentIdentityID
              const isOwner =
                participant.role ===
                GroupParticipantRole.GroupParticipantRoleOwner
              const showActions =
                !readOnly && (isCurrent || (canManage && !isOwner))
              return (
                <div
                  key={participant.identityId}
                  className="flex items-center gap-3 rounded-md px-2 py-2"
                >
                  <GroupParticipantAvatar participant={participant} />
                  <span className="min-w-0 flex-1 truncate text-sm">
                    {participant.displayName}
                    {isCurrent ? (
                      <span className="ml-1 text-xs text-muted-foreground">
                        {t("groupMemberYou")}
                      </span>
                    ) : null}
                  </span>
                  {isOwner ? (
                    <span className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
                      <CrownIcon className="size-3.5" />
                      {t("groupOwner")}
                    </span>
                  ) : null}
                  <span className="flex size-8 shrink-0 items-center justify-center">
                    {showActions ? (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t("groupMemberMore", {
                              name: participant.displayName,
                            })}
                            title={t("groupMemberMore", {
                              name: participant.displayName,
                            })}
                          >
                            <MoreHorizontalIcon />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          {isCurrent ? (
                            <DropdownMenuItem
                              destructive
                              onSelect={() => setLeaving(participant)}
                            >
                              {t(
                                isOwner && participants.length === 1
                                  ? "groupDissolve"
                                  : "groupLeave",
                              )}
                            </DropdownMenuItem>
                          ) : (
                            <>
                              <DropdownMenuItem
                                onSelect={() => setTransferring(participant)}
                              >
                                {t("groupTransferOwner")}
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                destructive
                                onSelect={() => setRemoving(participant)}
                              >
                                {t("groupRemoveMember")}
                              </DropdownMenuItem>
                            </>
                          )}
                        </DropdownMenuContent>
                      </DropdownMenu>
                    ) : null}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <GroupMemberPickerDialog
        open={addOpen}
        participants={participants}
        onOpenChange={setAddOpen}
        onAdd={onAdd}
      />

      <AlertDialog
        open={transferring !== null}
        onOpenChange={(open) => !open && setTransferring(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("groupTransferOwnerTitle", {
                name: transferring?.displayName ?? "",
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("groupTransferOwnerDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={acting}>
              {t("groupCreateCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              className={buttonVariants({ variant: "default" })}
              disabled={acting}
              onClick={() => void transferOwner()}
            >
              {t("groupTransferOwnerConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("groupRemoveMemberTitle", {
                name: removing?.displayName ?? "",
              })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("groupRemoveMemberDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={acting}>
              {t("groupCreateCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={acting}
              onClick={() => void removeMember()}
            >
              {t("groupRemoveMemberConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={leaving !== null && !leavingAsOwner}
        onOpenChange={(open) => !open && closeLeaveDialog()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("groupLeaveTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("groupLeaveDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={acting}>
              {t("groupCreateCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={acting}
              onClick={() => void leaveGroup()}
            >
              {t("groupLeaveConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog
        open={leaving !== null && leavingAsOwner}
        onOpenChange={(open) => !open && closeLeaveDialog()}
      >
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {t(
                dissolvingGroup
                  ? "groupDissolveTitle"
                  : "groupOwnerLeaveTitle",
              )}
            </DialogTitle>
            <DialogDescription>
              {t(
                dissolvingGroup
                  ? "groupDissolveDescription"
                  : "groupOwnerLeaveDescription",
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="grid min-h-0 gap-4">
            {!dissolvingGroup ? (
              <div className="grid gap-2">
                <span className="text-sm font-medium">
                  {t("groupOwnerSuccessor")}
                </span>
                <ScrollArea className="max-h-64 rounded-md border">
                  <div
                    className="divide-y"
                    role="radiogroup"
                    aria-label={t("groupOwnerSuccessor")}
                  >
                    {successorCandidates.map((participant) => (
                      <label
                        key={participant.identityId}
                        className="flex items-center gap-3 px-3 py-2.5 transition-colors hover:bg-muted/50"
                      >
                        <input
                          type="radio"
                          name={successorRadioName}
                          value={participant.identityId}
                          checked={
                            successorIdentityID === participant.identityId
                          }
                          disabled={acting}
                          className="size-4 accent-primary"
                          onChange={() =>
                            setSuccessorIdentityID(participant.identityId)
                          }
                        />
                        <GroupParticipantAvatar participant={participant} />
                        <span className="min-w-0 flex-1 truncate text-sm">
                          {participant.displayName}
                        </span>
                      </label>
                    ))}
                  </div>
                </ScrollArea>
              </div>
            ) : null}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={acting}
                onClick={closeLeaveDialog}
              >
                {t("groupCreateCancel")}
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={(!dissolvingGroup && !successorIdentityID) || acting}
                onClick={() => void leaveGroup()}
              >
                {t(
                  dissolvingGroup
                    ? "groupDissolveConfirm"
                    : "groupOwnerLeaveConfirm",
                )}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
