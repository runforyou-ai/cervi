/** 企业知识库调用与归一化。 */
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
  type KnowledgeQASimilarQuestion,
  type KnowledgeQASummary,
  KnowledgeBaseCategory,
  type KnowledgeBase,
  type KnowledgeBaseInput,
  type KnowledgeBaseList,
  type KnowledgeGroup,
  type KnowledgeGroupInput,

} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type KnowledgeBaseCategoryId = Exclude<
  KnowledgeBaseCategory,
  KnowledgeBaseCategory.$zero
>

export type KnowledgeGroupData = Omit<KnowledgeGroup, "children"> & {
  children: KnowledgeGroupData[]
}

export type KnowledgeBaseData = Omit<KnowledgeBase, "category" | "groups"> & {
  category: KnowledgeBaseCategoryId
  groups: KnowledgeGroupData[]
}

export type KnowledgeBaseListData = Omit<
  KnowledgeBaseList,
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
export function createKnowledgeBase(
  input: KnowledgeBaseInput,
): Promise<KnowledgeBaseData> {
  return createKnowledgeBaseBound(input).then(normalizeKnowledgeBase)
}

/** 读取企业知识库详情。 */
export function getKnowledgeBase(
  knowledgeBaseId: string,
  signal?: AbortSignal,
): Promise<KnowledgeBaseData> {
  return getKnowledgeBaseBound(knowledgeBaseId, signal).then(
    normalizeKnowledgeBase,
  )
}

/** 修改企业知识库。 */
export function updateKnowledgeBase(
  knowledgeBaseId: string,
  input: KnowledgeBaseInput,
): Promise<KnowledgeBaseData> {
  return updateKnowledgeBaseBound(knowledgeBaseId, input).then(
    normalizeKnowledgeBase,
  )
}

/** 删除企业知识库。 */
export const deleteKnowledgeBase = bind(DeleteKnowledgeBase)

/** 创建知识库分组。 */
export function createKnowledgeGroup(
  knowledgeBaseId: string,
  input: KnowledgeGroupInput,
): Promise<KnowledgeBaseData> {
  return createKnowledgeGroupBound(knowledgeBaseId, input).then(
    normalizeKnowledgeBase,
  )
}

/** 修改知识库分组。 */
export function updateKnowledgeGroup(
  knowledgeBaseId: string,
  groupId: string,
  input: KnowledgeGroupInput,
): Promise<KnowledgeBaseData> {
  return updateKnowledgeGroupBound(knowledgeBaseId, groupId, input).then(
    normalizeKnowledgeBase,
  )
}

/** 删除不含子分组的知识库分组。 */
export function deleteKnowledgeGroup(
  knowledgeBaseId: string,
  groupId: string,
): Promise<KnowledgeBaseData> {
  return deleteKnowledgeGroupBound(knowledgeBaseId, groupId).then(
    normalizeKnowledgeBase,
  )
}

/** 读取当前企业的知识库列表。 */
export function listKnowledgeBases(): Promise<KnowledgeBaseListData> {
  return listKnowledgeBasesBound().then((output) => ({
    ...output,
    knowledgeBases: asList(output.knowledgeBases).map(normalizeKnowledgeBase),
  }))
}

/** 归一化知识库分组树。 */
function normalizeKnowledgeGroup(group: KnowledgeGroup): KnowledgeGroupData {
  return {
    ...group,
    children: asList(group.children).map(normalizeKnowledgeGroup),
  }
}

/** 归一化知识库详情。 */
function normalizeKnowledgeBase(
  knowledgeBase: KnowledgeBase,
): KnowledgeBaseData {
  return {
    ...knowledgeBase,
    category: knowledgeBase.category as KnowledgeBaseCategoryId,
    groups: asList(knowledgeBase.groups).map(normalizeKnowledgeGroup),
  }
}

export type KnowledgeQAEntryData = Omit<
  KnowledgeQAEntry,
  "similarQuestions"
> & {
  similarQuestions: KnowledgeQASimilarQuestion[]
}

export type KnowledgeQASummaryData = Omit<KnowledgeQASummary, "similarQuestions"> & {
  similarQuestions: NonNullable<KnowledgeQASummary["similarQuestions"]>
}

export type KnowledgeQAListData = Omit<KnowledgeQAList, "entries"> & {
  entries: KnowledgeQASummaryData[]
}

const getKnowledgeQAEntryBound = bind(GetKnowledgeQAEntry)
const createKnowledgeQAEntryBound = bind(CreateKnowledgeQAEntry)
const updateKnowledgeQAEntryBound = bind(UpdateKnowledgeQAEntry)
const listKnowledgeQAEntriesBound = bind(ListKnowledgeQAEntries)

/** 归一化问答中的相似问题列表。 */
function normalizeKnowledgeQA(entry: KnowledgeQAEntry): KnowledgeQAEntryData {
  return { ...entry, similarQuestions: asList(entry.similarQuestions) }
}

