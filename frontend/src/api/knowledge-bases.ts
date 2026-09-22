/** 企业知识库调用。 */
import {
  RetryKnowledgeDocument,
  CreateKnowledgeTextDocument,
  CreateKnowledgeWebDocument,
  RefetchKnowledgeDocument,
  GetKnowledgeDocumentContent,
  UpdateKnowledgeDocumentContent,
  RenameKnowledgeDocument,
  RetrieveKnowledgeBase,
  ListKnowledgeDocumentSegments,
  ListKnowledgeDocuments,
  GetKnowledgeDocument,
  CreateKnowledgeDocuments,
  DeleteKnowledgeDocument,
  GetKnowledgeDocumentPreview,
  CreateKnowledgeQAEntry,
  UpdateKnowledgeQAEntry,
  GetKnowledgeQAEntry,
  ListKnowledgeQAEntries,
  DeleteKnowledgeQAEntry,
  RetryKnowledgeQAEntry,
  CreateKnowledgeBase,
  DeleteKnowledgeBase,
  GetKnowledgeBase,
  ListKnowledgeBaseAgents,
  ListKnowledgeBases,
  UpdateKnowledgeBase,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  type KnowledgeDocumentSegmentInput,
  type KnowledgeDocumentSegmentPage,
  KnowledgeIndexStatus,
  KnowledgeDocumentSourceKind,
  type KnowledgeDocumentList,
  type KnowledgeDocumentListInput,
  type KnowledgeDocument,
  type KnowledgeDocumentBatch,
  type KnowledgeDocumentBatchInput,
  type KnowledgeTextDocumentInput,
  type KnowledgeWebDocumentInput,
  type KnowledgeDocumentRefetchInput,
  type KnowledgeDocumentContentInput,
  type KnowledgeDocumentRenameInput,
  type KnowledgeQAEntry,
  type KnowledgeQAInput,
  type KnowledgeQAList,
  type KnowledgeQAListInput,
  type KnowledgeQASummary,
  KnowledgeBaseCategory,
  type KnowledgeBase,
  type KnowledgeBaseAgentList,
  type KnowledgeBaseInput,
  type KnowledgeBaseList,
  type KnowledgeRetrievalInput,
  type KnowledgeRetrievalResult,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type KnowledgeBaseCategoryId = Exclude<
  KnowledgeBaseCategory,
  KnowledgeBaseCategory.$zero
>

export type KnowledgeBaseData = Omit<NonNullArrays<KnowledgeBase>, "category"> & {
  category: KnowledgeBaseCategoryId
}

export type KnowledgeBaseAgentListData = NonNullArrays<KnowledgeBaseAgentList>

export type KnowledgeBaseListData = Omit<
  NonNullArrays<KnowledgeBaseList>,
  "knowledgeBases"
> & {
  knowledgeBases: KnowledgeBaseData[]
}

const createKnowledgeBaseBound = bind(CreateKnowledgeBase)
const getKnowledgeBaseBound = bind(GetKnowledgeBase)
const updateKnowledgeBaseBound = bind(UpdateKnowledgeBase)
const listKnowledgeBasesBound = bind(ListKnowledgeBases)
const listKnowledgeBaseAgentsBound = bind(ListKnowledgeBaseAgents)
/** 创建企业知识库。 */
export function createKnowledgeBase(input: KnowledgeBaseInput) {
  return createKnowledgeBaseBound(input) as Promise<KnowledgeBaseData>
}

/** 读取企业知识库详情。 */
export function getKnowledgeBase(knowledgeBaseId: string, signal?: AbortSignal) {
  return getKnowledgeBaseBound(
    knowledgeBaseId,
    signal,
  ) as Promise<KnowledgeBaseData>
}

/** 修改企业知识库。 */
export function updateKnowledgeBase(
  knowledgeBaseId: string,
  input: KnowledgeBaseInput,
) {
  return updateKnowledgeBaseBound(
    knowledgeBaseId,
    input,
  ) as Promise<KnowledgeBaseData>
}

/** 删除企业知识库。 */
export const deleteKnowledgeBase = bind(DeleteKnowledgeBase)

/** 读取当前配置版本绑定知识库的 AI 员工。 */
export function listKnowledgeBaseAgents(knowledgeBaseId: string) {
  return listKnowledgeBaseAgentsBound(knowledgeBaseId) as Promise<KnowledgeBaseAgentListData>
}

/** 读取当前企业的知识库列表。 */
export function listKnowledgeBases() {
  return listKnowledgeBasesBound() as Promise<KnowledgeBaseListData>
}

export type KnowledgeIndexStatusId = Exclude<
  KnowledgeIndexStatus,
  KnowledgeIndexStatus.$zero
>

export type KnowledgeQAEntryData = NonNullArrays<KnowledgeQAEntry>

export type KnowledgeQASummaryData = Omit<
  NonNullArrays<KnowledgeQASummary>,
  "status"
> & {
  status: KnowledgeIndexStatusId
}

export type KnowledgeQAListData = Omit<
  NonNullArrays<KnowledgeQAList>,
  "entries"
> & {
  entries: KnowledgeQASummaryData[]
}

const getKnowledgeQAEntryBound = bind(GetKnowledgeQAEntry)
const createKnowledgeQAEntryBound = bind(CreateKnowledgeQAEntry)
const updateKnowledgeQAEntryBound = bind(UpdateKnowledgeQAEntry)
const listKnowledgeQAEntriesBound = bind(ListKnowledgeQAEntries)

/** 读取指定知识库的问答列表。 */
export function listKnowledgeQAEntries(
  knowledgeBaseId: string,
  input: KnowledgeQAListInput,
  signal?: AbortSignal,
) {
  return listKnowledgeQAEntriesBound(
    knowledgeBaseId,
    input,
    signal,
  ) as Promise<KnowledgeQAListData>
}

/** 读取完整问答。 */
export function getKnowledgeQAEntry(
  knowledgeBaseId: string,
  entryId: string,
  signal?: AbortSignal,
) {
  return getKnowledgeQAEntryBound(knowledgeBaseId, entryId, signal)
}

/** 创建本地问答。 */
export function createKnowledgeQAEntry(
  knowledgeBaseId: string,
  input: KnowledgeQAInput,
) {
  return createKnowledgeQAEntryBound(knowledgeBaseId, input)
}

/** 修改本地问答。 */
export function updateKnowledgeQAEntry(
  knowledgeBaseId: string,
  entryId: string,
  input: KnowledgeQAInput,
) {
  return updateKnowledgeQAEntryBound(knowledgeBaseId, entryId, input)
}

/** 删除完整问答。 */
export const deleteKnowledgeQAEntry = bind(DeleteKnowledgeQAEntry)

/** 按当前配置重新索引问答。 */
export const retryKnowledgeQAEntry = bind(RetryKnowledgeQAEntry)

export type KnowledgeDocumentSourceKindId = Exclude<
  KnowledgeDocumentSourceKind,
  KnowledgeDocumentSourceKind.$zero
>
export type KnowledgeDocumentData = Omit<
  NonNullArrays<KnowledgeDocument>,
  "status" | "sourceKind"
> & {
  status: KnowledgeIndexStatusId
  sourceKind: KnowledgeDocumentSourceKindId
}
export type KnowledgeDocumentListData = Omit<
  NonNullArrays<KnowledgeDocumentList>,
  "documents"
> & {
  documents: KnowledgeDocumentData[]
}
export type KnowledgeDocumentBatchData = Omit<
  NonNullArrays<KnowledgeDocumentBatch>,
  "documents"
> & {
  documents: KnowledgeDocumentData[]
}
export type KnowledgeDocumentContentData = {
  document: KnowledgeDocumentData
  content: string
}
const createKnowledgeTextDocumentBound = bind(CreateKnowledgeTextDocument)
/** 创建在线编写的文档。 */
export function createKnowledgeTextDocument(
  baseId: string,
  input: KnowledgeTextDocumentInput,
) {
  return createKnowledgeTextDocumentBound(
    baseId,
    input,
  ) as Promise<KnowledgeDocumentData>
}
const createKnowledgeWebDocumentBound = bind(CreateKnowledgeWebDocument)
/** 导入网页作为知识文档。 */
export function createKnowledgeWebDocument(
  baseId: string,
  input: KnowledgeWebDocumentInput,
) {
  return createKnowledgeWebDocumentBound(
    baseId,
    input,
  ) as Promise<KnowledgeDocumentData>
}
const refetchKnowledgeDocumentBound = bind(RefetchKnowledgeDocument)
/** 重新抓取网页文档。 */
export function refetchKnowledgeDocument(
  baseId: string,
  documentId: string,
  input: KnowledgeDocumentRefetchInput,
) {
  return refetchKnowledgeDocumentBound(baseId, documentId, input)
}
const getKnowledgeDocumentContentBound = bind(GetKnowledgeDocumentContent)
/** 读取在线文档正文或网页抓取快照。 */
export function getKnowledgeDocumentContent(
  baseId: string,
  documentId: string,
  signal?: AbortSignal,
) {
  return getKnowledgeDocumentContentBound(
    baseId,
    documentId,
    signal,
  ) as Promise<KnowledgeDocumentContentData>
}
const updateKnowledgeDocumentContentBound = bind(UpdateKnowledgeDocumentContent)
/** 保存在线文档的名称与正文。 */
export function updateKnowledgeDocumentContent(
  baseId: string,
  documentId: string,
  input: KnowledgeDocumentContentInput,
) {
  return updateKnowledgeDocumentContentBound(
    baseId,
    documentId,
    input,
  ) as Promise<KnowledgeDocumentData>
}
const renameKnowledgeDocumentBound = bind(RenameKnowledgeDocument)
/** 修改在线文档或网页文档的名称。 */
export function renameKnowledgeDocument(
  baseId: string,
  documentId: string,
  input: KnowledgeDocumentRenameInput,
) {
  return renameKnowledgeDocumentBound(
    baseId,
    documentId,
    input,
  ) as Promise<KnowledgeDocumentData>
}
const listKnowledgeDocumentsBound = bind(ListKnowledgeDocuments)
const createKnowledgeDocumentsBound = bind(CreateKnowledgeDocuments)
/** 读取知识库文档列表。 */
export function listKnowledgeDocuments(
  baseId: string,
  input: KnowledgeDocumentListInput,
  signal?: AbortSignal,
) {
  return listKnowledgeDocumentsBound(
    baseId,
    input,
    signal,
  ) as Promise<KnowledgeDocumentListData>
}
const getKnowledgeDocumentBound = bind(GetKnowledgeDocument)
/** 读取文档详情。 */
export function getKnowledgeDocument(
  baseId: string,
  documentId: string,
  signal?: AbortSignal,
) {
  return getKnowledgeDocumentBound(
    baseId,
    documentId,
    signal,
  ) as Promise<KnowledgeDocumentData>
}
/** 将上传原件保存为文档。 */
export function createKnowledgeDocuments(
  baseId: string,
  input: KnowledgeDocumentBatchInput,
) {
  return createKnowledgeDocumentsBound(
    baseId,
    input,
  ) as Promise<KnowledgeDocumentBatchData>
}
/** 删除文档并释放原件。 */
export const deleteKnowledgeDocument = bind(DeleteKnowledgeDocument)
/** 取得用于预览的原件读取请求。 */
export const getKnowledgeDocumentPreview = bind(GetKnowledgeDocumentPreview)

/** 使用服务端签发的请求直接读取原件。 */
export async function readKnowledgeDocumentPreview(
  baseId: string,
  documentId: string,
  signal?: AbortSignal,
): Promise<Uint8Array<ArrayBuffer>> {
  const request = await getKnowledgeDocumentPreview(baseId, documentId, signal)
  const headers = new Headers()
  for (const [name, value] of Object.entries(request.headers ?? {})) if (value !== undefined) headers.set(name, value)
  const response = await fetch(request.url, { headers, signal, cache: "no-store" })
  if (!response.ok) throw new Error(`Document preview failed: ${response.status}`)
  return new Uint8Array(await response.arrayBuffer())
}

/** 按当前配置重新处理文档。 */
export const retryKnowledgeDocument = bind(RetryKnowledgeDocument)

export type KnowledgeDocumentSegmentPageData =
  NonNullArrays<KnowledgeDocumentSegmentPage>
const listKnowledgeDocumentSegmentsBound = bind(ListKnowledgeDocumentSegments)

/** 读取固定批次的一页分段或锚点所在页。 */
export function listKnowledgeDocumentSegments(baseId: string, documentId: string, input: KnowledgeDocumentSegmentInput, signal?: AbortSignal) {
  return listKnowledgeDocumentSegmentsBound(baseId, documentId, input, signal)
}

export type KnowledgeRetrievalResultData = NonNullArrays<KnowledgeRetrievalResult>
const retrieveKnowledgeBaseBound = bind(RetrieveKnowledgeBase)

/** 在指定知识库中执行检索测试。 */
export function retrieveKnowledgeBase(baseId: string, input: KnowledgeRetrievalInput) {
  return retrieveKnowledgeBaseBound(baseId, input) as Promise<KnowledgeRetrievalResultData>
}
