/** 待补知识调用。 */
import {
  AcceptKnowledgeGap,
  DismissKnowledgeGap,
  GetKnowledgeGap,
  ListKnowledgeGaps,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  KnowledgeGapDraftStatus,
  KnowledgeGapMessageSender,
  KnowledgeGapSource,
  KnowledgeGapStatus,
  type KnowledgeGap,
  type KnowledgeGapList,
  type KnowledgeGapListInput,
  type KnowledgeGapMessage,
  type KnowledgeGapSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type KnowledgeGapSourceId = Exclude<KnowledgeGapSource, KnowledgeGapSource.$zero>

export type KnowledgeGapStatusId = Exclude<KnowledgeGapStatus, KnowledgeGapStatus.$zero>

export type KnowledgeGapSummaryData = Omit<NonNullArrays<KnowledgeGapSummary>, "source" | "status"> & {
  source: KnowledgeGapSourceId
  status: KnowledgeGapStatusId
}

export type KnowledgeGapListData = Omit<NonNullArrays<KnowledgeGapList>, "gaps"> & {
  gaps: KnowledgeGapSummaryData[]
}

export type KnowledgeGapMessageData = Omit<KnowledgeGapMessage, "sender"> & {
  sender: Exclude<KnowledgeGapMessageSender, KnowledgeGapMessageSender.$zero>
}

export type KnowledgeGapData = Omit<NonNullArrays<KnowledgeGap>, "source" | "status" | "draftStatus" | "messages"> & {
  source: KnowledgeGapSourceId
  status: KnowledgeGapStatusId
  draftStatus: Exclude<KnowledgeGapDraftStatus, KnowledgeGapDraftStatus.$zero>
  messages: KnowledgeGapMessageData[]
}

const listKnowledgeGapsBound = bind(ListKnowledgeGaps)
const getKnowledgeGapBound = bind(GetKnowledgeGap)

/** 读取一页指定处理状态的待补知识。 */
export function listKnowledgeGaps(input: KnowledgeGapListInput) {
  return listKnowledgeGapsBound(input) as Promise<KnowledgeGapListData>
}

/** 读取待补知识详情。 */
export function getKnowledgeGap(gapId: string, signal?: AbortSignal) {
  return getKnowledgeGapBound(gapId, signal) as Promise<KnowledgeGapData>
}

/** 把待补知识整理的问答加入知识库。 */
export const acceptKnowledgeGap = bind(AcceptKnowledgeGap)

/** 忽略待补知识。 */
export const dismissKnowledgeGap = bind(DismissKnowledgeGap)