/** 读取指定分组的问答列表。 */
export function listKnowledgeQAEntries(
  knowledgeBaseId: string,
  input: KnowledgeQAListInput,
  signal?: AbortSignal,
): Promise<KnowledgeQAListData> {
  return listKnowledgeQAEntriesBound(knowledgeBaseId, input, signal).then(
    (output) => ({
      ...output,
      entries: asList(output.entries).map((entry) => ({
        ...entry,
        similarQuestions: asList(entry.similarQuestions),
      })),
    }),
  )
}

/** 读取完整问答。 */
export function getKnowledgeQAEntry(
  knowledgeBaseId: string,
  entryId: string,
  signal?: AbortSignal,
): Promise<KnowledgeQAEntryData> {
  return getKnowledgeQAEntryBound(knowledgeBaseId, entryId, signal).then(
    normalizeKnowledgeQA,
  )
}

/** 创建本地问答。 */
export function createKnowledgeQAEntry(
  knowledgeBaseId: string,
  input: KnowledgeQAInput,
): Promise<KnowledgeQAEntryData> {
  return createKnowledgeQAEntryBound(knowledgeBaseId, input).then(
    normalizeKnowledgeQA,
  )
}

/** 修改本地问答。 */
export function updateKnowledgeQAEntry(
  knowledgeBaseId: string,
  entryId: string,
  input: KnowledgeQAInput,
): Promise<KnowledgeQAEntryData> {
  return updateKnowledgeQAEntryBound(knowledgeBaseId, entryId, input).then(
    normalizeKnowledgeQA,
  )
}

/** 删除完整问答。 */
export const deleteKnowledgeQAEntry = bind(DeleteKnowledgeQAEntry)

/** 将文档列表的可空切片在 API 边界归一化。 */
export type KnowledgeDocumentData = Omit<KnowledgeDocument, "status"> & {
  status: Exclude<KnowledgeDocumentStatus, KnowledgeDocumentStatus.$zero>
}
export type KnowledgeDocumentListData = Omit<KnowledgeDocumentList, "documents"> & {
  documents: KnowledgeDocumentData[]
}
const listKnowledgeDocumentsBound = bind(ListKnowledgeDocuments)
const createKnowledgeDocumentsBound = bind(CreateKnowledgeDocuments)
/** 读取分组文档列表。 */
export async function listKnowledgeDocuments(
  baseId: string,
  input: KnowledgeDocumentListInput,
  signal?: AbortSignal,
): Promise<KnowledgeDocumentListData> {
  const result = await listKnowledgeDocumentsBound(baseId, input, signal)
  return { ...result, documents: asList(result.documents).map(normalizeKnowledgeDocument) }
}
const getKnowledgeDocumentBound = bind(GetKnowledgeDocument)
/** 读取文档详情并归一化状态类型。 */
export async function getKnowledgeDocument(
  baseId: string,
  documentId: string,
  signal?: AbortSignal,
): Promise<KnowledgeDocumentData> {
  return normalizeKnowledgeDocument(await getKnowledgeDocumentBound(baseId, documentId, signal))
}
/** 将上传原件保存为文档。 */
export async function createKnowledgeDocuments(
  baseId: string,
  input: KnowledgeDocumentBatchInput,
): Promise<Omit<KnowledgeDocumentBatch, "documents"> & { documents: KnowledgeDocumentData[] }> {
  const result = await createKnowledgeDocumentsBound(baseId, input)
  return { ...result, documents: asList(result.documents).map(normalizeKnowledgeDocument) }
}
/** 移动文档到同库分组。 */
export const moveKnowledgeDocument = bind(MoveKnowledgeDocument)
/** 删除文档并释放原件。 */
export const deleteKnowledgeDocument = bind(DeleteKnowledgeDocument)
/** 取得用于预览的原件读取请求。 */
export const getKnowledgeDocumentPreview = bind(GetKnowledgeDocumentPreview)

/** 将后端文档状态收敛为有效的业务枚举。 */
function normalizeKnowledgeDocument(document: KnowledgeDocument): KnowledgeDocumentData {
  return { ...document, status: document.status as KnowledgeDocumentData["status"] }
}
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

export type KnowledgeDocumentSegmentPageData = Omit<KnowledgeDocumentSegmentPage, "segments"> & {
  segments: NonNullable<KnowledgeDocumentSegmentPage["segments"]>
}
const listKnowledgeDocumentSegmentsBound = bind(ListKnowledgeDocumentSegments)

/** 读取固定批次的一页分段或锚点所在页。 */
export async function listKnowledgeDocumentSegments(baseId: string, documentId: string, input: KnowledgeDocumentSegmentInput, signal?: AbortSignal): Promise<KnowledgeDocumentSegmentPageData> {
  const result = await listKnowledgeDocumentSegmentsBound(baseId, documentId, input, signal)
  return { ...result, segments: asList(result.segments) }
}
