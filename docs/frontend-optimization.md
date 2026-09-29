# 前端代码优化计划

日期：2026-09-29。

本文记录前端代码审查确认、尚未实施的重复代码抽取事项，完成后删除本文。

## 抽取重复代码

- `useMountedRef`：替换收件箱、设置、集成、知识库中约 18 处挂载标识样板，`useAutoSave`、`useFormLifetime` 内部同步复用。
- 摘要、翻译、超时、业务时间四个自动保存表单改用 `useFormSave({ autoSave: true })`。
- `useDebouncedValue`：替换 `use-inbox-search.ts`、`composer-translation.tsx`、`customer-reply-assistant.tsx`、`team-member-picker.tsx` 的手写防抖，`useListSearchParams` 复用。
- 本机偏好读写工具：替换 `chat-rail.tsx`（含 `readCollapsedSections`）、`customer-reply-assistant.tsx`、`customer-copilot-panel.tsx`、`use-recent-conversations.ts` 中的 localStorage 读写。
- `useQRCode`：合并 `client-link-dialog.tsx` 与 `website-channel-usage-panel.tsx` 的二维码生成，链接变化时清空旧图并提供失败状态。
- AI 表现页与团队表现页抽取地址参数 hook 与周期、渠道筛选组件。
- 登录页与邀请页抽取账号会话跳转 hook，安装状态读取改用 `useResource`。
- 收件箱：`inbox-page.tsx` 与 `chat-route.tsx` 的打开会话 effect、弹层对齐偏移计算合并为共用实现；每条待翻译消息的 `IntersectionObserver` 改为按滚动区域共用。
- 实时客户端：合并两个原生 transport 的缓冲回放、重连退避公式，以及 `api/realtime/index.ts` 中两个客户端的装配代码。
- `role-member-dialog.tsx` 拿到首页总数后并行读取剩余成员分页。
- 拆分 `CustomerReplyAssistant`、`ConversationAttachmentUpload`、`GroupConversationProfile`、`ConversationTimelineContent`；以 effect 派生的状态改为渲染期调整，渲染期写 ref 改用 `useEffectEvent`。
