# 群内 Agent 协作方案

本文定义群聊中 Agent 参与协作的长期模型、落地顺序和扩展方式。目标是让每一步交付都建立在最终形态的契约上，后续能力通过增加实现获得。

## 1. 设计出发点

现有代码已经具备完整的单 Agent 执行骨架：

| 现有对象 | 承担的语义 |
| --- | --- |
| `conversation_agent_states` | 一个「会话 + Agent」的输入水位，`desired_seq` 与 `processed_seq` 保证不丢唤醒 |
| `conversation_agent_triggers` | 类型化的持久输入事实，带来源消息和连续序号 |
| `agent_runs` | 消费一段连续输入的一次业务执行，产出一条最终消息 |
| `agentRunPolicy` | 执行范围的可替换实现：锁上下文、资格门禁、上下文装配、消息写回、续跑 |
| `TurnLoop` + `InputFeed` | 运行期在安全点认领新输入，扩展本次运行的输入边界 |

独立 AI 聊天和网站 AI 客服是同一套骨架的两个 Policy 实现。群协作是第三个实现。

群聊引入三个新问题：

1. **范围键不够用**：`conversation_agent_triggers` 与 `agent_runs` 靠 `trigger_type + service_session_id` 区分客服周期，查询里带着 `service_session_id IS NOT DISTINCT FROM ?`；群聊会继续增加取值。
2. **上下文里有多个说话人**：现有上下文把消息投影为 `user` / `assistant`，丢掉发送者身份。
3. **Agent 可以把工作交给另一个 Agent**：这是唯一需要新语义的能力。

其余产品行为——多个 Agent 依次回答、澄清后继续、停止、失败——由现有骨架加一条轮转规则表达。

## 2. 两个正交的概念

方案中「Lane」和「执行互斥」是两件事，全文按以下定义使用：

| 概念 | 含义 | 粒度 |
| --- | --- | --- |
| Lane | 某个 Agent 在某个执行范围内的输入队列与水位 | 每 Agent 一条 |
| 执行互斥 | 同时允许存在的活动运行 | 每执行范围一个 |

一个执行范围内可以有多条 Lane，它们共享一个活动运行名额。群里 A 正在运行时，发给 B 的输入进入 B 的 Lane 等待轮转，与 A 的输入互不混合。

执行互斥的粒度跟随执行范围演进：当前范围是会话，群内 Agent 因此串行发言；话题上线后范围收细为话题，话题之间并行（第 10 节）。

由此获得的确定性：群内 Agent 的发言顺序完全确定，后发言者必然读到前面全部回复；模型调用并发为一；客户端同时只需展示一个运行状态。代价是队头阻塞——一个 Agent 长时间调用工具时，同范围其他 Agent 排队等待。现有单次运行时限与任务最终失败处理保证终态必达。

## 3. 核心模型

### 3.1 Lane

用 `agent_lanes` 取代 `conversation_agent_states`：

```text
agent_lanes
├── id
├── organization_id
├── conversation_id
├── agent_identity_id
├── scope_kind          -- conversation | service_session
├── scope_id
├── desired_seq
└── processed_seq

UNIQUE (organization_id, agent_identity_id, scope_kind, scope_id)
```

| scope_kind | scope_id | 覆盖的场景 |
| --- | --- | --- |
| `conversation` | 会话编号 | 独立 AI 聊天、群聊 |
| `service_session` | 客服周期编号 | 网站与渠道 AI 客服 |

`conversation_id` 是权威列：客服周期需要它定位写回目标，Action 在事务中校验 scope 归属该会话。

客服由此从「共享会话水位 + 周期过滤」变为「每周期一条 Lane」。转交与关闭本来就把原负责人的水位一次推到 `desired_seq`，因此产品行为等价，差别是输入序号按周期重新编号，`agent_runs` 的输入边界随之重排。

群聊使用 `conversation` 范围，一个群里一个 Agent 一条 Lane。两个人同时在群里 `@` 同一个 Agent 时，两条消息进入同一条 Lane，被同一次运行一起读到，得到一条综合回复。

### 3.2 Input

用 `agent_inputs` 取代 `conversation_agent_triggers`：

