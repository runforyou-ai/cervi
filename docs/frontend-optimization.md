# 前端代码优化计划

日期：2026-09-29。

本文记录前端代码审查确认、尚未实施的组件拆分事项，完成后删除本文。

## 拆分大组件

- 拆分 `CustomerReplyAssistant`、`ConversationAttachmentUpload`、`GroupConversationProfile`、`ConversationTimelineContent`。
- 以 effect 派生的状态改为渲染期调整：`inbox-page.tsx` 的消息定位、`group-conversation-context.tsx` 失去管理权限时的编辑与解散状态。
- 渲染期写 ref 改用 `useEffectEvent`：`use-agent-run-stream.ts`、`inbox-conversation-list.tsx`、`conversation-timeline.tsx`。
