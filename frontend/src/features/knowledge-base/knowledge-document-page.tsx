/** 本地知识文档预览页面，保留列表返回位置。 */
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useParams } from "react-router"
import { getKnowledgeDocument } from "@/api"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { KnowledgeQAFeedback } from "./knowledge-qa-feedback"
import { KnowledgeSegmentsDialog } from "./knowledge-segments-dialog"
import { KnowledgeDocumentPreview } from "./knowledge-document-preview"
import { KnowledgeDocumentStatus } from "./knowledge-document-status"

/** 显示原件预览和固定批次的分段阅读入口。 */
export function KnowledgeDocumentPage() {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const { knowledgeBaseId = "", groupId = "", documentId = "" } = useParams()
  const location = useLocation()
  // 打开时固定批次，后台发布新结果不会重置当前阅读位置。
  const [segmentBatchId, setSegmentBatchId] = useState("")
  const trigger = useRef<HTMLButtonElement>(null)
  const document = useResource(
    resourceKeys.knowledgeDocument(knowledgeBaseId, documentId),
    (signal) => getKnowledgeDocument(knowledgeBaseId, documentId, signal),
    { staleTime: 0, refetchInterval: (data) => data?.status === "queued" || data?.status === "running" ? 2000 : false },
  )
  const returnGroupId = document.data?.groupId ?? groupId
  const returnSearch = returnGroupId === groupId ? location.search : ""
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={document.data?.name ?? t("documents.title")}>
        <Button ref={trigger} variant="outline" size="sm" disabled={!document.data?.segmentBatchId} onClick={() => setSegmentBatchId(document.data?.segmentBatchId ?? "")}>
          {t("documentDetail.viewSegments")}
        </Button>
        <Button variant="ghost" size="sm" asChild>
          <Link to={`/knowledge-bases/${knowledgeBaseId}/groups/${returnGroupId}/documents${returnSearch}`}>
            {t("common:actions.back")}
          </Link>
        </Button>
      </PageHeader>
      <div className="flex min-h-10 shrink-0 items-center gap-3 px-6 text-sm text-muted-foreground" role="status">
        {document.data && <KnowledgeDocumentStatus document={document.data} />}
      </div>
      <PageContent className="overflow-hidden">
        {!document.data ? (
          <KnowledgeQAFeedback error={document.error} retry={() => void document.refresh()} />
        ) : (
          <KnowledgeDocumentPreview
            key={documentId}
            knowledgeBaseId={knowledgeBaseId}
            documentId={documentId}
            name={document.data.name}
          />
        )}
      </PageContent>
      {segmentBatchId && document.data && <KnowledgeSegmentsDialog
        knowledgeBaseId={knowledgeBaseId} documentId={documentId} documentName={document.data.name}
        segmentBatchId={segmentBatchId} triggerRef={trigger} onClose={() => setSegmentBatchId("")}
      />}
    </div>
  )
}
