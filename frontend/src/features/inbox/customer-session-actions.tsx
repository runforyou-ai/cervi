/** 客户会话处理周期的回复条件、领取、转交、关闭与重新打开操作。 */
import { useState } from "react"
import type { TFunction } from "i18next"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ChannelType,
  OrganizationIdentityType,
  ServiceSessionStatus,
  ServiceSessionTargetKind,
  claimServiceSession,
  closeServiceSession,
  isApiError,
  listCustomerServiceAssignees,
  listServiceQueueTeams,
  reopenServiceSession,
  transferServiceSession,
  type CustomerInboxConversationData,
  type CustomerServiceSession,
  type InboxAssignee,
  type ServiceQueueTeam,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import {
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type CustomerSummary = CustomerInboxConversationData["customer"]

/** 判断客户会话所在渠道是否支持客服回复。 */
export function customerReplySupported(customer: CustomerSummary) {
  return (
    customer.channelType === ChannelType.ChannelTypeWebsite ||
    customer.channelType === ChannelType.ChannelTypeTelegram
  )
}

/** 按接待资格、处理状态和负责人返回当前成员不能对客回复的原因。 */
export function customerReplyDisabledReason(
  customer: CustomerSummary,
  currentIdentityId: string,
  handlesCustomers: boolean,
  t: TFunction<"inbox">,
) {
  if (!handlesCustomers) return t("replyHandlingUnavailable")
  if (
    customer.serviceSessionStatus ===
    ServiceSessionStatus.ServiceSessionStatusClosed
  )
    return t("replyClosedUnavailable")
  if (customer.assignee && customer.assignee.identityId !== currentIdentityId)
    return t("replyAssignedUnavailable", { name: customer.assignee.displayName })
  return null
}

/** 管理客服处理周期命令的执行状态、可用操作和关闭确认；只有开启接待的成员可以领取、转交、关闭与重开。 */
export function useCustomerSessionActions(
  conversation: CustomerInboxConversationData | null,
  currentIdentityId: string,
  handlesCustomers: boolean,
  onChanged: () => void,
) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const [operation, setOperation] = useState("")
  const [closeConfirmationOpen, setCloseConfirmationOpen] = useState(false)
  const customer = conversation?.customer ?? null
  const sessionOpen =
    customer?.serviceSessionStatus === ServiceSessionStatus.ServiceSessionStatusOpen
  const sessionClosed =
    customer?.serviceSessionStatus === ServiceSessionStatus.ServiceSessionStatusClosed
  const assignedToCurrentUser = customer?.assignee?.identityId === currentIdentityId
  const { data: assignees = [] } = useResource(
    resourceKeys.customerServiceAssignees(),
    () => listCustomerServiceAssignees(),
    { enabled: Boolean(customer && sessionOpen && assignedToCurrentUser) },
  )
  const { data: transferTeams = [] } = useResource(
    resourceKeys.serviceQueueTeams(),
    () => listServiceQueueTeams(),
    { enabled: Boolean(customer && sessionOpen && assignedToCurrentUser) },
  )
  // 网站和 Telegram 会话可转给 AI 员工，其他渠道只转给真人客服。
  const transferCandidates = assignees.filter(
    (assignee) =>
      assignee.identityId !== currentIdentityId &&
      ((customer && customerReplySupported(customer)) ||
        assignee.type !== OrganizationIdentityType.OrganizationIdentityTypeAgent),
  )

  /** 执行客服处理周期命令并通知上层刷新受影响视图。 */
  async function run(
    nextOperation: string,
    execute: (conversationID: string) => Promise<CustomerServiceSession>,
    successMessage: string,
  ) {
    if (!conversation) return
    setOperation(nextOperation)
    try {
      await execute(conversation.id)
      onChanged()
      toast.success(successMessage)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("更新客服会话失败", {
        conversationId: conversation.id,
        operation: nextOperation,
        error,
      })
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("conversationActionError"),
      )
    } finally {
      setOperation("")
    }
  }

  return {
    operation,
    sessionOpen,
    sessionClosed,
    assignedToCurrentUser,
    reopenable: sessionClosed && handlesCustomers,
    claimable: sessionOpen && !assignedToCurrentUser && handlesCustomers,
    transferable: sessionOpen && assignedToCurrentUser && handlesCustomers,
    // 未分配或由本人负责的开放会话可以关闭。
    closable:
      sessionOpen && (!customer?.assignee || assignedToCurrentUser) && handlesCustomers,
    transferCandidates,
    transferTeams,
    closeConfirmationOpen,
    setCloseConfirmationOpen,
    unansweredMentionCount: customer?.unansweredMentionCount ?? 0,
    reopen: () =>
      run("reopen", reopenServiceSession, t("conversationReopenSuccess")),
    claim: () =>
      run(
        "claim",
        claimServiceSession,
        customer?.assignee
          ? t("conversationTakeoverSuccess")
          : t("conversationClaimSuccess"),
      ),
    transferToMember: (assignee: InboxAssignee) =>
      run(
        `transfer:${assignee.identityId}`,
        (conversationID) =>
          transferServiceSession(conversationID, {
            kind: ServiceSessionTargetKind.ServiceSessionTargetMember,
            identityId: assignee.identityId,
          }),
        t("conversationTransferSuccess", { name: assignee.displayName }),
      ),
    transferToTeam: (team: ServiceQueueTeam) =>
      run(
        `transfer:${team.id}`,
        (conversationID) =>
          transferServiceSession(conversationID, {
            kind: ServiceSessionTargetKind.ServiceSessionTargetTeam,
            teamId: team.id,
          }),
        t("conversationTransferSuccess", { name: team.name }),
      ),
    transferToPublicQueue: () =>
      run(
        "transfer:public-queue",
        (conversationID) =>
          transferServiceSession(conversationID, {
            kind: ServiceSessionTargetKind.ServiceSessionTargetPublicQueue,
          }),
        t("conversationTransferQueueSuccess"),
      ),
    close: () =>
      run("close", closeServiceSession, t("conversationCloseSuccess")),
  }
}

