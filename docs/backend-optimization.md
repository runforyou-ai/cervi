# 后端优化事项

日期：2026-09-29。

本文记录后端代码审查中确认的待优化事项，按优先级分组。每项写明问题、位置和改法；完成后直接删除对应条目，全部完成后删除本文。

## 1. 高收益性能

### 鉴权与实时

- 每个 `auth=member` 请求串行执行 `resolveAccount` 与 `ResolveMember` 两次查询（`appservice/direct/backend.go`）。成员鉴权合并为一条 JOIN 查询。
- 桌面设备事件流复用成员事件流，接收全部客服通知后丢弃（`realtime/gateway/gateway.go`、`devicehost/worker.go`）。新增设备专用路由，只下发 `ServerHello` 与 `DeviceWorkAdvanced`，握手不读同步头。
- 原生端实时事件向所有窗口广播，窗口数为 N 时开销为 N²；`apiproxy/realtime.go` 的 `windows` 在窗口关闭后不删除。按会话所属窗口定向发送，关闭时清理。

### 事务内逐条写库

以下位置在事务内按条执行，改为批量 SQL；任务运行时补批量入队接口供其中几处共用。

- 群聊加人：`conversation/chat_subject.go` 逐人确保聊天主体，`groupchat/group_management.go` 逐人 `restoreOrCreateGroupParticipant`，且重复查询成员是否在群内。
- 知识库重建索引：`knowledgebase/update_knowledge_base.go` 逐文档、逐问答更新并入队。
- 更换 Telegram 机器人：`chatstate/agent_lane.go` 的 `CancelChannelRuns` 对渠道全部会话加锁推进版本，只需处理有在途运行的会话。
- 启停 Telegram 渠道：`channel/update_telegram_channel_status.go` 逐会话加锁推进版本。
- 批量调整角色：`role/assignment.go` 逐人 UPDATE。
- 删除团队：`team/delete_team.go` 逐会话投递分配任务。
- 翻译写入：`translation/messages.go` 逐条 UPDATE 与 INSERT。
- 记忆写入：`agentrun/assistant_memory.go` 逐条 upsert。
- 移除 MCP 服务：`agent/revision_references.go` 的 `removeRevisionReference` 对每个员工 SELECT、INSERT、UPDATE。

### 知识检索重复向量化

`knowledgeretrieval/retrieval.go` 按“知识库 × 查询”并发，每个来源各自调用 `Embed`。检索前按供应商、模型和维度分组，一次批量向量化后分发。

### 其他性能

- 维护计划走完整可靠任务链路，每次触发约 6 次数据库写入加一次 NATS 往返（`server_tasks.go`），可让维护队列的计划在进程内直接执行扫描。
- 标记已读重复查询会话类型与服务会话，并加载目标消息整行（`conversation/mark_conversation_read.go`）。
- 联系人按名称排序时每行计算关联子查询，`primary_email` 手写了一份与 `contactname.PrimaryEmail` 相同的子查询（`contact/list_contacts.go`）。
- 周期关闭后资料抽取与标签判断串行执行，小结、质检、资料抽取各自加载一遍设置和全量对话（`servicesummary`）。
- 加团队成员对已在团队的成员也投递补分配并无条件通知（`team/add_members.go`）。
- 修改工作状态重复查询已锁定的用户行（`user/update_work_status.go`）。
- 团队存在性校验附带成员计数子查询（`team/list_members.go`、`list_member_candidates.go`）。
- `normalizeSlices` 对基础类型切片逐元素反射（`appservice/normalize.go`）。
- Office 文档整份读入并构建完整元素树（`documentconvert`）。
- 网页搜索与决策服务响应未限制读取大小（`websearch/client.go`、`decision/decision.go`）。

## 2. 重复与冗余

- `appservice/direct` 约 39 个错误映射函数重复处理 `ctx.Err()`、会话失效和字段校验错误，抽公共前置函数，各域只保留特有映射。
- 模型凭据加载复制 3 份：`translation/translation.go`、`servicesummary/servicesummary.go`、`agentrun/execute.go` 的 `managedAgentModel`。
- 附件消息手写幂等核对，改用 `conversation.LoadIdempotentMemberMessage`；单聊“查找或创建并恢复”在 `attachment_message.go` 与 `direct_conversation.go` 重复。
- `chatstate.AppendMessage` 的幂等预查询被调用方覆盖，`internal_text_message.go` 与 `group_conversation.go` 的 `!inserted` 分支走不到。
- 唯一约束冲突重试循环有 7 处，提供 `conversation` 包的统一重试函数。
- “有效真人或有效 AI 员工”查询条件与基础 join 有 6 处，在 identity 包提供统一构造函数。
- 语言与工作状态的枚举校验手写多处，在 domain 增加 `Valid()`；`role/list_roles.go` 在 SQL 中硬编码内置角色排序。
- `aiperformance/issues.go` 与 `teamperformance/issues.go` 几乎逐行相同。
- 锁定网站渠道行的代码块在 `channel/update_website_channel_*.go` 有 4 份。
- 会话上下文查询的发送者与引用 join 在 `agentrun/input_feed.go` 与 `group_execute.go` 重复。
- MCP `Discover` 重写了 `Connect` 加 `Session.Tools` 的分页遍历（`integration/mcp`）。
- SSE 写出与建流流程在 `realtime/gateway/connection.go` 与 `run_stream.go` 各一套。
- 标题截取、引用编号规范化在 directchat 与 groupchat 之间复制。
- `ListContacts` 可由生成器覆盖，去掉 `manual=api,proxy` 和对应手写路由。
- `conversation/agent_scheduler.go` 重复嵌入 `CustomerAgentMessageScheduler`。
- `mcpserver/update_tools.go` 写回工具目录未更新 `updated_at`。
- 报表公共集合关联 `contact_channel_identities` 未带 `organization_id` 条件（`aiperformance/scope.go`、`teamperformance/scope.go`）。
- 死代码：`user/validation.go` 的 `ValidationCurrentPasswordIncorrect`，`i18n` 的 `ErrorAccountReadFailed` 及其中英文词条。