```text
agent_inputs
├── id
├── organization_id
├── lane_id
├── input_seq              -- Lane 内连续
├── kind                   -- mention | agent_direct | customer_auto | handoff
├── source_message_id
├── source_ordinal         -- 同一条来源消息内的目标位置
├── source_subject_id      -- 造成本次输入的聊天主体
├── depth                  -- 真人发起为 0，Agent 接力为来源 depth + 1
├── agent_run_id
└── created_at

UNIQUE (lane_id, input_seq)
```

`kind` 记录输入的实际入口，用于归因与展示；`depth` 是 Agent 接力的防护机制，第 6 节说明。

跨 Lane 的发言顺序由 `(来源消息的 message_seq, source_ordinal)` 确定。`message_seq` 是会话内消息顺序的权威来源；`source_ordinal` 表达同一条消息内多个目标的先后，取值来自发送时的 `@` 顺序。输入序号与主键不参与跨 Lane 比较。

### 3.3 Run 与执行互斥

`agent_runs` 调整：`lane_id` 取代范围元组，`trigger_start_seq / trigger_end_seq` 更名为 `input_start_seq / input_end_seq`，删除 `trigger_type` 与 `service_session_id`，增加与所属 Lane 一致的 `scope_kind` 与 `scope_id`。

活动运行唯一索引表达执行互斥：

```sql
CREATE UNIQUE INDEX agent_runs_active_scope_unique
    ON agent_runs (organization_id, scope_kind, scope_id)
    WHERE status IN ('queued', 'running');
```

`agent_runs` 上的 scope 列与所属 Lane 一致，由建立运行的 Action 在事务中写入。Postgres 无法跨表表达部分唯一约束，这两列的存在即为表达本条互斥。

现有索引 `(conversation_id, agent_identity_id)` 的粒度是「会话 + Agent」。收窄到 scope 后，独立 AI 聊天与客服的产品行为不变，因为两者的执行范围内本来只有一个 Agent；客服转交与关闭依赖 `CancelForServiceSession` 在同一事务取消原负责人运行，PR-A 需要覆盖转交、认领与关闭三条路径。

## 4. 执行轮转

### 4.1 建立运行的入口

**发送事务**：写入输入后，若本执行范围没有在途运行，为本次写入的输入中 `source_ordinal` 最小的目标建立运行。发送路径已持有会话行锁。

**运行终态事务**（`complete`、`fail`、`StopAgentReply`）：

1. 本 Lane 还有未处理输入，为本 Lane 建立下一次运行。
2. 否则在本执行范围内，按第 3.2 节的顺序取最早一条未处理输入所属的 Lane。
3. 该 Lane 的 Agent 已退群、被停用或没有有效 Revision 时跳过，继续取下一条；跳过不影响本次终态事务提交。
4. 没有合格 Lane 则本执行范围进入空闲。

轮转是三条链路共用的实现。独立聊天与客服的执行范围内只有一条 Lane，规则退化为现有的续跑行为。Policy 只提供资格判定与 Revision 解析。

### 4.2 不参与轮转的路径

`CancelForServiceSession` 把负责人水位一次推到 `desired_seq`，由转交、认领或关闭事务决定是否为新负责人建立输入。它不执行轮转，否则会与转交事务争抢活动运行名额。

消息幂等重放不写入输入事实，不推进水位，不触发轮转。

### 4.3 锁序

群 Policy 的 `lockContext` 与发送路径遵循同一顺序：Conversation（群使用 `LockGroup`）→ Lane → Run → Task 租约。轮转在同一事务内选择下一条 Lane，因此终态事务与发送事务对同一执行范围串行化，「无在途运行才建立运行」的判断与唯一索引不产生竞态。同时锁多条 Lane 时按稳定编号排序。

## 5. 群内的触发与上下文

### 5.1 触发规则

在 `SendGroupTextMessageAction` 的同一事务内判定并写入输入事实：

| 消息形态 | 行为 |
| --- | --- |
| 显式 `@某个 Agent` | 为该 Agent 的 Lane 写入 `mention` 输入 |
| 引用回复某个 Agent 的消息 | 等同于 `@` 该 Agent |
| `@所有人` | 只产生群提醒，不触发任何 Agent |
| 普通消息 | 不触发，进入后续运行的上下文 |
| Agent 发出的消息 | 不触发，接力由第 6 节的显式结果表达 |
| 系统消息、历史补拉、幂等重放 | 不触发 |

