# Cervi AI 智能体路线图

## 1. 文档目的

本文定义 Cervi 将 AI Agent 建设为一等产品能力时的架构边界、运行事实、可靠性模型、客户端能力调用方式和阶段顺序。

本文与 `chat-roadmap.md` 的关系如下：

- `chat-roadmap.md` 负责聊天身份、会话、参与者、消息、同步和外部投递等通用事实。
- `chat-roadmap.md` 也是 Realtime Gateway、`realtime_outbox`、Core NATS、Protobuf 实时协议、连接票据、恢复和背压的权威设计；本文只定义 Agent 和设备能力需要增加的实时事件。
- 本文负责 Agent 配置、运行、工具、审批、设备能力和 Eino 接入。
- Agent 继续沿统一聊天路径发送消息，不创建第二套 Agent 会话或消息系统。
- 本文不提前创建尚未进入开发阶段的表和字段；文中的后续对象只在对应阶段出现首个真实场景时落地。

## 2. 当前代码事实

以下内容已经存在：

- `organization_identities.type = agent` 和 `agents` 已提供企业 AI 员工身份、状态、团队关系及管理接口。
- AI Provider 和模型目录已经存在，可以保存企业配置的模型服务。
- 服务端已有 PostgreSQL、NATS JetStream、`task_runs + task_outbox`、数据库租约、心跳和至少一次任务执行能力。
- 原生端通过 API Proxy 使用 Bearer Token 调用企业服务端，Token 当前保存在 WebView `localStorage`。
- 文件模块已有临时上传、激活、过期清理、本地存储和对象存储路径。

以下内容尚未实现，仍属于路线图决策：

- `chat_subjects`、`conversations`、`messages` 等聊天事实表。
- Agent 的模型选择、系统指令、工具策略和不可变配置版本。
- `conversation_agent_policies`、`conversation_agent_states`、`agent_runs` 和运行审计。
- 产品级 WebSocket 尚未实现，但 `chat-roadmap.md` 已确定统一 Realtime Gateway、Protobuf 协议、连接认证、同步恢复和背压方案；设备注册和 Capability Executor 仍未设计落地。
- 客户端可靠任务 Runtime；当前只有按真实场景落地的书面方案。
- Eino 依赖和任何 Agent 运行引擎实现。

现有 Wails MCP 只用于开发期桌面页面检查，不属于 Cervi 产品中的设备能力协议。

## 3. 总体架构决策

### 3.1 Agent 是业务身份，不是认证主体

Agent 继续使用现有企业身份：

```text
agent
  -> organization_identity
  -> chat_subject
  -> conversation_participant
  -> message.sender_participant_id
```

保持以下边界：

- Agent 可以加入会话、被 @、发送和引用消息，并进入统一审计链。
- Agent 不持有用户 Bearer Token，不以 Agent 身份登录 HTTP 或设备通道。
- Agent 的一次执行由企业策略和触发上下文授权，不继承某名用户的全部本机权限。
- 不创建 `bot`、`local_agent` 或第二套聊天主体；未来本地运行只改变 Run 的执行位置，不改变业务身份。

### 3.2 前期服务端是大脑，客户端是受控执行器

前期架构为：

```text
消息与业务触发
  -> 服务端 Agent Runtime
      -> 服务端业务工具
      -> Device Capability Gateway
          -> 持久化设备调用与 realtime_outbox
          -> Core NATS
          -> Realtime Gateway
          -> 同一 WSS 连接推送设备工作水位
          -> 客户端经 HTTP 领取并由 Capability Executor 执行
```

客户端 Capability Executor 不是 Agent：

- 不运行模型和 Eino。
- 不维护 Conversation 上下文。
- 不选择下一项工具。
- 只执行已经通过服务端策略和本机校验的具体调用。
- 负责本机权限、授权目录、审批界面、参数约束、结果裁剪和设备侧幂等。

完整本地 Agent 只在离线、隐私、本地模型或高频编码循环形成真实产品需求后引入。

### 3.3 三层事实严格分离

| 层级 | 事实 | 职责 |
| --- | --- | --- |
| 聊天业务 | `messages`、参与者、会话 | 用户可见的聊天时间线与发送者 |
| Agent 业务 | Agent 配置版本、Run、语义步骤、工具调用、审批 | 审计、费用、恢复、人工接管和产品状态 |
| 基础设施 | `task_runs + task_outbox`、NATS | 至少一次唤醒、租约、Worker 调度和临时发布 |

