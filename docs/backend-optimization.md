# 后端代码优化清单

日期：2026-09-27。

本文记录 2026-09 对 `internal/` 的全量只读审查中确认、尚未处理的优化项。某项完成后直接删除对应条目，全部完成后删除本文。容量、限流、性能索引与安全加固不在本文范围，统一在上线前专项处理。

## 随模块顺手处理

以下各项在修改对应模块时合并处理。

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
  - `chatstate.AppendRequesterStatus` 每次调用重新读取服务会话来源，改为由调用方传入已锁定的服务会话。
  - `agentrun` 同一运行内多次 `policyForRun`；`translation.PreviewReply` 两次加载模型；`knowledgebase.DeleteDocumentAction` 重复加锁查询。
  - `team/remove_members.go` 逐成员查询与删除，改为批量。
  - MCP 服务创建或修改时工具目录远程拉取两次，改为连接测试结果直接写入。
  - `servicesummary/history.go` 读取命中周期全部消息，改为窗口函数取命中前后范围。
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