一条消息 `@` 多个 Agent 时，为每个目标各写一条输入，按 `source_ordinal` 依次执行，各自产出独立回复。

现有 `loadGroupMentionTargets` 限制提醒目标为真人身份，本方案放开为群内有效参与者，Agent 目标额外写入输入事实，真人目标仍只写提醒关系。提醒目标不得是发送者本人的既有规则保持不变。

### 5.2 引用的语义边界

引用决定触发谁，不决定上下文范围。

被引用消息是 Agent 的哪一条都不改变执行行为：上下文仍是截至本次认领边界的最近若干条群消息，被引用的具体消息由现有的结构化正文封装传给模型。引用一条较早的消息时，模型同时看到该消息、其后的消息和当前提问，据此判断提问指向。被引用消息超出上下文窗口时，封装中保留的原文摘要仍然可用。

`loadConversationReplyTarget` 已将可引用目标限定为文本与附件消息，`agent_error` 与 `agent_cancelled` 不在其列，本方案沿用。

配套规则：

- 同一条消息的引用目标与 `@` 目标去重，一个 Agent 只产生一条输入，`source_ordinal` 取该 Agent 的首次出现位置。
- 一条消息引用 A 的消息并同时 `@` B 时，A 与 B 各产生一条输入，按 `source_ordinal` 依次执行。
- 引用一条较早的消息表示追问，独立上下文由第 10 节的话题对象表达。

### 5.3 上下文

`agentruntime.Message` 增加发送者标识，群 Policy 的 `loadMessages` 填充。自己的发言仍投影为 `assistant`，其余成员（真人与其他 Agent）投影为带发送者标识的 `user`。发送者与引用沿用现有的结构化正文封装方式，保持一层引用不占用新的对话角色。

上下文窗口沿用现有的「不越过已认领输入边界的最近 100 条文本消息」。串行执行保证后发言的 Agent 读到本轮前面全部回复。

群聊闲聊会进入这个窗口，本阶段依靠发送者标识和明确的 `@` 请求让模型区分相关性。窗口噪音的最终解法是把执行范围收敛到话题（第 10 节）。

## 6. 回合结果与接力

### 6.1 结果协议

`RunResult` 增加两个字段：

```text
RunResult
├── outcome    -- reply | silent
├── content
├── mentions   -- 本次回复指向的企业身份
├── endSeq
├── usage
└── blocks
```

`outcome` 与 `mentions` 由模型通过类型化结束工具提交。结束工具只注册给群 Policy 的运行；独立聊天与客服继续使用「最后一条不带工具的正文即最终回复」的现有协议。服务端不解析正文中的 `@` 文本。

`silent` 表示本次运行成功消费输入但没有公开发言，用于群里被顺带 `@` 到、无需回应的情况。运行推进 `processed_seq` 并触发轮转，不写入消息，不携带 `mentions`。现有运行时与完成事务都拒绝空正文，`silent` 需要同时放开这两处门禁。

`mentions` 的目标必须是当前群内的有效参与者，且不得是运行自身的 Agent。服务端按目标类型分流：

- 目标是真人：写入提醒关系，产生通知。
- 目标是 Agent：写入提醒关系，同时向该 Agent 的 Lane 追加一条 `handoff` 输入，由轮转规则安排执行。

接力挂在本次公开回复上，`handoff` 输入的 `source_message_id` 即该回复消息。

### 6.2 循环防护

`handoff` 输入的 `depth` 取来源运行所消费输入的最大 `depth` 加一，超过上限即拒绝该次接力。真人的任何新消息 `depth` 为 0，链条重置。

被拒绝的接力不写入提醒关系，也不写入输入，因此群里不会出现一个永远不会执行的 `@`。运行内容块记录被拒绝的目标供排查。

防护无需持久计数器，也无需在控制操作时重置，对并发链条同样成立。

## 7. 澄清、停止与失败