Eino Session、Checkpoint 和 BackgroundTask 若以后启用，只是某个 Run 的执行附件，不是聊天、审计或计费的事实来源。

### 3.4 数据库是真相，实时连接只负责在线能力

- 全产品只使用 `chat-roadmap.md` 定义的一个版本化 Realtime WebSocket 和 `cervi.realtime.v1` Protobuf 协议，不为设备能力另建连接、JSON WebSocket 或 MCP Transport。
- 聊天变化通过 Mailbox/Inbox 和会话同步水位通知，客户端经 HTTP 补拉；WebSocket 不复制完整业务 DTO。
- AI token 流可以使用 `AIStreamStarted/Delta/Completed/Failed` 临时帧，增量不写入 Message、Changelog 或 Outbox；最终消息和 Run 状态仍从 PostgreSQL 与 HTTP 恢复。
- 设备调用先写入 PostgreSQL，并在同一事务写 `realtime_outbox`；Realtime Gateway 只推送设备工作水位，参数领取、进度、结果和持久取消状态走 HTTP。
- WSS、Gateway 或 Core NATS 丢失通知后，客户端按设备工作水位和待领取列表恢复。
- WSS 在线只表示连接存在，不代表设备可弹审批、拥有 OS 权限或能在后台可靠执行。
- Agent Runtime 不持有 `connection_id`，只请求某个用户或指定设备上的某项类型化能力。

## 4. 长期不变量

以下规则现在写入设计，后续实现不得绕开：

1. Conversation、Agent Run、Task Run、工具调用和设备调用是不同对象。
2. Agent 是企业聊天身份，不是登录 Principal。
3. `messages` 是聊天事实；Agent Run 和工具调用是运行事实；`task_runs` 只负责唤醒和租约。
4. Eino 通过内部适配层接入，Eino 类型不得进入 `domain`、`storage` 或 `appservice` 契约。
5. 同一“会话 + Agent”默认只有一个在途 Run，但不能因此丢失运行期间到达的新消息。
6. 流式 token、progress tick、框架 callback 和调试日志不得写成永久语义步骤。
7. 外部副作用必须区分确定成功、确定失败和结果未知；结果未知时禁止盲重试。
8. Checkpoint 可以丢弃，消息、Run、工具调用和审批事实不可丢弃。
9. Device Gateway 是统一编排入口，但 Action、Gateway 和 Executor 必须分层校验。
10. 客户会话默认没有设备能力；按 Agent、会话类型、能力、设备和当前 Run 逐层放开。
11. 大文件和大结果只传文件引用，不进入 WSS 调用载荷或模型上下文。
12. 移动端默认是前台确认与选择器，不承担无人值守企业 Worker 职责。
13. 不为未来本地运行提前创建第二套身份、表或空字段；真实落地时直接调整目标模型。
14. Device Capability Gateway 复用 Realtime Gateway 的连接、票据、Protobuf、Outbox、NATS 和背压，不建设第二套实时基础设施。

## 5. Agent 配置与版本

### 5.1 配置权威源

现有 `agents` 只保存身份和启停状态。进入首个 Agent 运行阶段前，需要增加不可变的 Agent 配置版本，例如：

```text
agent_revisions
├── id
├── organization_id
├── agent_id
├── provider_model_id
├── system_instruction
├── generation_config
├── tool_policy
├── schema_version
├── created_by_user_id
└── created_at
```

并由 `agents.active_revision_id` 指向当前生效版本。项目不创建数据库外键，因此 Action 必须在事务中校验企业、Agent、Provider Model 和配置版本的关联。

规则：

- Revision 创建后不可修改；编辑 Agent 配置时创建新 Revision 并切换当前版本。
- 已被 Run 引用的 Revision 不物理删除。
- Provider 密钥和 Endpoint 仍属于 Provider 配置，不复制到 Revision 或 Run。
- Tool Policy 保存产品能力标识和策略，不保存 Eino Tool 实例或 Go 类型。
- Eino 升级不能改变历史 Revision 的业务含义；必要时通过 `schema_version` 解释。

### 5.2 Run 配置快照

每个 Run 同时保存：

- `agent_revision_id`：指向权威配置版本。
- 规范化配置快照：实际使用的 Provider、模型标识、系统指令哈希、生成参数和允许工具集合。

Revision 负责长期配置历史，Run 快照负责证明本次执行实际使用了什么。快照不得包含 Provider 明文密钥。

