# 后端优化事项

日期：2026-09-29。

本文记录后端代码审查中确认的待优化事项，按优先级分组。每项写明问题、位置和改法；完成后直接删除对应条目，全部完成后删除本文。

## 1. 重复与冗余

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