**澄清**：Agent 需要补充信息时，在群里把问题作为普通回复发出，运行正常结束并释放执行范围。真人回复这条消息，按第 5.1 节的引用规则产生新输入。持久化的等待对象只有工具审批需要，属于 `agent-roadmap.md` 的 P1.5。

**停止**：群内有效成员可以停止本群在途的 Agent 运行。停止边界包含已提交但尚未认领的输入——停止 A 即放弃 A 尚未认领的后续 `@`，水位推到 `desired_seq`，写入 `agent_cancelled` 消息，并按第 4.1 节安排下一个执行者。这与现有单聊停止语义一致。

**失败**：沿用现有 `fail` 路径，在群内写入 `agent_error` 消息、结算输入并推进轮转。单个 Agent 失败不阻断本轮其余 Agent。

## 8. 数据结构与迁移

| 迁移 | 内容 |
| --- | --- |
| 建 `agent_lanes` | 取代 `conversation_agent_states` |
| 建 `agent_inputs` | 取代 `conversation_agent_triggers` |
| 改 `agent_runs` | 增加 `lane_id`、`scope_kind`、`scope_id`、`outcome`，更名输入边界列，删除 `trigger_type`、`service_session_id`，重建活动运行唯一索引 |
| 删除旧表 | 直接删除，不做数据回填 |

按仓库约定：每个建表迁移一个文件、只建一张表、不建外键与 `CHECK`，索引只保留主键和表达业务约束的唯一索引，时间戳由 `wails3 task make:migration` 生成，字段使用中文 `COMMENT ON`。本地开发库通过 `migrate:reset` 重建。

## 9. 代码落点

| 位置 | 改动 |
| --- | --- |
| `internal/actions/agentrun` | `agentRunScope` 收敛为 `lane_id`；轮转选择作为三条链路共用实现；新增 `groupMentionRunPolicy`；停止路径改为按 Lane 定位并接入轮转 |
| `internal/actions/conversation` | 群发送事务内判定 `@Agent` 与引用回复并写入输入，按互斥状态决定是否立即建立运行；提醒目标放开为群内有效参与者 |
| `internal/integration/agentruntime` | `Message` 增加发送者标识；`RunResult` 增加 `outcome` 与 `mentions`；为群 Policy 注册类型化结束工具；放开空正文门禁 |
| `internal/appservice` | 新增群内停止运行的 Backend 方法与路由，授权为群内有效成员；新增会话级 Agent 运行摘要，返回当前运行与排队中的 Agent |
| `frontend` | 群输入区 `@` 候选包含 Agent；按会话级摘要展示当前运行状态、排队 Agent 与停止入口 |

排队信息来自各 Lane 的 `desired_seq > processed_seq`，由服务端汇总为会话级只读摘要，页面不自行拼装水位。

后端用户可见文案统一走 `internal/i18n`，复用已有语义键。

## 10. 后续能力如何长出来

| 能力 | 增加什么 | 不改什么 |
| --- | --- | --- |
| 话题 / 结构化协作（目标、参与范围、暂停继续、结论） | `conversation_topics` 表；Lane 与 Run 的 `scope_kind` 增加 `topic` 取值 | Lane、Input、Run 的结构与执行流程，活动运行唯一索引 |
| 话题之间并行发言 | 群 Policy 的 `loadMessages` 按话题收敛上下文，与上一行的 scope 取值一并生效 | 轮转规则本身，上下文契约 |
| 主持 / 编排式讨论 | 替换轮转的选择规则 | 结果协议与输入模型 |
| 工具审批与外部副作用 | `agent-roadmap.md` P1.5 的审批事实与调用幂等 | 群协作的触发与写回 |
| 定时或外部事件触发 | 一个写入 `agent_inputs` 的新入口，`kind` 增加取值 | 执行链路 |

`scope_kind` 是本模型的扩展点：执行范围从隐含的列组合变成显式的、可增加取值的维度，Lane 归属与执行互斥的粒度都随它收细。

并行与上下文收敛是同一件事的两面：话题之间并行的前提是各自的执行读取按话题收敛的历史。上下文仍覆盖整个群时，两个话题的执行读写同一段历史，重新回到非确定的交叉结果。

