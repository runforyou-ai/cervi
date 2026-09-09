/** 移动端群成员预览、群资料和个人设置的连续详情页。 */
import { groupMemberMaxCount } from "@/features/inbox/group-conversation-schema"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Outlet, useMatch, useNavigate } from "react-router"
import { toast } from "sonner"
import {
  ConversationStatus,
  GroupParticipantRole,
  isApiError,
  isNotFoundApiError,
  updateConversationNotificationSettings,
} from "@/api"
import { useMobileGroup } from "@/apps/mobile/mobile-group-context"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { MobilePageHeader, MobileScrollArea } from "@/apps/mobile/mobile-page"
import { MobileGroupInfo } from "@/apps/mobile/mobile-group-info"
import { MobileGroupMembersPreview } from "@/apps/mobile/mobile-group-members"
import type { MobileGroupDetailsContext } from "@/apps/mobile/mobile-group-context"
import { GroupDissolveDialog } from "@/features/inbox/group-dissolve-dialog"
import { MobileGroupLeaveDialog } from "@/apps/mobile/mobile-group-leave-dialog"
import { Button } from "@/components/ui/button"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 协调群管理操作，成功后刷新服务端事实并保留详情浏览位置。 */
export function MobileGroupDetailsPage() {
  const { t } = useTranslation("mobile")
  const {
    group,
    returnDepth,
    error,
    refreshing,
    refresh,
    onUnavailable,
    onLeft,
    onLeavingChange,
  } = useMobileGroup()
  const { identity } = useMobileWorkspace()
  const navigate = useNavigate()
  const childOpen = !useMatch("/inbox/group/:conversationID/details")
  const invalidate = useResourceInvalidator()
  const save = useImmediateSave()
  const muteSave = useImmediateSave()
  const [pendingMuted, setPendingMuted] = useState<boolean | null>(null)
  const [leaveOpen, setLeaveOpen] = useState(false)
  const [dissolveOpen, setDissolveOpen] = useState(false)
  const trigger = useRef<HTMLElement | null>(null)
  const archived =
    group.status === ConversationStatus.ConversationStatusArchived
  const isOwner = group.participants.some(
    (member) =>
      member.identityId === identity.user.identityId &&
      member.role === GroupParticipantRole.GroupParticipantRoleOwner,
  )
  const canManage = isOwner && !archived
  useEffect(() => {
    // 群聊解散或群主身份变化后关闭退出、解散确认。
    if ((archived || isOwner) && !save.saving) setLeaveOpen(false)
    if (archived || !isOwner) setDissolveOpen(false)
  }, [archived, isOwner, save.saving])

  /** 保存资料或退出群聊，卸载后忽略提示与导航结果。 */
  async function perform(
    action: () => Promise<unknown>,
    change: "messages" | "leave" = "messages",
  ) {
    const request = save.begin()
    if (request === null) return false
    const leaving = change === "leave"
    let completed = false
    if (leaving) onLeavingChange(true)
    try {
      await action()
      completed = true
      await Promise.all([
        invalidate(resourceKeys.groupConversation(group.id), {
          refetchType: leaving ? "none" : "active",
        }),
        invalidate(resourceKeys.inbox(), {
          refetchType: leaving ? "all" : "active",
        }),
        ...(change === "messages"
          ? [invalidate(resourceKeys.conversationMessages(group.id))]
          : []),
      ])
      if (leaving && save.isCurrent(request)) onLeft()
      return save.isCurrent(request)
    } catch (error) {
      if (!save.isCurrent(request) || recoverSession(error, navigate))
        return false
      if (isNotFoundApiError(error)) onUnavailable()
      else {
        console.warn("移动端群管理操作失败", {
          conversationID: group.id,
          error,
        })
        toast.error(isApiError(error) ? apiErrorMessage(error) : t("group.saveError"))
        void refresh()
      }
      return false
    } finally {
      if (leaving && !completed) onLeavingChange(false)
      save.finish(request)
    }
  }

  /** 保存免打扰开关，失败时恢复原值。 */
  async function changeMuted(muted: boolean) {
    const request = muteSave.begin()
    if (request === null) return
    setPendingMuted(muted)
    try {
      await updateConversationNotificationSettings(group.id, { muted })
      await Promise.all([
        invalidate(resourceKeys.groupConversation(group.id)),
        invalidate(resourceKeys.inbox()),
      ])
    } catch (error) {
      if (!muteSave.isCurrent(request) || recoverSession(error, navigate))
        return
      if (isNotFoundApiError(error)) onUnavailable()
      else {
        console.warn("移动端保存群免打扰失败", {
          conversationID: group.id,
          error,
        })
        toast.error(isApiError(error) ? apiErrorMessage(error) : t("group.saveError"))
      }
    } finally {
      if (muteSave.isCurrent(request)) setPendingMuted(null)
      muteSave.finish(request)
    }
  }

  return (
    <div className="relative h-full min-h-0">
      <section
        className={`flex h-full min-h-0 flex-col bg-background ${childOpen ? "absolute inset-0 opacity-0 pointer-events-none" : ""}`}
        inert={childOpen}
      >
        <MobilePageHeader
          title={t("group.details")}
          backTo={childOpen ? undefined : `/inbox/group/${group.id}`}
          actions={
            error ? (
              <Button
                variant="ghost"
                className="min-h-11 text-warning"
                disabled={refreshing}
                onClick={() => void refresh()}
              >
                {t("inbox.refreshFailed")}
              </Button>
            ) : null
          }
        />
        <MobileScrollArea storageKey={`group-details:${group.id}`}>
          <MobileGroupMembersPreview
            group={group}
            isOwner={isOwner}
            returnDepth={returnDepth}
            canAdd={
              canManage && !save.saving && group.participants.length < groupMemberMaxCount
            }
          />
          <MobileGroupInfo
            group={group}
            isOwner={isOwner}
            archived={archived}
            busy={save.saving}
            muted={pendingMuted ?? group.muted}
            muteBusy={muteSave.saving}
            onEdit={(field) => {
              if (!canManage) {
                toast.message(t(archived ? "group.editArchived" : "group.editOwnerOnly"))
                return
              }
              void navigate(`edit/${field}`, {
                replace: returnDepth === 0,
                state: {
                  mobileBack: returnDepth > 0,
                  groupReturnDepth: returnDepth > 0 ? returnDepth + 1 : 0,
                },
              })
            }}
            onLeave={(source) => {
              trigger.current = source
              if (isOwner) setDissolveOpen(true)
              else setLeaveOpen(true)
            }}
            onMute={(muted) => void changeMuted(muted)}
          />
        </MobileScrollArea>
        <GroupDissolveDialog
          group={group}
          open={dissolveOpen}
          onOpenChange={setDissolveOpen}
          trigger={trigger.current}
        />
        {leaveOpen && !archived && !isOwner ? (
          <MobileGroupLeaveDialog
            group={group}
            busy={save.saving || muteSave.saving}
            trigger={trigger.current}
            onClose={() => setLeaveOpen(false)}
            onSave={perform}
          />
        ) : null}
      </section>
      <Outlet
        context={
          {
            group,
            returnDepth,
            canManage,
            busy: save.saving,
            onSave: perform,
          } satisfies MobileGroupDetailsContext
        }
      />
    </div>
  )
}
