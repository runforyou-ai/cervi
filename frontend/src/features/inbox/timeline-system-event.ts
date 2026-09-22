/** 将会话系统事件转换为当前语言的时间线文案。 */
import type { TFunction } from "i18next"

import {
  ConversationSystemEventType,
  ServiceSessionReturnReason,
  ServiceSessionTargetKind,
  type ConversationMessageData,
  type ConversationSystemEventParticipant,
} from "@/api"

import { handoffReasonKey } from "./agent-process"

type TimelineSystemEvent = NonNullable<ConversationMessageData["systemEvent"]>

type TimelineTranslate = TFunction<["inbox", "common"]>

/** 按当前语言连接系统事件中的成员姓名。 */
function formatGroupParticipantNames(names: string[], t: TimelineTranslate) {
  if (names.length < 2) return names[0] ?? ""
  if (names.length === 2) {
    return names.join(t("groupSystemListPairSeparator"))
  }
  const previousNames = names.slice(0, -1).join(t("groupSystemListSeparator"))
  return `${previousNames}${t("groupSystemListFinalSeparator")}${names[names.length - 1]}`
}

/** 客服处理周期去向的时间线文案：成员取名称快照，团队与公共队列按队列文案展示。 */
function sessionTargetText(
  target: TimelineSystemEvent["sessionTarget"],
  currentIdentityID: string,
  t: TimelineTranslate,
) {
  if (target?.kind === ServiceSessionTargetKind.ServiceSessionTargetMember) {
    return target.identityId === currentIdentityID
      ? t("messageSenderYou")
      : (target.displayName ?? t("unknownSender"))
  }
  return target?.kind === ServiceSessionTargetKind.ServiceSessionTargetTeam
    ? t("handoffTargetTeam", { name: target.teamName ?? "" })
    : t("handoffTargetPublicQueue")
}

/** 将类型化系统事件转换为当前语言的时间线文案。 */
export function formatSystemEvent(
  event: TimelineSystemEvent,
  currentIdentityID: string,
  t: TimelineTranslate,
) {
  // 转人工事件按去向与原因码本地化，名称取事件写入时的快照。
  if (event.type === ConversationSystemEventType.ConversationSystemEventServiceSessionHandedOff) {
    return t("serviceSessionHandedOff", {
      agent: event.fromDisplayName ?? t("unknownSender"),
      target: sessionTargetText(event.sessionTarget, currentIdentityID, t),
      reason: t(handoffReasonKey(event.handoffReason)),
    })
  }
  // 退回队列事件没有操作人，按退回原因展示原负责人与退回去向。
  if (
    event.type ===
    ConversationSystemEventType.ConversationSystemEventServiceSessionReturned
  ) {
    const key =
      event.returnReason === ServiceSessionReturnReason.ServiceSessionReturnResponseTimeout
        ? "serviceSessionReturnedResponseTimeout"
        : "serviceSessionReturned"
    return t(key, {
      from:
        event.fromIdentityId === currentIdentityID
          ? t("messageSenderYou")
          : (event.fromDisplayName ?? t("unknownSender")),
      target: sessionTargetText(event.sessionTarget, currentIdentityID, t),
    })
  }
  // 自动分配事件没有操作人，只展示承接成员。
  if (
    event.type ===
    ConversationSystemEventType.ConversationSystemEventServiceSessionAssigned
  ) {
    return t("serviceSessionAssigned", {
      target: sessionTargetText(event.sessionTarget, currentIdentityID, t),
    })
  }
  const participantName = (
    participant: ConversationSystemEventParticipant,
  ) =>
    participant.identityId === currentIdentityID
      ? t("messageSenderYou")
      : participant.displayName
  const actor = participantName(event.actor)
  const targets = formatGroupParticipantNames(
    event.targets.map(participantName),
    t,
  )
  switch (event.type) {
    case ConversationSystemEventType.ConversationSystemEventServiceSessionClaimed:
      return t("serviceSessionClaimed", { actor })
    case ConversationSystemEventType.ConversationSystemEventServiceSessionTakenOver:
      return t("serviceSessionTakenOver", {
        actor,
        from:
          event.fromIdentityId === currentIdentityID
            ? t("messageSenderYou")
            : (event.fromDisplayName ?? t("unknownSender")),
      })
    case ConversationSystemEventType.ConversationSystemEventServiceSessionTransferred:
      return t("serviceSessionTransferred", {
        actor,
        target: sessionTargetText(event.sessionTarget, currentIdentityID, t),
      })
    case ConversationSystemEventType.ConversationSystemEventServiceSessionClosed:
      return t("serviceSessionClosed", { actor })
    case ConversationSystemEventType.ConversationSystemEventServiceSessionReopened:
      return t("serviceSessionReopened", { actor })
    case ConversationSystemEventType.ConversationSystemEventGroupRenamed:
      return t("groupSystemRenamed", {
        actor,
        previousTitle: event.previousTitle,
        title: event.title,
      })
    case ConversationSystemEventType.ConversationSystemEventGroupMembersAdded:
      return t("groupSystemMembersAdded", { actor, targets })
    case ConversationSystemEventType.ConversationSystemEventGroupMemberRemoved:
      return t("groupSystemMemberRemoved", {
        actor,
        target: targets,
      })
    case ConversationSystemEventType.ConversationSystemEventGroupMemberLeft:
      return t("groupSystemMemberLeft", { actor })
    case ConversationSystemEventType.ConversationSystemEventGroupOwnerTransferred:
      return t("groupSystemOwnerTransferred", {
        actor,
        target: targets,
      })
    case ConversationSystemEventType.ConversationSystemEventGroupDissolved:
      return t("groupSystemDissolved", { actor })
    default:
      return t("groupSystemUpdated")
  }
}
