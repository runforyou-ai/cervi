/** 本地知识文档预览页面，保留列表返回位置。 */
import { useTranslation } from "react-i18next"
import { Link, useLocation, useParams } from "react-router"
import { getKnowledgeDocument } from "@/api"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { KnowledgeQAFeedback } from "./knowledge-qa-feedback"
import { KnowledgeDocumentPreview } from "./knowledge-document-preview"

/** 显示原件预览和暂不可用的分段入口。 */
export function KnowledgeDocumentPage() {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const { knowledgeBaseId = "", groupId = "", documentId = "" } = useParams()
  const location = useLocation()
  const document = useResource(
    resourceKeys.knowledgeDocument(knowledgeBaseId, documentId),
    (signal) => getKnowledgeDocument(knowledgeBaseId, documentId, signal),
    { staleTime: 0 },
  )
  const returnGroupId = document.data?.groupId ?? groupId
  const returnSearch = returnGroupId === groupId ? location.search : ""
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={document.data?.name ?? t("documents.title")}>
        <Button variant="outline" size="sm" disabled>
          {t("documentDetail.viewSegments")}
        </Button>
        <Button variant="ghost" size="sm" asChild>
          <Link to={`/knowledge-bases/${knowledgeBaseId}/groups/${returnGroupId}/documents${returnSearch}`}>
            {t("common:actions.back")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent className="overflow-hidden">
        {document.error || !document.data ? (
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
    </div>
  )
}
