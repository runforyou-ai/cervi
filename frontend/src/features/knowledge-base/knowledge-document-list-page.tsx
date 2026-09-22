/** 标准知识库分组文档列表、上传和原地管理。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useLocation } from "react-router"
import { KnowledgeBaseCategory, listKnowledgeDocuments } from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { KnowledgeDocumentUpload } from "./knowledge-document-upload"
import { KnowledgeDocumentActions, type DocumentAction } from "./knowledge-document-actions"
import { KnowledgeDocumentTable } from "./knowledge-document-table"
import {
  KnowledgeGroupListShell,
  KnowledgeGroupRoute,
  useKnowledgeGroupList,
} from "./knowledge-group-list"

/** 按分组隔离上传批次、弹窗和滚动恢复状态。 */
export function KnowledgeDocumentListPage() {
  return (
    <KnowledgeGroupRoute>
      {(ids) => <KnowledgeDocumentGroupList {...ids} />}
    </KnowledgeGroupRoute>
  )
}

/** 显示创建时间倒序的文档，返回时恢复原列表位置。 */
function KnowledgeDocumentGroupList({
  knowledgeBaseId,
  groupId,
}: {
  knowledgeBaseId: string
  groupId: string
}) {
  const { t } = useTranslation("knowledgeBase")
  const location = useLocation()
  const list = useKnowledgeGroupList({
    knowledgeBaseId,
    groupId,
    section: "documents",
    listKey: (parameters) => resourceKeys.knowledgeDocuments(knowledgeBaseId, parameters),
    load: (parameters, signal) => listKnowledgeDocuments(knowledgeBaseId, parameters, signal),
    processing: (data) =>
      data.documents.some((document) => document.status === "queued" || document.status === "running"),
  })
  const [action, setAction] = useState<DocumentAction | null>(null)
  return (
    <>
      <KnowledgeGroupListShell
        list={list}
        knowledgeBaseId={knowledgeBaseId}
        category={KnowledgeBaseCategory.KnowledgeBaseCategoryStandard}
        fallbackTitle={t("documents.title")}
        description={t("documents.description")}
        searchLabel={t("documents.search")}
        errorMessage={t("documents.loadError")}
        actions={<KnowledgeDocumentUpload baseId={knowledgeBaseId} groupId={groupId} />}
      >
        <KnowledgeDocumentTable
          knowledgeBaseId={knowledgeBaseId}
          data={list.list.data!}
          listPath={list.listPath}
          search={location.search}
          filtered={Boolean(list.query)}
          refreshing={list.list.isPlaceholderData}
          canMove={list.groups.length > 1}
          onAction={setAction}
          onPage={list.changePage}
        />
      </KnowledgeGroupListShell>
      {action && list.base.data && (
        <KnowledgeDocumentActions base={list.base.data} action={action} onClose={() => setAction(null)} />
      )}
    </>
  )
}
