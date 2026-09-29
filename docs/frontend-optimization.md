# 前端代码优化计划

日期：2026-09-29。

本文记录前端代码审查确认的优化事项，分两个 PR 实施。某个 PR 合入后删除本文对应小节，全部完成后删除本文。

## PR 1：缺陷、约定与渲染

### 缺陷与约定

- 移动端分页列表的加载更多与失败文案改用 `common:status.loadingMore`、`common:status.loadMoreError`，`MobilePagedList` 去掉对应 label；删除 `mobile:contacts.loadingMore`、`mobile:contacts.loadMoreError`。
- `business-hours-form.tsx` 删除以数组路径调用 `trigger` 的 watch；`service-timeouts-form.tsx` 的 watch 改为 `responseReminderMinutes` 上的 `rules.deps`。
- 网站渠道读取服务器地址改用 `resourceKeys.serverURL()` 与 `resolveServerURL`，删除 `websiteChannelOrigin` key、`resolveWebsiteChannelOrigin` 和 `trimOrigin`。
- 服务器连接表单读取已保存地址改用 `useResource(resourceKeys.serverURL())`。
- `use-conversation-summary.ts` 回到前台重读摘要只在未接入实时同步时执行。
- 登录、注册、创建工作区与首次安装表单的提交按钮移出 `FieldGroup`，表单使用 `space-y-9`。
- 收件箱搜索框去掉 placeholder。
- `account-status-toggle.tsx` 使用 `ConfirmationDialog` 默认进行中文案，删除 agents、contacts 中重复的 `status.saving`。
- 删除 common 中未使用的 `actions.refresh`、`actions.view`、`actions.new`；删除未使用的 `prepareFileUpload` 导出；删除 `@types/js-yaml`。
- `react-hook-form` 与 `@hookform/resolvers` 锁定精确版本，AGENTS.md 注明升级前验证 `lib/zod-resolver` 行为。
- api 中手写的响应容器类型（`KnowledgeDocumentContentData`、`ServiceIssueDetailData`、`AssistantDetailData`、`AssistantListData`、`ContactFieldListData`）改为从绑定类型派生。
- 去掉只在本文件使用的导出：`knowledge-gap-form.tsx` 的 `draftValues`、`KnowledgeGapDraftNotice`、`KnowledgeGapMergeHint`、`useMergeInto`，`document-preview-parser.ts` 的 `decodeDocumentText`，`timeline-message-meta.tsx` 的 `MessageTranslationLabel`。
- 收件箱中使用“避免……”“不再……”的注释改为直述；只调用一次且不超过 10 行的 `delegateDescription`、`readCollapsedSections` 内联到调用处。

### 渲染

- `useResource` 与 `usePagedResource` 的状态字段改为 getter，只订阅调用方读取的查询属性。
- `usePagedResource` 合并分页结果与 `more` 使用 memo。
- `StartupProvider`、`WorkspaceGate` 的 Context value 使用 memo。
- `useMemberChatPollingActive` 改为模块级窗口状态加 `useSyncExternalStore`。
- `useConversationTypingLabel` 的订阅函数保持稳定引用。
- `useDateTime` 缓存当前年份并 memo 返回对象；`useOutgoingMessages` memo 合并后的消息与操作函数。

## PR 2：抽取重复代码

- `useMountedRef`：替换收件箱、设置、集成、知识库中约 18 处挂载标识样板，`useAutoSave`、`useFormLifetime` 内部同步复用。
- 摘要、翻译、超时、业务时间四个自动保存表单改用 `useFormSave({ autoSave: true })`。
- `useDebouncedValue`：替换 `use-inbox-search.ts`、`composer-translation.tsx`、`customer-reply-assistant.tsx`、`team-member-picker.tsx` 的手写防抖，`useListSearchParams` 复用。
- 本机偏好读写工具：替换 `chat-rail.tsx`、`customer-reply-assistant.tsx`、`customer-copilot-panel.tsx`、`use-recent-conversations.ts` 中的 localStorage 读写。
- `useQRCode`：合并 `client-link-dialog.tsx` 与 `website-channel-usage-panel.tsx` 的二维码生成，链接变化时清空旧图并提供失败状态。
- AI 表现页与团队表现页抽取地址参数 hook 与周期、渠道筛选组件。
- 登录页与邀请页抽取账号会话跳转 hook，安装状态读取改用 `useResource`。
- 收件箱：`inbox-page.tsx` 与 `chat-route.tsx` 的打开会话 effect、弹层对齐偏移计算合并为共用实现；每条待翻译消息的 `IntersectionObserver` 改为按滚动区域共用。
- 实时客户端：合并两个原生 transport 的缓冲回放、重连退避公式，以及 `api/realtime/index.ts` 中两个客户端的装配代码。
- `role-member-dialog.tsx` 拿到首页总数后并行读取剩余成员分页。
- 拆分 `CustomerReplyAssistant`、`ConversationAttachmentUpload`、`GroupConversationProfile`、`ConversationTimelineContent`；以 effect 派生的状态改为渲染期调整，渲染期写 ref 改用 `useEffectEvent`。