## 6. 会话触发、游标与并发

### 6.1 策略和状态

进入 AI 阶段时增加：

```text
conversation_agent_policies
├── organization_id
├── conversation_id
├── agent_identity_id
├── trigger_mode
├── allowed_tools
├── response_policy
├── enabled
└── timestamps

conversation_agent_states
├── organization_id
├── conversation_id
├── agent_identity_id
├── desired_originated_at
├── desired_message_id
├── processed_originated_at
├── processed_message_id
├── summary_message_id
├── paused_at
└── updated_at
```

`desired_*` 表示已经观察到、希望 Agent 处理到的位置；`processed_*` 表示最后一次成功纳入处理范围的位置。两者都使用 `(originated_at, message_id)`，不能使用 UUID 大小推导时间顺序。

### 6.2 不丢唤醒的单 Run 模型

每个“会话 + Agent”最多一个 `queued/running` Run，属于上下文执行互斥，不是消息合并机制。

触发流程：

1. 新消息事务锁定对应 `conversation_agent_states`。
2. 推进 `desired_*`。
3. 若当前没有在途 Run，则创建 `queued` Run，并通过 `TxEnqueuer.EnqueueIn` 在同一事务写入唤醒任务。
4. 若已有在途 Run，不重复创建；当前 Run 结束时必须再次检查游标差距。
5. Run 完成事务推进 `processed_*` 到本次输入末尾。
6. 如果 `desired_* > processed_*`，在同一事务创建下一 Run 并入队。

这样可以同时保证单 Agent 上下文串行和运行期间新消息不丢失。

迟到的历史补拉默认只进入消息时间线，不推进自动响应的 `desired_*`；需要时由独立总结或人工回放操作处理。

## 7. Agent Run 与审计模型

### 7.1 Agent Run

首个目标模型：

```text
agent_runs
├── id
├── organization_id
├── conversation_id
├── agent_identity_id
├── agent_revision_id
├── trigger_type
├── trigger_message_id
├── input_snapshot
├── config_snapshot
├── output_message_id
├── status
├── token_and_cost_usage
├── error_code
├── error_detail
├── started_at
├── completed_at
├── created_at
└── updated_at
```

规则：

- 一次 Run 对应一次有界的服务端 Agent 循环，不对应 Conversation 的常驻进程。
- `input_snapshot` 保存实际传给模型的有序输入、消息编号、内容版本或哈希、摘要引用和 schema 版本，不能只保存起止消息编号。
- 输入快照按审计需求保存必要内容并限制大小；敏感字段按产品策略处理。
- Agent 输出仍以 Agent 参与者身份创建 `messages`。
- 输出消息、Run 终态和 `processed_*` 的推进必须原子提交。
- 业务失败先写入 `agent_runs`，Task Handler 随后正常结束；只有基础设施级临时失败才触发 `task_runs` 重试，避免任务重试、模型重试和工具重试相乘。
- P1 不提前增加 `runtime` 或 `device_id`。只有整段 Agent 循环真正迁移到设备时才给 Run 增加运行位置。

### 7.2 语义步骤

`agent_run_steps` 只保存用户和运维真正需要审计的有界步骤：

```text
agent_run_steps
├── id
├── organization_id
├── agent_run_id
├── position
├── type
├── status
├── summary
├── usage
├── error
└── timestamps
```

首期类型只包括模型调用、工具调用、审批和人工交接。Eino 原始事件只用于驱动投影，不直接决定业务表结构。

以下内容不得逐条写入 Step：

- 流式 token delta。
- 高频 progress tick。
- Middleware callback。
- Worker 心跳和租约刷新。
- 调试日志和框架内部重试日志。

详细技术日志进入普通日志和可观测性系统；确有跨断线逐事件回放需求时，再增加有保留期的 `agent_run_events`，不能污染永久步骤表。

### 7.3 工具调用

工具调用拥有独立业务事实，避免 `agent_run_steps` 成为通用事件垃圾桶：

```text
agent_tool_invocations
├── id
├── organization_id
├── agent_run_id
├── step_id
├── tool_name
├── arguments
├── arguments_hash
├── idempotency_key
├── status
├── result
├── result_file_id
├── error
├── dispatched_at
├── completed_at
├── created_at
└── updated_at
```

设备能力落地时才增加或启用：

```text
executor_type
device_id
device_invocation_id
uncertain_at
```

规则：

