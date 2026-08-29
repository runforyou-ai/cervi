/** 企业知识库调用与归一化。 */
import {
  CreateKnowledgeBase,
  CreateKnowledgeGroup,
  DeleteKnowledgeBase,
  DeleteKnowledgeGroup,
  GetKnowledgeBase,
  GetKnowledgeDocument,
  ListExternalKnowledgeBaseOptions,
  ListKnowledgeBases,
  ListKnowledgeDocumentSegments,
  ListKnowledgeDocuments,
  UpdateKnowledgeBase,
  UpdateKnowledgeGroup,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  KnowledgeBaseCategory,
  KnowledgeDocumentSegmentIndexStatus,
  KnowledgeDocumentStatus,
  type ExternalKnowledgeBaseOption,
  type ExternalKnowledgeBaseOptionList,
  type KnowledgeBase,
  type KnowledgeBaseInput,
  type KnowledgeBaseList,
  type KnowledgeDocument,
  type KnowledgeDocumentList,
  type KnowledgeDocumentListInput,
  type KnowledgeDocumentSegment,
  type KnowledgeDocumentSegmentList,
  type KnowledgeDocumentSegmentListInput,
  type KnowledgeDocumentSummary,
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

export type ExternalKnowledgeBaseOptionData = Omit<
  ExternalKnowledgeBaseOption,
  "category"
> & {
  category: KnowledgeBaseCategoryId
}

export type ExternalKnowledgeBaseOptionListData = Omit<
  ExternalKnowledgeBaseOptionList,
  "knowledgeBases"
> & {
  knowledgeBases: ExternalKnowledgeBaseOptionData[]
}

export type KnowledgeDocumentStatusId = Exclude<
  KnowledgeDocumentStatus,
  KnowledgeDocumentStatus.$zero
>

export type KnowledgeDocumentData = Omit<
  KnowledgeDocumentSummary,
  "status"
> & {
  status: KnowledgeDocumentStatusId
}

export type KnowledgeDocumentListData = Omit<
  KnowledgeDocumentList,
  "documents"
> & {
  documents: KnowledgeDocumentData[]
}

export type KnowledgeDocumentListQuery = Partial<KnowledgeDocumentListInput>

export type KnowledgeDocumentDetailData = Omit<
  KnowledgeDocument,
  "status"
> & {
  status: KnowledgeDocumentStatusId
}

export type KnowledgeDocumentSegmentIndexStatusId = Exclude<
  KnowledgeDocumentSegmentIndexStatus,
  KnowledgeDocumentSegmentIndexStatus.$zero
>

export type KnowledgeDocumentSegmentData = Omit<
  KnowledgeDocumentSegment,
  "indexStatus"
> & {
  indexStatus: KnowledgeDocumentSegmentIndexStatusId
}

export type KnowledgeDocumentSegmentListData = Omit<
  KnowledgeDocumentSegmentList,
  "segments"
> & {
  segments: KnowledgeDocumentSegmentData[]
}

export type KnowledgeDocumentSegmentListQuery = Partial<
  KnowledgeDocumentSegmentListInput
>

const createKnowledgeBaseBound = bind(CreateKnowledgeBase)
const getKnowledgeBaseBound = bind(GetKnowledgeBase)
const updateKnowledgeBaseBound = bind(UpdateKnowledgeBase)
const createKnowledgeGroupBound = bind(CreateKnowledgeGroup)
const updateKnowledgeGroupBound = bind(UpdateKnowledgeGroup)
const deleteKnowledgeGroupBound = bind(DeleteKnowledgeGroup)
const listKnowledgeBasesBound = bind(ListKnowledgeBases)
const listExternalKnowledgeBaseOptionsBound = bind(
  ListExternalKnowledgeBaseOptions,
)
const listKnowledgeDocumentsBound = bind(ListKnowledgeDocuments)
const getKnowledgeDocumentBound = bind(GetKnowledgeDocument)
const listKnowledgeDocumentSegmentsBound = bind(
  ListKnowledgeDocumentSegments,
)

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

/** 读取指定 Dify 连接可访问的知识库选项。 */
export function listExternalKnowledgeBaseOptions(
  connectionId: string,
): Promise<ExternalKnowledgeBaseOptionListData> {
  return listExternalKnowledgeBaseOptionsBound(connectionId).then((output) => ({
    ...output,
    knowledgeBases: asList(output.knowledgeBases).map((knowledgeBase) => ({
      ...knowledgeBase,
      category: knowledgeBase.category as KnowledgeBaseCategoryId,
    })),
  }))
}

/** 读取指定外部知识库的一页文档。 */
export function listKnowledgeDocuments(
  knowledgeBaseId: string,
  query: KnowledgeDocumentListQuery = {},
  signal?: AbortSignal,
): Promise<KnowledgeDocumentListData> {
  return listKnowledgeDocumentsBound(
    knowledgeBaseId,
    {
      keyword: query.keyword ?? "",
      status: query.status ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 20,
    },
    signal,
  ).then((output) => ({
    ...output,
    documents: asList(output.documents).map((document) => ({
      ...document,
      status: document.status as KnowledgeDocumentStatusId,
    })),
  }))
}

/** 读取指定外部知识文档详情。 */
export function getKnowledgeDocument(
  knowledgeBaseId: string,
  documentId: string,
  signal?: AbortSignal,
): Promise<KnowledgeDocumentDetailData> {
  return getKnowledgeDocumentBound(knowledgeBaseId, documentId, signal).then(
    (document) => ({
      ...document,
      status: document.status as KnowledgeDocumentStatusId,
    }),
  )
}

/** 读取指定外部知识文档的一页分段。 */
export function listKnowledgeDocumentSegments(
  knowledgeBaseId: string,
  documentId: string,
  query: KnowledgeDocumentSegmentListQuery = {},
  signal?: AbortSignal,
): Promise<KnowledgeDocumentSegmentListData> {
  return listKnowledgeDocumentSegmentsBound(
    knowledgeBaseId,
    documentId,
    {
      keyword: query.keyword ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 20,
    },
    signal,
  ).then((output) => ({
    ...output,
    segments: asList(output.segments).map((segment) => ({
      ...segment,
      indexStatus:
        segment.indexStatus as KnowledgeDocumentSegmentIndexStatusId,
    })),
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