`agent-roadmap.md` 的不变量 5 与第 6.3 节当前把互斥写在「会话 + Agent」上。这条表述要防的是两个执行读到彼此未完成的状态、产出互不承认的回复，约束单位是一次执行读取历史与写回结果的边界。独立 AI 聊天一个会话一个 Agent，客服一个会话同时只有一个开放周期，因此现有表述与代码行为一致；客服的上下文实际已按处理周期隔离。

PR-A 收窄互斥粒度时同步改写这两处：不变量表述为「同一执行范围内最多一个在途 Run，运行期间到达的新输入不得丢失，执行范围由 `scope_kind` 与 `scope_id` 表达」，并补充扩展条件——引入新的执行范围取值时，不同范围的执行所读取的历史与写回的结果必须互不重叠。话题按这条条件检验，其并行资格来自上下文收敛。

## 11. 交付顺序

| PR | 范围 | 验收 |
| --- | --- | --- |
| A | Lane / Input 重构与执行互斥索引收窄，同步改写 `agent-roadmap.md` 不变量 5 与第 6.3 节，三条已上线链路行为不变 | 现有服务端测试全部通过；客服转交、认领、关闭三条路径不产生重叠运行；输入序号按客服周期重新编号；无新增产品行为 |
| B | 群内单 Agent：`@` 与引用触发、带发送者的上下文、群内回复、失败、停止 | 群内 `@` 得到回复；运行期间到达的新消息在安全点补入同一次运行；非 `@` 消息不触发；`@所有人` 不触发；幂等重放不重复触发 |
| C | 群内多 Agent 轮转：一条消息 `@` 多个 Agent | 按 `@` 顺序依次发言，后发言者读到前面全部回复；同一时刻只有一个运行；失败、停止或目标失去资格后轮转继续 |
| D | 类型化结束工具：`outcome`、`silent`、`mentions`、`handoff` 输入、`depth` 上限 | `A → B → A` 接力保留来源；超过上限的接力被拒绝且不留下 `@`；真人消息重置链条；`silent` 不写消息但推进轮转 |
| E | 话题 / 结构化协作 | 按真实需求启动 |

A 是纯重构，改动集中在 `internal/actions/agentrun`、三个存储模型和四处调度调用方，测试为机械替换，清单见附录 A。B 与 C 可以合并交付。`silent` 依赖结束工具，与接力一起在 D 落地，B 阶段被 `@` 的 Agent 一定回复。

`internal/integrationtest/agent_group_integration_test.go` 现在断言群内 `@Agent` 返回 `GroupMentionTargetInvalid` 且不产生 Agent 记录，这是当前契约。B 需要改写这条用例。

## 12. 取舍说明

**不建立协作对象。** 跨 Agent 的工作单元拆开后只剩「下一个谁说话」，由运行终态事务里的一次选择表达。承载目标、参与范围和生命周期的对象是话题，属于聊天层，真人单独也能使用，见第 10 节。

**不建立批次与汇合。** 串行执行下发言顺序确定，每个运行的上下文边界由它认领输入时的消息序号决定，一致性无需冻结快照。

**串行的代价是队头阻塞。** 收益是确定的发言顺序、天然为一的模型并发和单一运行状态的客户端。缓解方式是把执行范围收细为话题，而非放开互斥。

## 13. 本方案明确不做

- 不引入协作对象、批次、汇合策略、执行名额和自动运行预算。
- 不引入计算检查点与跨进程恢复；澄清用聊天表达，审批留在 P1.5。
- 不引入策略快照版本、控制版本代次和成员代次；资格在事务中按当前事实校验。
- 不引入展示裁剪策略层；群成员读取公开消息与现有运行过程，权限统一在角色体系建设时处理。
- 不为群聊单独建立实时协议；沿用现有轮询，统一实时按 `chat-roadmap.md` 阶段 2E 交付。

## 附录 A：PR-A 实施清单

PR-A 只做模型收敛，不引入群聊行为。完成后三条已上线链路的产品行为不变，输入序号按客服周期重新编号。

### A.1 迁移

四个文件，时间戳由 `wails3 task make:migration` 生成。

**1. `create_agent_lanes_table`**

