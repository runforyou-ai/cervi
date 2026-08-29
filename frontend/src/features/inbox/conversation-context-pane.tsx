import {
  BotIcon,
  BriefcaseBusinessIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import type { InboxConversation } from "@/api"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { ConversationAvatar } from "@/features/inbox/conversation-header"
import { cn } from "@/lib/utils"

function ContextPlaceholder({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof BotIcon
  title: string
  description: string
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center">
      <div className="mb-3 flex size-10 items-center justify-center rounded-xl border bg-muted/30 text-muted-foreground">
        <Icon className="size-4" />
      </div>
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="mt-1.5 max-w-60 text-xs leading-5 text-muted-foreground">
        {description}
      </p>
    </div>
  )
}

/** 展示当前联系人摘要，并为资料、AI 和业务上下文预留独立页签。 */
function ConversationContextContent({
  conversation,
  contactName,
  sheet = false,
}: {
  conversation: InboxConversation
  contactName: string
  sheet?: boolean
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <div
        className={cn(
          "flex h-16 shrink-0 items-center gap-3 border-b px-4",
          sheet && "pr-12",
        )}
      >
        <ConversationAvatar conversation={conversation} className="size-9" />
        <div className="min-w-0">
          <p className="text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
            {t("contextCurrentContact")}
          </p>
          <h2
            className="mt-0.5 truncate text-sm font-semibold"
            title={contactName}
          >
            {contactName}
          </h2>
        </div>
      </div>

      <Tabs defaultValue="profile" className="min-h-0 flex-1">
        <TabsList
          aria-label={t("contextTabsLabel")}
          className="h-auto shrink-0 justify-start gap-1 px-3 py-2"
        >
          <TabsTrigger
            value="profile"
            className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
          >
            {t("contextProfileTab")}
          </TabsTrigger>
          <TabsTrigger
            value="assistant"
            className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
          >
            {t("contextAssistantTab")}
          </TabsTrigger>
          <TabsTrigger
            value="business"
            className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
          >
            {t("contextBusinessTab")}
          </TabsTrigger>
        </TabsList>

        <TabsContent
          value="profile"
          className="mt-0 min-h-0 flex-1 overflow-hidden data-[state=active]:flex data-[state=active]:flex-col"
        >
          <ScrollArea className="min-h-0 flex-1">
            <section className="p-4">
              <dl className="grid gap-3 text-sm">
                <div className="grid grid-cols-[5rem_minmax(0,1fr)] items-center gap-3">
                  <dt className="text-xs text-muted-foreground">
                    {t("contextContactName")}
                  </dt>
                  <dd className="min-w-0 truncate" title={contactName}>
                    {contactName}
                  </dd>
                </div>
              </dl>
              <p className="mt-4 rounded-lg border border-dashed p-3 text-xs leading-5 text-muted-foreground">
                {t("contextContactDetailsPlaceholder")}
              </p>
            </section>
          </ScrollArea>
        </TabsContent>

        <TabsContent
          value="assistant"
          className="mt-0 min-h-0 flex-1 overflow-hidden data-[state=active]:flex data-[state=active]:flex-col"
        >
          <ContextPlaceholder
            icon={BotIcon}
            title={t("contextAssistantTitle")}
            description={t("contextAssistantDescription")}
          />
        </TabsContent>

        <TabsContent
          value="business"
          className="mt-0 min-h-0 flex-1 overflow-hidden data-[state=active]:flex data-[state=active]:flex-col"
        >
          <ContextPlaceholder
            icon={BriefcaseBusinessIcon}
            title={t("contextBusinessTitle")}
            description={t("contextBusinessDescription")}
          />
        </TabsContent>
      </Tabs>
    </div>
  )
}

/** 在宽屏常驻栏和较窄视口 Sheet 中复用联系人上下文。 */
export function ConversationContextPane({
  conversation,
  contactName,
  desktopVisible,
  sheetOpen,
  onDesktopToggle,
  onSheetOpenChange,
}: {
  conversation: InboxConversation
  contactName: string
  desktopVisible: boolean
  sheetOpen: boolean
  onDesktopToggle: () => void
  onSheetOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <>
      <div className="relative hidden h-full w-4 shrink-0 border-l bg-background xl:block">
        <button
          type="button"
          className="absolute top-1/2 left-0 z-10 flex h-12 w-4 -translate-y-1/2 items-center justify-center border border-border bg-muted text-muted-foreground shadow-sm transition-colors hover:bg-muted/80 hover:text-foreground"
          aria-label={
            desktopVisible ? t("contextClose") : t("contextOpen")
          }
          title={desktopVisible ? t("contextClose") : t("contextOpen")}
          onClick={onDesktopToggle}
        >
          {desktopVisible ? (
            <ChevronRightIcon className="size-3" />
          ) : (
            <ChevronLeftIcon className="size-3" />
          )}
        </button>
      </div>

      {desktopVisible ? (
        <aside className="hidden h-full min-h-0 w-80 shrink-0 overflow-hidden xl:block">
          <ConversationContextContent
            conversation={conversation}
            contactName={contactName}
          />
        </aside>
      ) : null}

      <Sheet open={sheetOpen} onOpenChange={onSheetOpenChange}>
        <SheetContent className="data-[side=right]:w-full gap-0 p-0 sm:max-w-sm">
          <SheetHeader className="sr-only">
            <SheetTitle>{t("contextTitleBar")}</SheetTitle>
            <SheetDescription>{t("contextDescription")}</SheetDescription>
          </SheetHeader>
          <ConversationContextContent
            conversation={conversation}
            contactName={contactName}
            sheet
          />
        </SheetContent>
      </Sheet>
    </>
  )
}
