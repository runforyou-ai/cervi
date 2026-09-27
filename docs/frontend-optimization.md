# 前端优化计划

日期：2026-09-27。

本文记录前端代码审查发现的待办优化事项，按 PR 分组，某项完成后直接删除对应内容。会话时间线的窗口裁剪与虚拟化已列入 [路线图](roadmap.md)「实时同步」的「会话列表虚拟化与窗口裁剪」，按其启动条件开启，本文不重复。

## 代码整理：重复代码、死代码与代码组织

### API 层

- `api/inbox.ts`、`api/knowledge-bases.ts` 中只做参数透传的包装函数统一改为 `export const x = bind(X)`；只保留补默认值或改形状的函数（如 `loadInbox`、`transferServiceSession`、`markConversationRead`）。函数体内重复 `bind` 的调用改为模块顶层绑定一次。
- `loadInbox` 与 `searchInbox` 共用同一组筛选默认值；四个会话类型守卫合并为按 `type` 判别的一组实现。
- `api/inbox.ts` 精简后按会话、服务周期、群聊、Copilot 拆分文件，由 `api/index.ts` 汇总导出。

### 共享实现

- 新增 `requestErrorMessage(error, fields?)`：接口错误取服务端文案，其余返回 `common:errors.network`。替换 29 处 `isApiError ? apiErrorMessage : networkError` 判断，删除各 namespace 中 8 份“无法连接服务器”文案。
- `ResourceTable` 默认隐藏表头，需要表头的列表显式声明 `showHeader`，删除各调用处的 `hideHeader`。
- 新增 `use-copy-feedback`，替换设置页、邀请弹窗、网站渠道使用说明中 3 份“复制后显示已复制”的实现，词条并入 `common`。
- `use-pending-image-upload` 的 `ensureUploaded` 在上传失败时返回统一结果，助理表单、AI 员工表单、移动端群资料等 6 处不再各自维护上传中标志和异常分支。
- 标签、服务分类、联系人字段三个设置页抽出共用的字典式列表外壳（列表、新建编辑弹窗、删除确认、打开时聚焦名称）。
- `service-summary-settings` 与 `translation-settings` 共用按服务商分组模型的实现。
- `unicodeLength` 统一放到 `lib`，替换 4 处重复实现。
- 工作状态切换在桌面端用户菜单与移动端“我”页共用一个 hook。

### 死代码

- 删除未引用的 `features/contacts/team-checkbox-options.tsx`。
- 删除未使用的导出：`ReadonlyDetailRow`、`userStatusLabel`、`renameKnowledgeDocument`、`LoadInboxQuery`、`MessageAttachmentTransferStatusId`；只在本文件使用的类型与常量去掉 `export`。
- 删除未使用的翻译键：`auth:invalidCredentials`、`auth:serverError`、`auth:registrationClosed`、`setup:serverError`、`common:actions.loadMore`、`contacts:columns.accountStatus`、`contacts:validation.checkFields`。

### 国际化

- 业务 namespace 中重复 `common` 的“正在处理…”“返回”“重试”“编辑”“删除”“正在删除…”改用 `common` 键。
- “{{time}} 添加”与“添加时间”列名从 agents、channels、contacts 合并到 `common`。
- AI 助理在线状态标签合并为一套，inbox 只保留“离线，电脑上线后回复”。

### 代码组织

- contacts 下被 settings、agents 共用的 `RoleSelectField`、`TeamSelectField`、`account-status-toggle`、`MemberAssistantsSection` 移到 `src/components`，`use-contact-invalidator`、`use-contact-search` 移到 `src/hooks`。
- `attachment-queue-context`、`outgoing-message-context` 移到 `src/contexts`；`use-conversation-name`、`account/workspace-schema`、`notification-permission-settings` 等跨 feature 引用按同一规则上移。
- 拆分超大组件：`ConversationTimelineContent` 抽出消息定位与回到最新、行操作两个 hook；`workspace-navigation.tsx` 拆出用户菜单与工作区设置菜单；`ai-knowledge-gap-sheet.tsx` 拆出表单与操作 hook；`timeline-message-bubble.tsx` 拆出引用、时间与翻译标签。
- 表单约定：营业时间表单去掉与浏览器原生提示重复的校验文字；`model-picker-dialog` 的可选模型读取改用 `useResource`。
- 补齐约 24 处缺失的函数注释，4 处写在 `useAutoSave` 调用上方的注释移到函数定义前。

## 首屏与包体积

构建前后记录首屏与访客端产物体积，逐项验证收益。

- `api/index.ts` 以 `export *` 聚合全部绑定模型，首屏预加载约 212KB 的 API chunk。为 `src/api` 与 `bindings` 声明无副作用，使打包按实际使用裁剪；验证生成的模型无副作用。
- 语言包按命名空间懒加载，首屏只加载 common、auth、setup、connection、account，其余命名空间随各 feature 路由加载。
- 网站访客端 `markdown.js` 约 719KB，其中 parse5（rehype-raw）、shiki 语言表与 mermaid 文案不会用到。在 messenger 构建中移除这些依赖，或换用更轻的渲染组合。
- 知识库表格预览改用 `xlsx` 的 mini 构建。
- 字体只引入实际使用的字符子集。
- `@types/hast` 加入 devDependencies。
- 知识库文档预览的 docx、xlsx 解析移到 Web Worker，工作表按当前页签生成 HTML。