- `arguments_hash` 使用规范化参数计算，并参与审批和防重放。
- 大结果只保存摘要和 `result_file_id`。
- 服务端工具也通过类型化 Tool Adapter 调用 Action，不允许 Eino Tool 绕过组织边界直接拼 SQL。
- 一次 Tool Invocation 对应一次业务副作用意图；基础设施重试不能创建新的副作用意图。

## 8. 服务端任务运行时边界

`task_runs + task_outbox` 继续复用现有服务端任务能力，但只负责：

- 可靠唤醒 `agent.dispatch` 或以后增加的 `agent.resume`。
- Worker 租约、心跳和基础设施错误退避。
- NATS 发布失败后的恢复。

它不负责：

- Agent Run 的产品状态。
- 工具调用、审批或设备调用账本。
- 模型与工具费用。
- 外部副作用永久幂等。
- Exactly Once。

实现要求：

- 在首个需要消息事务与 Agent 唤醒原子提交的场景前，公开 `TxEnqueuer.EnqueueIn`。
- Task Payload 只携带 `organization_id` 和 `agent_run_id`；Handler 重新加载并校验业务事实。
- Handler 以 `agent_run_id` 幂等领取 Run，不假设一次 Task Delivery 只执行一次。
- 长 Agent Run 使用独立队列或独立 Worker 配额，不能耗尽文件清理、投递和定时任务的 Worker。
- 数据库扫描或游标差距是恢复正确性来源，NATS 任务只是降低延迟的快路径。

## 9. Eino 接入边界

### 9.1 版本策略

- 不把 Eino v0.10 或任何具体版本写成产品架构不变量。
- 实施时精确锁定所选版本；禁止依赖浮动版本。
- 若 v0.10 仍为 alpha，只允许在明确接受升级成本的实验分支使用；正式接入优先选择当时稳定版本，或等待 v0.10 正式发布。
- 升级 Eino 时通过适配层吸收破坏性变化，不让框架类型扩散到业务层。

### 9.2 Agent Engine 适配层

服务端定义内部 Agent Engine 接口，输入输出只使用 Cervi 类型：

```text
AgentEngine
├── Run
├── Resume（确有恢复场景后增加）
└── Cancel
```

Eino 实现负责：

- 将 Cervi 配置 Revision 转成 ChatModelAgent、Tool 和 Middleware。
- 执行 Runner 并消费流式事件。
- 把有限语义事件投影为 Run Step 和 Tool Invocation。
- 通过 Realtime Gateway 的内部发布接口发送合并后的 `AIStream*` 临时帧；不直接持有 WebSocket 连接或 NATS Subject。
- 将 Cervi 取消请求传入 `context`。

Eino 不负责：

- Conversation 或 Message 存储。
- 企业、会话和工具授权。
- Agent Run、审批和费用的权威状态。
- 设备选择、设备连接和本机权限。
- NATS 调度。

发起、取消和人工接管 Agent Run 都是持久命令，统一经 HTTP、`appservice` 和 Action；Realtime WebSocket 只承载流式展示和进度。

### 9.3 P1 最小接入集

P1 只使用：

- ChatModelAgent。
- Runner 和流式事件迭代。
- 服务端类型化 Tool。
- `context` 取消。
- 必要的模型用量采集。

P1 工具只允许只读或具有强业务幂等的服务端能力。

P1 默认不使用：

- 常驻 TurnLoop。
- 持久 SessionStore。
- Automemory。
- BackgroundTask Store。
- DeepAgent 或本地 Filesystem 工具。
- 默认 CheckpointStore。

### 9.4 按触发条件引入 v0.10 能力

| 能力 | 引入条件 | 业务边界 |
| --- | --- | --- |
| Checkpoint | 出现无法由 Run、Step、Tool Invocation 和审批事实重建的中间状态 | 只作可丢附件，不能替代业务状态或解决外部副作用 `uncertain` |
| TurnLoop | 同一个 Run 需要运行中 Push、新输入抢占或长期 idle 生命周期 | Conversation 的长期监听仍由消息游标和新 Run 表达 |
| SessionStore | 单个 Run 内确需框架工作集或 Middleware 回放 | 不作为 Conversation 或聊天历史 |
| Automemory | 出现明确的跨 Run 长期记忆产品 | 长期事实进入 Cervi 摘要或记忆表，不能只留在 Eino Store |
| BackgroundTask Store | 出现同一 Run 内子代理或长工具的框架级租约 | 服务端唤醒仍由 `task_runs` 承担，设备长任务由客户端工作项承担 |
| Reduction Middleware | 真实上下文或工具输出达到上限 | 业务层仍先限制输入和大结果，不依赖 Middleware 兜底 |