export type CustomerSessionActions = ReturnType<typeof useCustomerSessionActions>

/** 转交去向菜单项：同事、团队队列和公共队列分组展示。 */
export function CustomerTransferMenuItems({
  actions,
  itemClassName,
}: {
  actions: CustomerSessionActions
  itemClassName?: string
}) {
  const { t } = useTranslation("inbox")
  return (
    <>
      {actions.transferCandidates.length > 0 ? (
        <>
          <DropdownMenuLabel>{t("conversationTransferCoworkers")}</DropdownMenuLabel>
          {actions.transferCandidates.map((assignee) => (
            <DropdownMenuItem
              key={assignee.identityId}
              className={itemClassName}
              onSelect={() => void actions.transferToMember(assignee)}
            >
              {assignee.displayName}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
        </>
      ) : null}
      {actions.transferTeams.length > 0 ? (
        <>
          <DropdownMenuLabel>{t("conversationTransferTeams")}</DropdownMenuLabel>
          {actions.transferTeams.map((team) => (
            <DropdownMenuItem
              key={team.id}
              className={itemClassName}
              disabled={!team.available}
              onSelect={() => void actions.transferToTeam(team)}
            >
              <span className="min-w-0 flex-1 truncate">{team.name}</span>
              {team.available ? null : (
                <span className="text-xs text-muted-foreground">
                  {t("conversationTransferTeamUnavailable")}
                </span>
              )}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
        </>
      ) : null}
      <DropdownMenuItem
        className={itemClassName}
        onSelect={() => void actions.transferToPublicQueue()}
      >
        {t("conversationTransferPublicQueue")}
      </DropdownMenuItem>
    </>
  )
}

/** 关闭客户会话处理周期前的确认弹窗。 */
export function CustomerSessionCloseDialog({
  actions,
}: {
  actions: CustomerSessionActions
}) {
  const { t } = useTranslation("inbox")
  return (
    <ConfirmationDialog
      open={actions.closeConfirmationOpen}
      pending={false}
      title={t("conversationCloseConfirmTitle")}
      description={
        <>
          {t("conversationCloseConfirmDescription")}
          {actions.unansweredMentionCount > 0 ? (
            <span className="mt-1 block text-foreground">
              {t("conversationCloseUnansweredMentions", { count: actions.unansweredMentionCount })}
            </span>
          ) : null}
        </>
      }
      onOpenChange={actions.setCloseConfirmationOpen}
      onConfirm={() => {
        actions.setCloseConfirmationOpen(false)
        void actions.close()
      }}
    />
  )
}
