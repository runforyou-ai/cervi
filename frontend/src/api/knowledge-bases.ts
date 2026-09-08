/** 企业知识库调用与归一化。 */
import {
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
