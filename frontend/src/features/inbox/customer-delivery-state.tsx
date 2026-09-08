/** 展示客户消息的外部投递状态并处理人工确认。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { MoreHorizontalIcon } from "lucide-react"
import { toast } from "sonner"
import {
  CustomerDeliveryResolution,
  CustomerDeliveryStatus,
  isApiError,
  resolveCustomerMessageDelivery,
  type CustomerMessageDelivery,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
import { useResourceInvalidator } from "@/hooks/use-resource"
import { MessageSendState } from "./message-send-state"

/** 在气泡内的时间右侧展示投递状态和处理入口。 */
export function CustomerDeliveryState({
  conversationID, delivery, loadingError, onRefresh, localFailed = false,
  onRetryLocal, retryLocalDisabled = false,
}: {
  conversationID: string
  delivery?: CustomerMessageDelivery
  loadingError: boolean
  onRefresh: () => void
  localFailed?: boolean
  onRetryLocal?: () => void
  retryLocalDisabled?: boolean
}) {
  const { t } = useTranslation("inbox")
  const invalidate = useResourceInvalidator()
  const [busy, setBusy] = useState(false)
  const [confirmRetry, setConfirmRetry] = useState(false)
  const review = delivery?.status === CustomerDeliveryStatus.CustomerDeliveryNeedsReview
  const retryable = delivery?.canRetry

  /** 提交人工处理意图并重新读取持久状态。 */
  async function resolve(
    resolution: CustomerDeliveryResolution,
    confirmDuplicateRisk = false,
  ) {
    if (!delivery) return
    setBusy(true)
    try {
      await resolveCustomerMessageDelivery(conversationID, delivery.id, {
        resolution, confirmDuplicateRisk,
      })
      await invalidate(resourceKeys.customerDeliveries(conversationID))
      setConfirmRetry(false)
    } catch (error) {
      toast.error(isApiError(error) ? error.message : t("deliveryResolveError"))
    } finally {
      setBusy(false)
    }
  }

  // 本地提交、排队和自动重试统一为发送中，异常原因只在提示中展示。
  const uncertain = review || delivery?.status === CustomerDeliveryStatus.CustomerDeliveryUncertain
  const failed = localFailed || delivery?.status === CustomerDeliveryStatus.CustomerDeliveryFailed
  // 已确认发送成功的消息不因后续轮询失败退回异常状态。
  const showLoadError = loadingError && delivery?.status !== CustomerDeliveryStatus.CustomerDeliverySent
  const state = failed || uncertain || delivery?.paused || showLoadError
    ? "attention"
    : delivery?.status === CustomerDeliveryStatus.CustomerDeliverySent ? "sent" : "sending"
  // 发送成功后只保留消息时间，发送中和异常状态继续提示。
  if (state === "sent") return null
  let detail: string | undefined
  if (localFailed) detail = t("messageSendError")
  else if (showLoadError) detail = t("deliveryLoadError")
  else if (uncertain) detail = t("deliveryUnconfirmed")
  else if (delivery?.paused) detail = t("deliveryError_channel_disabled")
  else if (failed) detail = delivery?.lastError
    ? t("deliveryError", { context: delivery.lastError })
    : t("messageSendError")

  return (
    <div className="inline-flex items-center gap-1.5 text-[11px]">
      <MessageSendState state={state} detail={detail} />
      {localFailed && onRetryLocal ? (
        <button type="button" disabled={retryLocalDisabled} onClick={onRetryLocal}>{t("messageRetry")}</button>
      ) : null}
      {showLoadError ? (
        <button type="button" onClick={onRefresh}>{t("deliveryRefresh")}</button>
      ) : null}
      {retryable ? (
        <button type="button" disabled={busy} onClick={() => {
          // 未知结果重试前说明可能重复，包括人工标记失败后的再次重试。
          if (review || delivery?.lastError === "unknown_result") setConfirmRetry(true)
          else void resolve(CustomerDeliveryResolution.CustomerDeliveryRetry)
        }}>{t("messageRetry")}</button>
      ) : null}
      {review ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-5" disabled={busy} aria-label={t("deliveryActions")}>
              <MoreHorizontalIcon className="size-3" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => void resolve(CustomerDeliveryResolution.CustomerDeliveryConfirmSent)}>
              {t("deliveryConfirmSent")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void resolve(CustomerDeliveryResolution.CustomerDeliveryConfirmFailed)}>
              {t("deliveryConfirmFailed")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
      <AlertDialog open={confirmRetry} onOpenChange={(open) => { if (!busy) setConfirmRetry(open) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deliveryRetryTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deliveryRetryRisk")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>{t("deliveryCancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={busy} onClick={(event) => {
              event.preventDefault()
              void resolve(CustomerDeliveryResolution.CustomerDeliveryRetry, true)
            }}>
              {t("messageRetry")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
