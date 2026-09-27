/** 问题会话侧栏：展示周期的问题类型、小结与对客沟通。 */
import { useTranslation } from "react-i18next"

import { getAIPerformanceIssue } from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useResource } from "@/hooks/use-resource"

import { issueTypesOf } from "./ai-performance-format"
import { ServiceTranscript } from "./service-transcript"

/** 按周期编号打开侧栏，serviceSessionId 为空时关闭。 */
export function AIPerformanceIssueSheet({
  serviceSessionId,
  onClose,
}: {
  serviceSessionId: string
  onClose: () => void
}) {
  const { t } = useTranslation("agents")
  const { formatDateTime } = useDateTime()
  const detail = useResource(
    resourceKeys.aiPerformanceIssue(serviceSessionId),
    (signal) => getAIPerformanceIssue(serviceSessionId, signal),
    { enabled: Boolean(serviceSessionId) },
  )
  const data = detail.data?.issue.serviceSessionId === serviceSessionId ? detail.data : undefined

  return (
    <Sheet open={Boolean(serviceSessionId)} onOpenChange={(open) => (open ? undefined : onClose())}>
      <SheetContent className="w-full gap-0 p-0 sm:max-w-xl">
        <SheetHeader className="border-b px-6 py-4 pr-12">
          <SheetTitle>{t("performance.issueSheet.title")}</SheetTitle>
          <SheetDescription>
            {data
              ? [
                  ...issueTypesOf(data.issue).map((issue) => t(`performance.issueTypes.${issue}`)),
                  t("records.closedAt", { time: formatDateTime(data.issue.closedAt) }),
                ].join(" · ")
              : null}
          </SheetDescription>
        </SheetHeader>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-6">
            <ResourceContent resources={detail} errorMessage={t("performance.issueSheet.loadError")}>
              {data ? (
                <div className="space-y-9">
                  {data.issue.summary ? (
                    <p className="text-sm leading-6 whitespace-pre-wrap break-words">{data.issue.summary}</p>
                  ) : null}
                  <ServiceTranscript conversationId={data.issue.conversationId} messages={data.messages} />
                </div>
              ) : null}
            </ResourceContent>
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
