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
  claimServiceSession,
  closeServiceSession,
  isApiError,
  listCustomerServiceAssignees,
  reopenServiceSession,
  transferServiceSession,
  type CustomerInboxConversationData,
  type CustomerServiceSession,
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
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"

type CustomerSummary = CustomerInboxConversationData["customer"]

/** 判断客户会话所在渠道是否支持客服回复。 */
export function customerReplySupported(customer: CustomerSummary) {
  return (
    customer.channelType === ChannelType.ChannelTypeWebsite ||
    customer.channelType === ChannelType.ChannelTypeTelegram
  )
}

/** 按处理状态和负责人返回当前客服不能回复的原因。 */
export function customerReplyDisabledReason(
  customer: CustomerSummary,
  currentIdentityId: string,
  t: TFunction<"inbox">,
) {
  if (
    customer.serviceSessionStatus ===
    ServiceSessionStatus.ServiceSessionStatusClosed
  )
    return t("replyClosedUnavailable")
  if (customer.assignee && customer.assignee.identityId !== currentIdentityId)
    return t("replyAssignedUnavailable", { name: customer.assignee.displayName })
  return null
}

/** 管理客服处理周期命令的执行状态、可用操作和关闭确认。 */
export function useCustomerSessionActions(
  conversation: CustomerInboxConversationData | null,
  currentIdentityId: string,
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
    // 未分配或由本人负责的开放会话可以关闭。
    closable: sessionOpen && (!customer?.assignee || assignedToCurrentUser),
    transferCandidates,
    closeConfirmationOpen,
    setCloseConfirmationOpen,
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
    transfer: (assignee: (typeof transferCandidates)[number]) =>
      run(
        `transfer:${assignee.identityId}`,
        (conversationID) =>
          transferServiceSession(conversationID, {
            assigneeIdentityId: assignee.identityId,
          }),
        t("conversationTransferSuccess", { name: assignee.displayName }),
      ),
    close: () =>
      run("close", closeServiceSession, t("conversationCloseSuccess")),
  }
}

export type CustomerSessionActions = ReturnType<typeof useCustomerSessionActions>

/** 关闭客户会话处理周期前的确认弹窗。 */
export function CustomerSessionCloseDialog({
  actions,
}: {
  actions: CustomerSessionActions
}) {
  const { t } = useTranslation(["inbox", "common"])
  // 移动端弹窗按钮使用触屏尺寸。
  const buttonClassName = cn(resolveAppPlatform() === "mobile" && "min-h-11")
  return (
    <AlertDialog
      open={actions.closeConfirmationOpen}
      onOpenChange={actions.setCloseConfirmationOpen}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("conversationCloseConfirmTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("conversationCloseConfirmDescription")}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel className={buttonClassName}>
            {t("common:actions.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            className={buttonClassName}
            onClick={() => void actions.close()}
          >
            {t("conversationCloseConfirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