审批并不自动要求 Checkpoint。审批请求、参数哈希、决定和有效期必须先成为业务事实；只有审批后无法从这些事实安全重建 Runner 状态时，才增加 Checkpoint。

## 10. 审批、取消与不确定结果

### 10.1 审批事实

出现首个需要人工确认的工具时增加独立审批记录，例如：

```text
agent_tool_approvals
├── id
├── organization_id
├── invocation_id
├── arguments_hash
├── requested_from_user_id
├── status
├── decided_by_user_id
├── expires_at
├── decided_at
└── created_at
```

设备审批落地时再增加 `requested_on_device_id`，服务端审批阶段不提前创建该字段。

审批至少包含三层：

1. Action 校验组织、会话、Agent 策略和委托用户是否允许请求该能力。
2. Gateway 校验设备、能力广告、审批凭证、有效期和参数哈希。
3. Executor 校验签名调用、方法白名单、参数 Schema、本机授权范围和 OS 权限。

审批必须绑定 `invocation_id + arguments_hash`，参数变化后重新审批。跨设备执行时，审批默认出现在实际执行设备上。

### 10.2 工具调用状态

按真实能力逐步引入状态，目标语义为：

```text
proposed
  -> awaiting_approval
  -> ready
  -> dispatched
  -> running
      -> succeeded
      -> failed
      -> uncertain

cancel_requested
  -> cancelled             执行器确认未执行或已安全停止
  -> succeeded/failed      取消前已经产生确定结果
  -> uncertain             无法确认是否产生副作用
```

取消是协作式请求，不是确定事实。连接断开、超时或进程退出后，若调用已经派发且没有收到权威终态：

- 只读、纯函数或有强幂等键的调用可以按策略重试。
- 非幂等副作用进入 `uncertain`，禁止自动重放。
- 有权威查询能力时先对账；无法确认时进入人工处理。

Checkpoint 只能恢复模型执行位置，不能证明外部副作用是否发生。

## 11. Device Capability Gateway

### 11.1 形态

Device Capability Gateway 与 Realtime Gateway 是两个不同职责：

- Device Capability Gateway 是服务端业务编排模块，负责能力策略、设备选择、审批和持久调用。
- Realtime Gateway 是 `chat-roadmap.md` 定义的传输模块，负责连接认证、Protobuf 帧、Core NATS 订阅、发送队列和背压。

第一版两者都在 Cervi Server 内运行。Device Capability Gateway 不建立第二个 WebSocket 监听器，不直接管理连接，也不自行订阅 NATS。它负责：

- 解析 Agent Tool 请求为类型化 Capability。
- 计算企业策略、会话类型、Agent 策略、用户授权和设备能力的交集。
- 选择设备或返回需要用户选择。
- 创建持久设备调用和审批。
- 写入 `realtime_outbox` 请求设备工作水位通知，管理超时并通过 HTTP 接收结果。
- 把调用状态投影到 Tool Invocation 和审计链。

Gateway 不替代 Action 和 Executor 的安全校验，也不允许 Agent 直接持有连接编号。

### 11.2 设备注册与认证

设备能力落地前先增加可撤销设备注册：

```text
devices
├── id
├── organization_id
├── user_id
├── name
├── platform
├── trust_level
├── capability_manifest
├── work_seq
├── last_seen_at
├── revoked_at
└── timestamps
```

`capability_manifest` 是最近一次经过校验的设备能力广告，不是允许集；实际允许集仍为设备广告、企业策略、会话类型、Agent Tool Policy、本机授权和当前 Run 的交集。`work_seq` 是该设备持久调用的最新水位。

认证要求：

