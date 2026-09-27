# 后端代码优化清单

日期：2026-09-27。

本文记录 2026-09 对 `internal/` 的全量只读审查中确认的优化项，按 PR 批次分组。某项完成后直接删除对应条目，全部完成后删除本文。容量、限流、性能索引与安全加固不在本文范围，统一在上线前专项处理。

## 批次一：正确性缺陷

1. **超时扫描被无收件人的队列提醒占满**（`actions/servicetimeout/worker.go`）
   - 队列提醒在没有工作中客服时直接返回，不写 `reminded_at`；扫描按等待时长升序每次取 200 条。
   - 此类周期累计超过 200 条后，全部署的负责人回收、真人提醒、AI 跟进与 AI 关单全部停止。
   - 处理：扫描条件只选取存在可提醒客服的队列提醒周期。
2. **渠道路由校验团队时不加锁**（`actions/channel/routing.go`）
   - 与删除团队并发时，渠道会写入已删除团队的路由，新会话进入不可见队列。
   - 处理：团队分支改用 `FOR KEY SHARE` 锁定团队行，与删除团队的 `FOR UPDATE` 互斥。
3. **原生端文件地址依赖手工类型清单补全**（`apiproxy/backend.go` 的 `normalizeOutput`）
   - 本地存储时服务端返回相对地址 `/storage/...`，原生端按类型清单补成绝对地址；`AssistantList`、`ServiceAssigneeList`、`InboxSearchResult`、`ConversationAttention`、`ServiceCopilotThreadList` 与会话负责人头像均未覆盖，原生端显示不出头像。
   - 处理：服务端本地存储地址以 `PUBLIC_URL` 拼接为绝对地址，删除 `normalizeOutput`、`absoluteContentURL` 及生成器中的对应调用。
4. **模型 HTTP 客户端 2 分钟总超时截断流式输出**（`integration/agentruntime/model.go`）
   - `http.Client.Timeout` 覆盖读取响应体的全过程，长推理或长回复在 2 分钟时被断开，整次运行失败。
   - 处理：去掉客户端总超时，由运行 ctx 控制时限；Transport 设置 `ResponseHeaderTimeout` 发现卡死连接。
5. **本地配置写入先删除原文件**（`integration/localmcp/store.go`、`integration/toolchain/archive.go`）
   - 删除与改名之间进程退出时，本地 MCP 配置全部丢失。Go 的 `os.Rename` 在 Windows 上覆盖已有文件。
   - 处理：删除 `os.Remove`，直接改名覆盖。

## 批次二：客服链路吞吐与锁

6. **Telegram 外发依赖 5 秒扫描推进**（`actions/customerdelivery/worker.go`）
   - 抢不到渠道锁或前序投递未完成时直接返回成功，`finish` 不唤醒同一渠道身份的下一条；同一渠道每轮扫描约只发出一条。
   - 按整个渠道检查 `status='sending'`，进程崩溃遗留的发送租约让整个渠道停发至租约过期。
   - 处理：`finish` 在事务内为同一渠道身份的下一条投递入队；未抢到锁时短延迟重新入队；`sending` 检查收窄到当前渠道身份。
7. **Telegram 入站同步拉取头像**（`actions/channel/receive_telegram_webhook.go`）
   - 每条入站消息（含重放）在响应前同步调用 `GetUserProfilePhoto`，最长 15 秒。
   - 处理：只在渠道身份新建时，在入站事务内投递幂等键为渠道身份的异步头像任务。
8. **每条入站消息都解析并锁定新周期路由**（`actions/conversation/receive_channel_message.go`、`actions/chatstate/route.go`）
   - 已有进行中周期时仍加 `KEY SHARE` 锁并多查 2～4 次；路由不可用时每条消息打一条告警日志。
   - 处理：只在需要开新周期时解析并锁定路由。
9. **资料变更逐条推进会话版本**（`actions/chatstate/profile.go` 的 `touchProfileConversations`）
   - 先锁住全部命中会话，再逐条 `UPDATE` 并逐条查受众，查询数约为会话数的 3 倍。
   - 处理：一条 `UPDATE ... WHERE id IN (...) RETURNING` 批量推进，受众按会话类型批量查询。
10. **运行开始时串行连接 MCP 服务**（`integration/agentruntime/mcp.go`）
    - 本地服务单个握手时限 2 分钟，多服务时等待时间累加。
    - 处理：并发执行 `openMCPServer`，登记与去重保持原顺序。
11. **AI 表现报表重复执行同一重型 CTE**（`actions/aiperformance/overview.go`、`breakdown.go`）
    - 处理：拆分页用 `count(*) OVER ()` 取总数；概览页合并为一条 SQL。

## 批次三：领域模型收敛

12. **服务周期状态迁移多处手写**
    - 指派负责人、退回队列、关闭分散在 `conversation/manage_service_session.go`、`conversation/send_service_text_message.go`、`serviceassignment/assign.go`、`agentrun/customer_handoff.go`、`servicetimeout/worker.go` 等处，规则已出现分叉（如 AI 关单不清 `queued_at`）。
    - 处理：在 `chatstate` 收敛为指派、退回队列、关闭三个迁移函数，同时维护数据库与内存模型。同一事务内对服务会话行的重复加锁与查询（`chatstate/service.go`）一并收敛。
