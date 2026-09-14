/** 移动端群主逐个移除成员和转让群主的独立页面。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useOutletContext } from "react-router"

import {
  ConversationStatus,
  GroupParticipantRole,
  OrganizationIdentityType,
  removeGroupConversationMember,
  transferGroupConversationOwner,
  type GroupParticipant,
} from "@/api"
import type { MobileGroupDetailsContext } from "@/apps/mobile/mobile-group-context"
import { MobileGroupMemberList } from "@/apps/mobile/mobile-group-members"
import { useMobileBack } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"

/** 列出可操作成员并二次确认，移除后留在列表，转让后返回群详情。 */
export function MobileGroupMemberActionPage({
  action,
}: {
  action: "remove" | "transfer"
}) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const { group, canManage, busy, onSave } =
    useOutletContext<MobileGroupDetailsContext>()
  const close = useMobileBack(`/inbox/group/${group.id}/details`)
  const [target, setTarget] = useState<GroupParticipant | null>(null)
  // 确认框开关独立于目标成员，关闭时保留 target 供标题展示姓名。
  const [confirming, setConfirming] = useState(false)
  const trigger = useRef<HTMLElement | null>(null)
  const alive = useRef(false)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  const removing = action === "remove"
  const archived =
    group.status === ConversationStatus.ConversationStatusArchived
  const name = target?.displayName ?? ""
  const config = removing
    ? {
        title: t("group.removeMembers"),
        empty: t("group.removeMembersEmpty"),
        notice: t(archived ? "group.removeArchived" : "group.removeOwnerOnly"),
        button: t("common:actions.remove"),
        buttonLabel: (memberName: string) =>
          t("group.removeMember", { name: memberName }),
        confirmTitle: t("inbox:groupRemoveMemberTitle", { name }),
        confirmDescription: t("inbox:groupRemoveMemberDescription"),
        confirmButton: t("inbox:groupRemoveMemberConfirm"),
        confirmVariant: "destructive" as const,
        submit: (identityID: string) =>
          removeGroupConversationMember(group.id, {
            memberIdentityId: identityID,
          }),
      }
    : {
        title: t("inbox:groupTransferOwner"),
        empty: t("group.transferOwnerEmpty"),
        notice: t(
          archived ? "group.transferArchived" : "group.transferOwnerOnly",
        ),
        button: t("group.transfer"),
        buttonLabel: (memberName: string) =>
          t("group.transferOwnerTo", { name: memberName }),
        confirmTitle: t("inbox:groupTransferOwnerTitle", { name }),
        confirmDescription: t("inbox:groupTransferOwnerDescription"),
        confirmButton: t("inbox:groupTransferOwnerConfirm"),
        confirmVariant: "default" as const,
        submit: (identityID: string) =>
          transferGroupConversationOwner(group.id, {
            ownerIdentityId: identityID,
          }),
      }
  // 移除候选为群主以外的成员，转让候选为群主以外的真人成员。
  const candidates = group.participants.filter(
    (member) =>
      member.role !== GroupParticipantRole.GroupParticipantRoleOwner &&
      (removing ||
        member.identityType ===
          OrganizationIdentityType.OrganizationIdentityTypeUser),
  )

  /** 提交确认的成员操作，离开页面后忽略迟到结果。 */
  async function confirm(member: GroupParticipant) {
    const success = await onSave(() => config.submit(member.identityId))
    if (!success || !alive.current) return
    setConfirming(false)
    if (!removing) close()
  }

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        title={config.title}
        backTo={`/inbox/group/${group.id}/details`}
        backDisabled={busy}
      />
      {canManage ? null : (
        <p
          className="border-b px-4 py-3 text-sm text-muted-foreground"
          role="status"
        >
          {config.notice}
        </p>
      )}
      <MobileGroupMemberList
        members={candidates}
        storageKey={`group-${action}:${group.id}`}
        emptyText={config.empty}
        trailing={(member) => (
          <Button
            type="button"
            variant="outline"
            className="min-h-11 shrink-0"
            disabled={busy || !canManage}
            aria-label={config.buttonLabel(member.displayName)}
            onClick={(event) => {
              trigger.current = event.currentTarget
              setTarget(member)
              setConfirming(true)
            }}
          >
            {config.button}
          </Button>
        )}
      />
      <AlertDialog
        open={confirming && canManage}
        onOpenChange={(open) => {
          if (!open && !busy) setConfirming(false)
        }}
      >
        <AlertDialogContent
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            trigger.current?.focus({ preventScroll: true })
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{config.confirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {config.confirmDescription}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="min-h-11" disabled={busy}>
              {t("common:actions.cancel")}
            </AlertDialogCancel>
            <Button
              className="min-h-11"
              variant={config.confirmVariant}
              disabled={busy}
              onClick={() => target && void confirm(target)}
            >
              {config.confirmButton}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}
