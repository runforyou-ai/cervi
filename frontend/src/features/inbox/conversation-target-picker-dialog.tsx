/** 单聊对象选择器。 */
import { useRef } from "react"
import { useTranslation } from "react-i18next"

import { OrganizationIdentityType, type MemberOption } from "@/api"
import { DirectConversationDraftAvatar } from "@/features/inbox/direct-conversation-draft-header"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { ScrollArea } from "@/components/ui/scroll-area"
import { listChatTargets, orderChatTargets } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 选择一位同事或活跃 AI 员工开始单聊，同事在前。 */
export function ConversationTargetPickerDialog({
  open,
  currentIdentityId,
  onOpenChange,
  onSelected,
}: {
  open: boolean
  currentIdentityId: string
  onOpenChange: (open: boolean) => void
  onSelected: (member: MemberOption) => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const dialogRef = useRef<HTMLDivElement>(null)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.chatTargets(),
    listChatTargets,
    { enabled: open, staleTime: 0 },
  )
  const candidates = orderChatTargets(data ?? [], currentIdentityId)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={dialogRef}
        className="grid max-h-[min(42rem,calc(100svh-2rem))] grid-rows-[auto_minmax(0,1fr)] overflow-hidden outline-none"
        onOpenAutoFocus={(event) => {
          // 选择器打开时聚焦弹窗容器并启用键盘导航。
          event.preventDefault()
          dialogRef.current?.focus()
        }}
      >
        <DialogHeader>
          <DialogTitle>{t("newDirectConversation")}</DialogTitle>
          <DialogDescription>
            {t("chatPickerDescription")}
          </DialogDescription>
        </DialogHeader>
        <ScrollArea className="min-h-64 rounded-md border">
          {loading ? (
            <LoadingIndicator className="min-h-64 justify-center">
              {t("chatPickerLoading")}
            </LoadingIndicator>
          ) : error ? (
            <div className="flex min-h-64 flex-col items-center justify-center p-6 text-center">
              <p className="text-sm text-muted-foreground">
                {t("chatPickerLoadError")}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-3"
                onClick={() => void refresh()}
              >
                {t("common:actions.retry")}
              </Button>
            </div>
          ) : candidates.length === 0 ? (
            <p className="px-6 py-12 text-center text-sm text-muted-foreground">
              {t("chatPickerEmpty")}
            </p>
          ) : (
            <div className="grid p-1.5">
              {candidates.map((member) => (
                <button
                  key={member.id}
                  type="button"
                  className="flex items-center gap-3 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-muted"
                  onClick={() => {
                    onSelected(member)
                    onOpenChange(false)
                  }}
                >
                  <DirectConversationDraftAvatar
                    member={member}
                    className="size-9"
                  />
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">
                    {member.displayName}
                  </span>
                  {member.type === OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                    <span className="shrink-0 text-xs text-muted-foreground">{t("chatPickerAgent")}</span>
                  ) : member.type === OrganizationIdentityType.OrganizationIdentityTypeAssistant ? (
                    <span className="shrink-0 text-xs text-muted-foreground">{t("chatPickerAssistant")}</span>
                  ) : null}
                </button>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