- 复用 Realtime Gateway 的 HTTP 换票和首帧认证，不创建第二套 lane ticket。
- 一次性短期连接票据继续绑定企业、用户、稳定 `device_id`、客户端种类、Origin 和过期时间；声明 Executor 能力时同时校验设备未撤销。
- 首次设备绑定由当前登录用户确认；设备信任和本机授权保存在设备记录及客户端安全存储中，不能只依赖 `ClientHello` 能力字段。
- 登出、换服、切换账号、设备撤销和用户停用必须使相关设备权限失效。
- P2 前台模式由 WebView 使用现有 Bearer Token 换取连接票据，并在 HTTP claim/result 时携带 Bearer Token 和本机稳定 `device_id`；服务端校验设备属于当前用户且未撤销，Go Executor 再校验目标设备与本机编号一致。
- 前端通过 Wails 绑定把已领取的类型化调用交给 Go Executor，再把结果经 HTTP 提交，因此 P2 不要求 Go 进程读取 `localStorage`。
- 只有 Go 进程需要脱离 WebView 独立重连或后台访问服务端时，才先将用户会话接入桌面安全凭据、iOS Keychain 或 Android Keystore；不能用稳定 `device_id` 代替认证凭据。

### 11.3 持久设备调用

设备能力首次落地时增加独立执行记录，不能只把连接状态塞进 Tool Invocation：

```text
device_invocations
├── id
├── organization_id
├── user_id
├── device_id
├── tool_invocation_id
├── work_seq
├── capability
├── arguments
├── arguments_hash
├── idempotency_key
├── status
├── available_at
├── expires_at
├── claimed_at
├── result
├── result_file_id
├── error
└── timestamps
```

`agent_tool_invocations` 表达 Agent 的工具意图和业务审计；`device_invocations` 表达该意图在一台设备上的领取、执行和结果。一次非幂等 Tool Invocation 不得同时向多台设备创建活动调用。

设备调用采用数据库事实和实时通知分离：

```text
服务端事务锁定 devices，分配 work_seq
  -> 创建 device_invocation 并写 realtime_outbox
  -> 发布用户 Subject，并携带目标 device_id 路由提示
  -> Realtime Gateway 只向匹配该 device_id 且声明 Executor 能力的连接推送 DeviceWorkAdvanced
  -> 前端通过 HTTP claim 领取完整调用
  -> 前端通过 Wails 绑定调用本机 Go Executor
  -> Executor 持久化设备侧幂等状态
  -> 前端通过 HTTP 提交 progress/result
  -> Gateway 更新 Tool Invocation
```

设备能力进入开发阶段时，在同一个 `proto/cervi/realtime/v1` 中新增可被旧客户端忽略的 `DeviceWorkAdvanced` ServerFrame，并通过 `ClientHello` 能力协商。它属于 P1 水位通知优先级，只携带设备编号和最新 `work_seq`，不携带工具名、参数或审批内容；不增加客户端持久命令帧。首版复用用户 NATS Subject，由各 Realtime Gateway 按已认证 `device_id` 过滤，不提前增加设备 Subject。

设备重连后使用现有 Realtime 认证和 Hello，再通过 HTTP 比较工作 Head、补拉或领取调用；不能依赖 Gateway 重放帧。终态设备调用按保留策略清理，长期审计仍由 Agent Tool Invocation 保存。

### 11.4 客户端 Executor

桌面端首期 Executor 只在应用进程存活且用户在线时承诺执行。首批能力限制为：

- 文件选择并上传为 Cervi `file_id`。
- 授权根目录内的文件元数据读取。
- 只读 `git status`、`git diff` 等明确能力。

不在首批开放通用 Shell、删除、任意路径读取、通讯录全量读取或相册全量扫描。

长命令、跨进程恢复或客户端已经执行但尚未上报的场景出现后，再以该能力驱动 `client_work_items` 和设备侧幂等账本。客户端不嵌入 NATS。

现有文件上传只支持头像用途。设备工具需要返回文件引用前，先扩展类型化文件用途、授权和激活关系，不能绕过文件模块直接写存储。

### 11.5 桌面端与移动端差异

桌面端：

- 适合前台文件、Git 和授权目录能力。
- 应用退出后不承诺继续执行。
- 长任务需要 SQLite 工作项和明确恢复语义。

移动端：

- 默认只承担文件/相册选择、OS 权限确认和前台审批。
- iOS 挂起、Android Doze 和厂商进程限制下，不假设常驻 WSS。
- 系统推送只负责提示用户打开应用，不保证无人值守执行。
- WorkManager、BGTaskScheduler 和系统传输能力只在真实后台场景出现后接入。

## 12. MCP 决策

P2 不直接采用 MCP subset 作为设备主协议，优先使用 Cervi 类型化的 HTTP invocation、claim、progress、result 和 cancel 契约；实时提示只扩展现有 Protobuf `ServerFrame`。

原因：

