/** 企业知识库调用。 */
import {
  RetryKnowledgeDocument,
  ListKnowledgeDocumentSegments,
  ListKnowledgeDocuments,
  GetKnowledgeDocument,
  CreateKnowledgeDocuments,
  MoveKnowledgeDocument,
  DeleteKnowledgeDocument,
  GetKnowledgeDocumentPreview,
  CreateKnowledgeQAEntry,
  UpdateKnowledgeQAEntry,
  GetKnowledgeQAEntry,
  ListKnowledgeQAEntries,
  DeleteKnowledgeQAEntry,
  CreateKnowledgeBase,
  CreateKnowledgeGroup,
  DeleteKnowledgeBase,
  DeleteKnowledgeGroup,
  GetKnowledgeBase,
  ListKnowledgeBases,
  UpdateKnowledgeBase,
  UpdateKnowledgeGroup,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  type KnowledgeDocumentSegmentInput,
  type KnowledgeDocumentSegmentPage,
  KnowledgeDocumentStatus,
  type KnowledgeDocumentList,
  type KnowledgeDocumentListInput,
  type KnowledgeDocument,
  type KnowledgeDocumentBatch,
  type KnowledgeDocumentBatchInput,
  type KnowledgeQAEntry,
  type KnowledgeQAInput,
  type KnowledgeQAList,
  type KnowledgeQAListInput,
  type KnowledgeQASummary,
  KnowledgeBaseCategory,
  type KnowledgeBase,
  type KnowledgeBaseInput,
  type KnowledgeBaseList,
  type KnowledgeGroup,
  type KnowledgeGroupInput,

} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type KnowledgeBaseCategoryId = Exclude<
  KnowledgeBaseCategory,
  KnowledgeBaseCategory.$zero
>

export type KnowledgeGroupData = NonNullArrays<KnowledgeGroup>

export type KnowledgeBaseData = Omit<NonNullArrays<KnowledgeBase>, "category"> & {
  category: KnowledgeBaseCategoryId
}

export type KnowledgeBaseListData = Omit<
  NonNullArrays<KnowledgeBaseList>,
  "knowledgeBases"
> & {
  knowledgeBases: KnowledgeBaseData[]
}

const createKnowledgeBaseBound = bind(CreateKnowledgeBase)
const getKnowledgeBaseBound = bind(GetKnowledgeBase)
const updateKnowledgeBaseBound = bind(UpdateKnowledgeBase)
const createKnowledgeGroupBound = bind(CreateKnowledgeGroup)
const updateKnowledgeGroupBound = bind(UpdateKnowledgeGroup)
const deleteKnowledgeGroupBound = bind(DeleteKnowledgeGroup)
const listKnowledgeBasesBound = bind(ListKnowledgeBases)
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

/** 创建知识库分组。 */
export function createKnowledgeGroup(
  knowledgeBaseId: string,
  input: KnowledgeGroupInput,
) {
  return createKnowledgeGroupBound(
    knowledgeBaseId,
    input,
  ) as Promise<KnowledgeBaseData>
}

/** 修改知识库分组。 */
export function updateKnowledgeGroup(
  knowledgeBaseId: string,
  groupId: string,
  input: KnowledgeGroupInput,
) {
  return updateKnowledgeGroupBound(
    knowledgeBaseId,
    groupId,
    input,
  ) as Promise<KnowledgeBaseData>
}

/** 删除不含子分组的知识库分组。 */
export function deleteKnowledgeGroup(knowledgeBaseId: string, groupId: string) {
  return deleteKnowledgeGroupBound(
    knowledgeBaseId,
    groupId,
  ) as Promise<KnowledgeBaseData>
}

/** 读取当前企业的知识库列表。 */
export function listKnowledgeBases() {
  return listKnowledgeBasesBound() as Promise<KnowledgeBaseListData>
}

export type KnowledgeQAEntryData = NonNullArrays<KnowledgeQAEntry>

export type KnowledgeQASummaryData = NonNullArrays<KnowledgeQASummary>

export type KnowledgeQAListData = NonNullArrays<KnowledgeQAList>

const getKnowledgeQAEntryBound = bind(GetKnowledgeQAEntry)
const createKnowledgeQAEntryBound = bind(CreateKnowledgeQAEntry)
const updateKnowledgeQAEntryBound = bind(UpdateKnowledgeQAEntry)
const listKnowledgeQAEntriesBound = bind(ListKnowledgeQAEntries)

/** 读取指定分组的问答列表。 */
export function listKnowledgeQAEntries(
  knowledgeBaseId: string,
  input: KnowledgeQAListInput,
  signal?: AbortSignal,
) {
  return listKnowledgeQAEntriesBound(knowledgeBaseId, input, signal)
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

export type KnowledgeDocumentData = Omit<
  NonNullArrays<KnowledgeDocument>,
  "status"
> & {
  status: Exclude<KnowledgeDocumentStatus, KnowledgeDocumentStatus.$zero>
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
const listKnowledgeDocumentsBound = bind(ListKnowledgeDocuments)
const createKnowledgeDocumentsBound = bind(CreateKnowledgeDocuments)
/** 读取分组文档列表。 */
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
/** 移动文档到同库分组。 */
export const moveKnowledgeDocument = bind(MoveKnowledgeDocument)
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