```sql
CREATE TABLE agent_lanes (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    organization_id    uuid NOT NULL,
    conversation_id    uuid NOT NULL,
    agent_identity_id  uuid NOT NULL,
    scope_kind         text NOT NULL,
    scope_id           uuid NOT NULL,
    desired_seq        bigint NOT NULL DEFAULT 0,
    processed_seq      bigint NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX agent_lanes_scope_agent_unique
    ON agent_lanes (organization_id, scope_kind, scope_id, agent_identity_id);
```

**2. `create_agent_inputs_table`**

```sql
CREATE TABLE agent_inputs (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at         timestamptz NOT NULL DEFAULT now(),
    organization_id    uuid NOT NULL,
    lane_id            uuid NOT NULL,
    input_seq          bigint NOT NULL,
    kind               text NOT NULL,
    source_message_id  uuid NOT NULL,
    source_subject_id  uuid NOT NULL,
    source_ordinal     integer NOT NULL DEFAULT 0,
    depth              integer NOT NULL DEFAULT 0,
    agent_run_id       uuid
);

CREATE UNIQUE INDEX agent_inputs_lane_seq_unique
    ON agent_inputs (lane_id, input_seq);
```

按目标结构一次建全。PR-A 写入 `kind`、`source_message_id` 与 `source_subject_id`；`source_ordinal` 与 `depth` 保持默认值，分别由 PR-C 与 PR-D 开始写入。

**3. `switch_agent_runs_to_execution_scope`**

先清空 `agent_runs` 与 `agent_run_blocks`，再变更结构。运行记录是执行事实，不做回填；本地开发库中已有 AI 回复的过程展开随之清空，消息本身保留。

```sql
TRUNCATE agent_run_blocks, agent_runs;

ALTER TABLE agent_runs
    ADD COLUMN lane_id uuid NOT NULL,
    ADD COLUMN scope_kind text NOT NULL,
    ADD COLUMN scope_id uuid NOT NULL,
    ADD COLUMN outcome text,
    DROP COLUMN trigger_type,
    DROP COLUMN service_session_id;

ALTER TABLE agent_runs RENAME COLUMN trigger_start_seq TO input_start_seq;
ALTER TABLE agent_runs RENAME COLUMN trigger_end_seq TO input_end_seq;

DROP INDEX agent_runs_conversation_active_unique;

CREATE UNIQUE INDEX agent_runs_active_scope_unique
    ON agent_runs (organization_id, scope_kind, scope_id)
    WHERE status IN ('queued', 'running');
```

`conversation_id` 与 `agent_identity_id` 保留，收件箱与运行过程查询不受影响。

**4. `drop_conversation_agent_tables`**

删除 `conversation_agent_triggers` 与 `conversation_agent_states`。Down 迁移恢复两张表的结构，不恢复数据。

### A.2 领域值

| 文件 | 改动 |
| --- | --- |
| `internal/domain/agent_run.go` | `AgentTriggerType` 更名为 `AgentInputKind`，取值保留 `agent_direct` 与 `customer_auto`；新增 `AgentExecutionScopeKind`，取值 `conversation` 与 `service_session` |

### A.3 存储模型

| 文件 | 改动 |
| --- | --- |
| `models/conversation_agent_state.go` | 删除，由 `models/agent_lane.go` 取代 |
| `models/conversation_agent_trigger.go` | 删除，由 `models/agent_input.go` 取代 |
| `models/agent_run.go` | 增加 `LaneID`、`ScopeKind`、`ScopeID`、`Outcome`；`TriggerStartSeq` / `TriggerEndSeq` 更名为 `InputStartSeq` / `InputEndSeq`；删除 `TriggerType` 与 `ServiceSessionID` |

新模型别名沿用现有风格：`agent_lanes AS al`、`agent_inputs AS ai`。

### A.4 执行链路

