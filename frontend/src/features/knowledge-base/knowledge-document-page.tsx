/** 外部知识文档原文件预览页面与分段阅读入口。 */
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useParams } from "react-router"
import { getKnowledgeBase, getKnowledgeDocument, isApiError } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageBack } from "@/components/page-back"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { KnowledgeDocumentPreview } from "./knowledge-document-preview"
import { KnowledgeSegmentsDialog } from "./knowledge-segments-dialog"

/** 在独立页面预览文档，并通过模态框查看连续分段。 */
export function KnowledgeDocumentPage() {
  const { t } = useTranslation("knowledgeBase")
  const { knowledgeBaseId = "", documentId = "" } = useParams()
  const [segmentsOpen, setSegmentsOpen] = useState(false)
  const trigger = useRef<HTMLButtonElement>(null)
  const knowledgeBase = useResource(resourceKeys.knowledgeBase(knowledgeBaseId), (signal) => getKnowledgeBase(knowledgeBaseId, signal), { enabled: knowledgeBaseId !== "" })
  const document = useResource(resourceKeys.knowledgeDocument(knowledgeBaseId, documentId), (signal) => getKnowledgeDocument(knowledgeBaseId, documentId, signal), {
    enabled: Boolean(knowledgeBase.data?.integrationConnectionId) && documentId !== "", staleTime: Infinity, refetchOnWindowFocus: false,
  })
  const error = knowledgeBase.error ?? document.error
  if (knowledgeBase.data?.integrationConnectionId === "") return <Navigate replace to={`/knowledge-bases/${knowledgeBaseId}`} />
  return <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
    <PageHeader title={document.data?.name ?? t("documentDetail.title")}>
      <Button ref={trigger} variant="outline" size="sm" disabled={!document.data} onClick={() => setSegmentsOpen(true)}>{t("documentDetail.viewSegments")}</Button>
      <PageBack to={`/knowledge-bases/${knowledgeBaseId}/documents`} />
    </PageHeader>
    <PageContent className="overflow-hidden">
      {error ? <div className="flex h-full flex-col items-center justify-center gap-4 text-sm text-muted-foreground">
        <p>{isApiError(error) ? apiErrorMessage(error) : t("documentDetail.loadError")}</p>
        <Button variant="outline" onClick={() => void (knowledgeBase.error ? knowledgeBase.refresh() : document.refresh())}>{t("retry")}</Button>
      </div> : document.data ? <KnowledgeDocumentPreview key={`${knowledgeBaseId}-${documentId}`} knowledgeBaseId={knowledgeBaseId} documentId={documentId} /> : <LoadingIndicator className="h-full justify-center">{t("documentDetail.loading")}</LoadingIndicator>}
    </PageContent>
    {segmentsOpen && document.data && <KnowledgeSegmentsDialog knowledgeBaseId={knowledgeBaseId} documentId={documentId} documentName={document.data.name} triggerRef={trigger} onClose={() => setSegmentsOpen(false)} />}
  </div>
}
