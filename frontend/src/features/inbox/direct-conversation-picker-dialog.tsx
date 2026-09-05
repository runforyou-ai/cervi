/** 企业成员内部单聊对象选择器。 */
import { useEffect, useState } from "react"
import {
  BotIcon,
  LoaderCircleIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  findDirectConversation,
  isApiError,
  sessionPath,
  OrganizationIdentityType,
  type DirectInboxConversationData,
  type MemberOption,
} from "@/api"
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
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"

/** 选择一位活跃企业成员。 */
export function DirectConversationPickerDialog({
  open,
  currentIdentityID,
  onOpenChange,
  onSelected,
}: {
  open: boolean
  currentIdentityID: string
  onOpenChange: (open: boolean) => void
  onSelected: (
    member: MemberOption,
    conversation: DirectInboxConversationData | null,
  ) => void
}) {
  const { t } = useTranslation("inbox")
  const [selectedMember, setSelectedMember] = useState<MemberOption | null>(null)
  const { data, loading, error, refresh } = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { enabled: open, staleTime: 0 },
  )
  const lookup = useResource(
    resourceKeys.directConversation(selectedMember?.id ?? ""),
    () => findDirectConversation(selectedMember?.id ?? ""),
    { enabled: open && Boolean(selectedMember), staleTime: 0 },
  )
  const candidates = (data ?? []).filter(
    (member) => member.id !== currentIdentityID,
  )

  useEffect(() => {
    if (!open) setSelectedMember(null)
  }, [open])

  useEffect(() => {
    if (!open || !selectedMember || lookup.loading || lookup.refreshing) return
    if (lookup.error) {
      setSelectedMember(null)
      if (isApiError(lookup.error) && sessionPath(lookup.error.state)) return
      console.warn("查找企业成员内部单聊失败", lookup.error)
      toast.error(
        isApiError(lookup.error)
          ? apiErrorMessage(lookup.error, ["targetIdentityId"])
          : t("directLookupError"),
      )
      return
    }
    if (lookup.data === undefined) return
    onSelected(selectedMember, lookup.data)
    setSelectedMember(null)
    onOpenChange(false)
  }, [
    lookup.data, lookup.error, lookup.loading, lookup.refreshing,
    onOpenChange, onSelected, open, selectedMember, t,
  ])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[min(42rem,calc(100svh-2rem))] grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("directPickerTitle")}</DialogTitle>
          <DialogDescription>{t("directPickerDescription")}</DialogDescription>
        </DialogHeader>
        <ScrollArea className="min-h-64 rounded-md border">
          {loading ? (
            <LoadingIndicator className="min-h-64 justify-center">
              {t("directPickerLoading")}
            </LoadingIndicator>
          ) : error ? (
            <div className="flex min-h-64 flex-col items-center justify-center p-6 text-center">
              <p className="text-sm text-muted-foreground">
                {t("directPickerLoadError")}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-3"
                onClick={() => void refresh()}
              >
                {t("messagesRetry")}
              </Button>
            </div>
          ) : candidates.length === 0 ? (
            <p className="px-6 py-12 text-center text-sm text-muted-foreground">
              {t("directPickerEmpty")}
            </p>
          ) : (
            <div className="grid p-1.5">
              {candidates.map((member) => (
                <button
                  key={member.id}
                  type="button"
                  disabled={Boolean(selectedMember)}
                  className="flex items-center gap-3 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-muted disabled:opacity-60"
                  onClick={() => setSelectedMember(member)}
                >
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
                    {member.type ===
                    OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                      <BotIcon className="size-4" />
                    ) : member.displayName ? (
                      Array.from(member.displayName)[0]?.toLocaleUpperCase()
                    ) : (
                      <UserRoundIcon className="size-4" />
                    )}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">
                    {member.displayName}
                  </span>
                  {member.type ===
                  OrganizationIdentityType.OrganizationIdentityTypeAgent ? (
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {t("directPickerAgent")}
                    </span>
                  ) : null}
                  {selectedMember?.id === member.id ? (
                    <LoaderCircleIcon className="size-4 shrink-0 animate-spin text-muted-foreground" />
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