- MCP 不表达 Cervi 的企业、会话、Agent Run、设备寻址、审批、幂等和 `uncertain`。
- 只有 `tools/list`、`tools/call`、progress 和 cancel 不是完整 MCP Profile。
- 完整 MCP 还需要初始化、协议版本和能力协商；取消也只是尽力请求，不能作为副作用未发生的证明。
- 自定义 WSS Transport 加不完整 MCP Profile，会同时承担自有协议和 MCP 兼容成本。

出现第三方本地 MCP 工具生态需求后，在 Executor 后增加完整 MCP Adapter：

```text
Cervi Gateway
  -> Cervi Device Invocation
  -> Executor
      -> 内置类型化能力
      -> MCP Adapter
          -> 本地 MCP Server
```

届时要求：

- 精确锁定 MCP 协议版本。
- 实现完整生命周期和能力协商。
- `tools/list` 只表示设备广告，不表示企业授权。
- Gateway 仍负责策略、审批、设备选择、业务审计和 `uncertain`。
- 禁止用 MCP resources、prompts、sampling 或 session 表达聊天、文件和企业事实。

## 13. 替代架构及取舍

### 13.1 客户端直接运行完整 Agent

优点是本地隐私、低延迟和离线能力更强。现阶段缺点是会复制模型凭据、运行状态、审计、恢复和移动端后台机制，因此暂不采用。只在 P4 出现明确需求后引入，并继续回写统一 Agent Run 事实。

### 13.2 设备直接暴露为端到端 MCP Server

优点是生态兼容。缺点是 MCP 不能替代 Cervi 的设备注册、授权、审批、幂等和不确定结果状态，最终仍需要 Gateway。现阶段不采用，未来作为 Executor 内部适配器。

### 13.3 Agent Worker 直接调用 WSS 连接

组件最少，但会把设备寻址、授权、审批、连接状态和断线恢复散入 Eino Runtime，并使 Agent 持有瞬时连接。否决。

### 13.4 持久调用 + WSS 唤醒

相比同步 WSS RPC 多一次持久化和领取请求，但能自然处理断线、重连、多实例、审计和 `uncertain`，并直接复用现有 Realtime Gateway、`realtime_outbox`、Core NATS、Protobuf 连接与背压。采用为首选方案。

## 14. 实施阶段

本节的 P0 至 P4 是 Agent 建设子阶段，不等同于 `chat-roadmap.md` 的聊天阶段编号。Agent P0 表示进入首个 Agent Runtime 前必须完成的聊天和任务前置能力。

### P0：聊天和任务前置能力

- 完成 `chat_subjects`、Conversation、Participant 和 Message 事实。
- 落地消息事务、幂等、引用、@ 和双时间顺序。
- 增加 `TxEnqueuer.EnqueueIn`。
- 按 `chat-roadmap.md` 落地 Conversation Changelog、用户 Mailbox/共享 Inbox、`realtime_outbox`、Core NATS、Realtime Gateway、Protobuf 协议、连接票据和 HTTP 恢复。

验收边界：聊天事实和断线补拉成立，但不创建 Agent 运行表，不接入 Eino。

### P1：纯服务端、只读或强幂等 Agent

- 增加不可变 Agent Revision 和当前版本切换。
- 增加 Policy、State、desired/processed cursor、Run、语义 Step 和 Tool Invocation。
- 为 Agent 使用独立任务队列或 Worker 配额。
- 接入 Agent Engine 和精确锁定版本的 Eino。
- 只使用 ChatModelAgent、Runner、服务端类型化 Tool、流式事件和取消。
- 合并后的临时模型增量复用 `AIStream*` Protobuf 帧；发起、取消、最终消息和 Run 状态继续走 HTTP 与数据库事实。
- 支持自动响应、@ 触发、费用和失败审计。
- 工具只读或具有强业务幂等。

验收边界：Agent 能以统一参与者身份可靠说话；崩溃或任务重复不会产生重复输出；运行期间的新消息不会丢失。

### P1.5：服务端副作用、审批和恢复

- 增加 Approval 事实和 Tool Invocation 状态机。
- 绑定 `invocation_id + arguments_hash`，实现审批过期和防重放。
- 先以服务端可变更工具验证取消、重试、`uncertain` 和人工接管。
- 只有无法从业务事实安全重建中间状态时才接入 CheckpointStore。

验收边界：副作用得到确定结果或进入明确的 `uncertain/needs_review`，不会因 Task 重试被盲目重复执行。