| 文件 | 改动 |
| --- | --- |
| `agentrun/input_feed.go` | 删除 `agentRunScope`、`agentRunScopeFor`、`validateAgentRunScope` 与 `applySelect`；`Peek` 与 `Claim` 按 `lane_id` 过滤；`assignAgentTriggers` 改为按 `lane_id` 认领输入；`lockAgentRun` 按 `run.LaneID` 锁 Lane |
| `agentrun/schedule.go` | `advanceAgentSequence` 改为按 scope 建立或锁定 Lane 并分配序号，返回 Lane 编号与水位；`agentRunSpec` 携带 scope 与来源主体；`insertAndEnqueueRun` 写入 `lane_id`、`scope_kind`、`scope_id` |
| `agentrun/customer_schedule.go` | 输入范围改为 `service_session` scope |
| `agentrun/execute.go` | `policyForRun` 依据 `scope_kind` 选择实现；完成与失败事务改用输入边界新列名 |
| `agentrun/cancellation.go` | `cancelServiceSessionRuns` 按 `service_session` scope 定位 Lane |
| `agentrun/stop_agent_reply.go` | 按 Lane 定位运行与水位 |
| `agentrun/agent_chat_execute.go`、`agentrun/customer_execute.go` | `enqueueNext` 的运行规格携带 scope |

`policyForRun` 在 PR-A 只按 `scope_kind` 分支。PR-B 引入群聊后，`conversation` scope 需要再按会话类型区分 AI 聊天与群聊，会话类型由 `begin` 已有的查询一并读出。

跨 Lane 的轮转选择在 PR-B 落地。PR-A 的每个执行范围内只有一条 Lane，续跑行为与现有 `enqueueNext` 一致。

### A.5 调用方

以下四处需要向调度入口传入发送者主体编号，用于填充 `source_subject_id`：

| 文件 | 入口 |
| --- | --- |
| `actions/conversation/internal_text_message.go` | `Scheduler.Schedule` |
| `actions/conversation/receive_website_customer_text_message.go` | `ScheduleCustomerAuto` |
| `actions/conversation/manage_service_session.go` | `ScheduleCustomerAuto` |
| `actions/channel/receive_telegram_webhook.go` | `ScheduleCustomerAuto` |

`actions/conversation/agent_process.go` 与 `actions/inbox/load_inbox.go` 只使用 `agent_runs` 的保留列，无需改动。

### A.6 测试

现有断言引用旧表名与旧列名，分布在 11 个文件共 26 处，改动为机械替换：

```text
internal/integrationtest/agent_chat_lock_integration_test.go
internal/integrationtest/agent_conversation_integration_test.go
internal/integrationtest/agent_customer_reply_integration_test.go
internal/integrationtest/agent_direct_reply_integration_test.go
internal/integrationtest/agent_error_message_integration_test.go
internal/integrationtest/agent_group_integration_test.go
internal/integrationtest/agent_stop_reply_integration_test.go
internal/integrationtest/agent_telegram_integration_test.go
internal/integrationtest/chat_message_append_integration_test.go
internal/integrationtest/customer_lock_integration_test.go
internal/integrationtest/server_actions_integration_test.go
internal/task/server/agent_execution_integration_test.go
```

新增用例：

- 客服转交、认领与关闭三条路径各自不产生重叠的活动运行。
- 同一会话内先后两个客服周期，第二个周期的输入序号从 1 开始，且第一个周期的水位已结算。
- 同一执行范围内并发建立运行时，唯一索引拒绝第二次插入。

### A.7 路线图同步

改写 `agent-roadmap.md` 不变量 5 与第 6.3 节，表述改为「同一执行范围内最多一个在途 Run，运行期间到达的新输入不得丢失，执行范围由 `scope_kind` 与 `scope_id` 表达」，并补充扩展条件：引入新的执行范围取值时，不同范围的执行所读取的历史与写回的结果必须互不重叠。第 6.1 节的 `conversation_agent_states` 与第 6.2 节的 `conversation_agent_triggers` 结构说明同步替换为 Lane 与 Input。

### A.8 验收

- `wails3 task test:server` 全量通过。
- 独立 AI 聊天、网站 AI 客服与 Telegram AI 客服的收发、抢占、停止与转交行为不变。
- 数据库中不再存在 `conversation_agent_states` 与 `conversation_agent_triggers`。
- 代码中不再出现 `service_session_id IS NOT DISTINCT FROM`。
- 无新增前端改动，无新增 appservice 方法。
