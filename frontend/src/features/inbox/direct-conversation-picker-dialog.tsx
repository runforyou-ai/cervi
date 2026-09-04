/** 企业成员内部单聊对象选择器。 */
import { useMemo, useState } from "react"
import {
  BotIcon,
  SearchIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  OrganizationIdentityType,
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
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

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
  onSelected: (member: MemberOption) => void
}) {
  const { t } = useTranslation("inbox")
  const [query, setQuery] = useState("")
  const { data, loading, error, refresh } = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { enabled: open, staleTime: 0 },
  )
  const candidates = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    return (data ?? []).filter(
      (member) =>
        member.id !== currentIdentityID &&
        (!normalizedQuery ||
          member.displayName.toLocaleLowerCase().includes(normalizedQuery)),
    )
  }, [currentIdentityID, data, query])

  /** 打开所选成员的本地单聊草稿。 */
  function start(member: MemberOption) {
    onSelected(member)
    setQuery("")
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[min(42rem,calc(100svh-2rem))] grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("directPickerTitle")}</DialogTitle>
          <DialogDescription>{t("directPickerDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-1.5">
          <label htmlFor="direct-member-search" className="text-sm font-medium">
            {t("directPickerSearch")}
          </label>
          <div className="relative">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              id="direct-member-search"
              value={query}
              autoComplete="off"
              className="pl-9"
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
        </div>
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
                  className="flex items-center gap-3 rounded-md px-3 py-2.5 text-left transition-colors hover:bg-muted disabled:opacity-60"
                  onClick={() => start(member)}
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
                </button>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