13. **客服设置缺行按默认值兜底**
    - 默认值同时存在于迁移、`domain.DefaultServiceTimeouts()`、扫描 SQL 的 `COALESCE` 与 5 个 upsert 中。
    - 处理：创建工作区时写入设置行，列上给齐默认值；各更新改为普通 `UPDATE`，删除读取侧的缺行分支。
14. **网站访客类型借用 `external_id` 前缀表达**
    - `web-session:`、`web-user:` 前缀决定是否已验签，前缀常量定义三份。
    - 处理：渠道身份增加显式的访客类型字段。

## 批次四：分层调整

15. **本地对象上传的授权与归属校验位于 Gin 层**（`api/file_content.go`）
    - 处理：文件 Action 提供本地对象上传授权，经 appservice 暴露，HTTP 层只读写请求与映射状态码；删除无调用方的 `VerifyOrganizationCustomer`。
16. **Telegram 回调解析与业务过滤位于 Gin 层**（`api/telegram_webhook.go`）
    - 处理：Update 解析与过滤移入 `integration/telegram` 或 channel Action，Gin 只读取请求体。
17. **HTTP 横切逻辑重复**
    - 错误响应体有 `apiError`、网关 `writeError` 与 `appservice.Error` 三套；`RequestMeta` 解析在 `api/service.go`、`realtime/gateway/gateway.go`、`realtime/gateway/run_stream.go` 三处实现，后者缺 DeviceID。
    - 处理：appservice 提供唯一的错误写出与 `RequestMetaFromHTTP`。
18. **实时网关接口暴露存储模型**（`realtime/gateway/gateway.go` 的 `MemberBackend`）
    - 处理：改用只含所需字段的身份结构体。
19. **directOperations 中的业务逻辑**
    - `ListInboxChannels` 的排序移入 Query；网站访客 `ListMessages` 的游标互斥校验移入 Action。
    - 渠道启停按类型分派（`appservice/direct_backend_channel.go`）移入 Action，通用启停 Action 的类型条件排除 Telegram。

## 随模块顺手处理

以下各项在修改对应模块时合并处理，不单独开 PR。

- **重复实现**
  - 知识文档与问答的向量化加发布流程（`knowledgebase/process_document.go`、`process_qa_entry.go`）。
  - 用户头像激活与回收（`user/update_profile.go`）改用 `fileaction.ActivateLinkedImage`、`RetireLinkedImage`。
  - 成员与访客的完成上传流程（`file/complete_upload.go`、`file/visitor.go`），并发落后方重读后按幂等返回。
  - 联系方式追加（`contact/ensure_channel_identity.go` 的 `AddEmail`、`contactprofile/extraction.go`），统一数量上限与组织条件。
  - 主 Agent 与子 Agent 的中间件装配（`agentruntime/eino.go`、`subagent.go`）。
  - 从模型输出截取 JSON 对象（`agent_chat_title.go`、`translation.go`、`servicesummary.go`、`reply_candidates.go`）。
  - 服务端运行与设备运行的能力装配（`agentrun/execute.go`、`device_run.go`）；托管配置 JOIN 与运行内容块组装（`agentrun/execute.go`）。
  - 模型服务供应商引用判断（`aiprovider/delete_ai_provider.go`、`helpers.go`）。
  - 按周期编号锁会话与周期（`servicetimeout`、`servicesummary`、`conversation/service_session_summaries.go`）与队列比较 `sameTeam`。
  - 桌面端与移动端 SQLite 存储代码（`storage/desktop`、`storage/mobile`），迁移与 models 保持独立。
  - `team.LoadIdentityTeams` 与 `LoadTeamsByIdentity`；`task/server/registry.go` 的解码闭包；devicehost 的运行时限、流回调与尾部缓冲。
- **重复查询**
  - `agentrun` 同一运行内多次 `policyForRun`；`translation.PreviewReply` 两次加载模型；`knowledgebase.DeleteDocumentAction` 重复加锁查询。
  - `team/remove_members.go` 逐成员查询与删除，改为批量。
  - MCP 服务创建或修改时工具目录远程拉取两次，改为连接测试结果直接写入。
  - `servicesummary/history.go` 读取命中周期全部消息，改为窗口函数取命中前后范围。
- **死代码**：`customerdelivery.LoadRoute`、`appservice.(*Error).WithState`。
- **零散缺陷**
  - `auth/official_login.go` 以 `la.id::text` 比较，改为校验 UUID 后按主键比较。
  - `account/register.go` 注册时锁整张 `accounts` 表，删除表锁。
  - `knowledgebase/document_processing.go` 手写锁定未校验 UUID，改用 `lockDocument`。
  - `agentrun/execute.go` 运行配置失效时返回可重试错误，改为 `task.Permanent`。
  - `toolchain` 下载源探测失败结果被永久缓存，改为只缓存成功结果。
  - `localskill/fetch.go` 为判断格式整包读入内存，改为读取文件头。
  - `embedding`、`localworkspace` 丢弃原始错误原因；Ollama 发现逐模型串行请求并整段记录响应体。
  - `integration/telegram/client.go` 的 `HTTPStatusError` 与 `telegramAPIError` 分类重复。
  - 分页参数越界处理不一致（`knowledgebase/document_query.go` 静默纠正），`agent/list_agents.go` 的 ILIKE 未转义。
  - `domain.UserStatus` 同时用于 AI 员工状态，改为中性命名。
- **约定**：`integration`、`devicehost` 中约 17 处注释使用“不再……”表述；`devicehost/worker.go` 的 `localAgentKinds`、`errorReason` 应内联。