### P2：桌面端设备能力

- 增加设备注册、撤销和 Capability Manifest；复用现有稳定 `device_id` 和 Realtime 一次性连接票据，不增加 lane ticket。
- 在服务端单体内实现 Device Capability Gateway，并与 Realtime Gateway 保持业务编排和传输职责分离。
- 增加 `device_invocations`、设备 `work_seq`、HTTP claim/progress/result，以及同一 Realtime Protobuf 连接上的 `DeviceWorkAdvanced` 水位通知。
- 桌面前端负责 Realtime 与 HTTP，通过 Wails 绑定调用 Go Executor；Executor 实现本机二次校验和设备侧幂等。
- 首批开放文件选择上传、授权根元数据、只读 Git 状态能力。
- 客户会话继续默认禁用设备工具。

验收边界：WSS 丢帧或重连不会丢调用；非幂等调用结果未知时不会自动重放；设备撤销后不能继续领取调用。

### P3：设备能力扩展

- 按真实场景增加 `client_work_items`、长任务恢复和结果对账。
- 支持用户明确选择多设备，不自动广播副作用。
- 扩展类型化文件用途和大结果文件引用。
- 移动端支持前台审批、文件和相册选择，不承诺无人值守执行。
- 出现第三方工具生态需求后增加完整 MCP Adapter。

### P4：本地 Agent Runtime

只有以下需求至少出现一项时进入：

- 企业要求模型和上下文不离开设备。
- 无网络时仍需完成完整 Agent 循环。
- 本地编码场景需要高频、低延迟工具往返。
- 需要本地模型或长时间操作本地工作区。

进入 P4 后：

- 复用同一 Agent 身份、Revision、Run、Step 和 Tool Invocation 语义。
- 给 `agent_runs` 增加真实需要的运行位置，不创建第二套 Run 表。
- 客户端本地数据库只保存执行和离线同步所需状态；联网后回写服务端业务事实。
- 根据真实循环需求再引入 TurnLoop、Filesystem、DeepAgent 或 BackgroundTask。
- 本地模型凭据、沙箱、工作区授权和离线冲突另行设计，不由 WSS 方案隐式继承。

## 15. 暂缓决策与触发条件

| 暂缓能力 | 触发条件 |
| --- | --- |
| Eino 最终版本 | 开始 P1 实施时根据正式发布状态选择并精确锁定 |
| CheckpointStore | 业务事实无法安全重建 Runner 中间状态 |
| TurnLoop | 同一 Run 需要 Push、抢占或长期 idle |
| SessionStore | 单 Run 内存在明确的框架工作集或 Middleware 回放需求 |
| Automemory | 产品定义了跨 Run 的长期记忆及用户可管理语义 |
| BackgroundTask Store | 同 Run 子代理或长工具需要框架级租约 |
| `agent_run_events` | 必须跨断线逐事件回放，且日志系统不能满足 |
| 完整 MCP Adapter | 出现第三方本地 MCP 工具生态需求 |
| 客户端可靠任务 | 首个设备任务必须跨进程恢复 |
| 独立 Device Capability Gateway 服务 | 设备业务编排需要独立扩缩容；连接扩缩容继续由 Realtime Gateway 负责 |
| 多设备自动选择 | 产品已经定义可解释且安全的选择规则 |
| 移动端后台执行 | 存在系统允许且用户明确需要的真实后台任务 |
| Run 运行位置 | 整段 Agent 循环首次在设备运行 |

## 16. 评审检查表

进入每个 Agent 实施 PR 前检查：

- 当前修改属于聊天事实、Agent 业务事实还是任务基础设施，是否发生混用。
- 是否显式校验 `organization_id`、会话、Agent、Revision 和工具策略。
- 是否可能因 Task 至少一次执行产生重复模型输出或重复副作用。
- 是否把流式事件、日志或 Eino 内部状态误写成永久 Step。
- 是否能区分确定成功、确定失败、取消请求和结果未知。
- 是否把大内容改成文件引用并设置大小上限。
- 是否把客户消息等不可信输入暴露给设备能力。
- 是否依赖 WSS 帧、内存连接、移动后台或进程常驻维持正确性。
- 是否绕过统一 Realtime Gateway、连接票据和 Protobuf Schema 建设第二套设备实时协议。
- 是否提前创建没有真实场景的表、字段、运行时或协议。
- 是否能在不改变聊天身份和业务事实的前提下替换或升级 Eino。
