# AI 员工客服同构与自动履职方案

## 1. 文档状态

- 状态：提案
- 目标阶段：Agent P1b
- 交付范围：网站客户会话中的 AI 员工自动履职
- 前置能力：Agent Direct 运行时、通用客户会话负责人工作流、网站访客轮询

## 2. 背景与原则

Cervi 将 AI Agent 视为企业中的真实员工，而不是附着在客服会话上的特殊功能。AI 员工和真人员工可以具有不同的执行机制，但在客服产品模型中必须遵循相同的身份、负责人和会话处理语义。

本方案采用以下原则：

1. `ServiceSession.assignee_identity_id` 是当前负责人的唯一事实来源。
2. AI 只是 `organization_identity.type = agent` 的员工子类型，不引入 AI 专属客服会话状态。
3. 收件箱、负责人筛选、领取、接管、转交、关闭和重新打开对真人与 AI 使用同一套接口和界面。
4. Agent Policy、State、Trigger 和 Run 属于后台执行与审计事实，不得成为第二套客服产品状态。
5. 实现差异可以存在于运行层，不能泄漏为 AI Tab、AI 模式或专属处理按钮。

## 3. 当前基础

当前代码已经具备以下基础：

- `ServiceSession` 使用 `open/closed` 表达处理周期，使用 `assignee_identity_id` 表达公共队列或当前负责人。
- 收件箱提供排队中、我负责的、同事和已关闭四个查询视图。
- 同事筛选和转交候选统一返回具有客服角色的真人与 AI 企业身份。
- 领取、接管、同事转交、关闭和重新打开已经使用通用客服命令实现。
- 其他员工负责会话时，当前用户不能直接回复，只能先接管；负责人是 AI 时保持相同行为。
- Agent Direct 已具备 State、Trigger、Run、Task、最终消息写回和运行中连续消息处理能力。
- 网站访客可以通过轮询读取新增消息，不依赖 Realtime 即可形成首轮自动回复闭环。

P1b 不重新实现上述客服流程，只在当前负责人是一名可执行 AI 客服时驱动后台自动回复。

## 4. 统一产品语义

| 能力 | 真人员工 | AI 员工 |
| --- | --- | --- |
| 当前负责人 | `assignee_identity_id` 指向真人身份 | `assignee_identity_id` 指向 Agent 身份 |
| 会话状态 | `open/closed` | `open/closed` |
| 收件箱筛选 | 在同事筛选器按身份查看 | 在同一个筛选器按身份查看 |
| 负责人信息 | 现有同事筛选和转交组件展示真人身份 | 使用同一组件和位置，必要时附轻量 AI 类型标签 |
| 领取或接管 | 通用 Claim 命令 | 同一个 Claim 命令 |
| 转交 | 通用 Transfer 命令 | 同一个 Transfer 命令 |
| 其他人负责时 | 当前用户不能回复，可以接管 | 完全相同 |
| 关闭与重开 | 通用 ServiceSession 命令 | 完全相同 |
| 后台执行状态 | 不进入客服状态 | Trigger/Run 不进入客服状态 |

本阶段明确不增加：

- `CustomerAgentStatus`；
- AI `active/paused/taken_over` 状态；
- “交给 AI”“暂停 AI”“恢复 AI”等专属操作；
- 客服收件箱中的 AI Tab；
- 客户会话中的 `queued/running/failed` 标签；
- 以 `Policy.enabled` 表达当前是否由 AI 负责。

如果以后需要暂停接待，应先定义为真人与 AI 共用的客服会话能力，而不是增加 Agent 专属暂停状态。

## 5. 自动履职资格

网站客户会话的 Agent 执行资格必须同时满足：

1. 当前 `ServiceSession.status = open`；
2. `assignee_identity_id` 指向一个有效的 Agent 企业身份；
3. 该身份处于 active 状态且 `roles.kind = customer_service`；
4. Agent 存在有效的 active Revision；
5. 当前渠道具备 Agent 外发能力，P1b 仅开放 website。

真人负责、未分配、会话已关闭、Agent 已停用、Revision 无效或渠道不支持外发时均不触发。客户入站路径还要求本次消息是首次成功持久化而不是幂等重放；转交路径可以指向已经存在但仍待回复的客户消息，不受该入站条件限制。

渠道路由保存、路由运行时校验、客服负责人候选和转交候选必须复用同一套 active customer service identity 判定。接待设置下拉也必须改为使用该候选集合，不能继续展示保存后才被后端拒绝的非客服身份。

对于尚未支持 Agent 外发的渠道，任何入口都不能把负责人设置为 Agent：路由保存和运行时、会话转交候选及 Transfer 后端校验都必须应用渠道能力限制。历史 Agent 路由视为不可用，沿用现有回退规则进入 fallback 或公共队列并记录 `WARN`，不能出现“会话显示由 AI 负责，但 AI 永远不会回复”的状态。

## 6. 负责人变化规则

### 6.1 真人转交给 Agent

转交仍使用通用 Transfer 命令。只有目标身份满足第 5 节中的身份、客服角色、Revision 和渠道能力条件时才允许把会话转给 Agent；非 website 会话的转交候选不包含 Agent，后端也必须拒绝绕过前端提交的 Agent 目标。

事务更新负责人后，必须重新核验第 5 节的五项 Agent 执行资格。任一条件不成立时，转交失败并回滚，且绝不创建 `customer_auto` Trigger；全部成立后才判断最后一条消息：

- 如果当前开放 ServiceSession 的最后一条消息来自 contact，说明客户仍在等待回复，为新负责人创建一个 `customer_auto` Trigger；
- 如果最后一条消息来自任意企业员工身份，Agent 等待下一条客户消息，不立即补发。

这保证队列中的待回复会话转给 AI 后可以立即开始处理，同时避免 Agent 对已经由员工答复的会话再次发言。

最后一条消息必须从当前开放 ServiceSession 的 `last_message_id` 出发，通过发送参与者对应的 `chat_subjects.kind` 判断为 `contact` 或 `organization_identity`。不得使用负责人类型、消息 author 枚举或 Conversation 级摘要代替，否则重新打开或存在多个处理周期时可能读取错误消息。

### 6.2 Agent 被真人接管

成员仍使用通用 Claim 命令接管 Agent 负责的 ServiceSession。若原负责人是 Agent，同一事务必须：

1. 只将本 Conversation 上、原负责人 Agent 的 `queued/running` Run 标记为 `cancelled`；
2. 接管写入 `error_code = assignee_changed`；防御性关闭路径使用 `session_closed`；
3. 如果对应 State 存在，将其 `processed_seq` 推进到事务内读取的 `desired_seq`；
4. 保留已有 Trigger 和 Run 作为审计记录；
5. 事务提交后再对模型调用发送尽力而为的 context cancellation。

数据库状态负责阻止迟到写回，context cancellation 只用于减少无效模型调用。

取消协调必须允许 State 或在途 Run 不存在。P1b-1 合并后、P1b-2 开始调度前，已经可以通过渠道路由或通用转交让 Agent 成为负责人，但这些会话尚无 State/Run；接管、转交和关闭不能因空运行状态失败。

取消绝不能只按 `agent_identity_id` 全局执行。Agent Direct 没有 ServiceSession，客服协调器不得触碰同一 Agent 在其他 Conversation 中的 Direct 或客户 Run。

P1b 不放宽现有命令权限。Agent 不能主动登录，因此当前真正从 Agent 移走负责人的入口是成员 Claim/Takeover；成员不能直接转交或关闭由另一个身份负责的会话，必须先接管。Transfer 和 Close 中的 Agent 取消只能作为事务防御，不能成为绕过负责人权限的特殊路径。

### 6.3 Agent A 转给 Agent B

P1b 不新增 Agent 专属改派命令。由于 Agent 当前不能主动登录操作，成员先通过通用接管取得会话，再通过通用转交交给 Agent B。

接管步骤取消 Agent A 的执行；转交给 Agent B 后，如果最后一条消息仍来自客户，则为 Agent B 创建新的 Trigger。不得复用 Agent A 的 State、序号或 Run。

未来若增加主管直接改派能力，应对真人和 AI 负责人统一开放，不在 P1b 内增加 AI 特例。

### 6.4 关闭与重新打开

- Agent 负责时，成员必须先通过通用接管取消在途 Run，再关闭自己负责的会话；直接关闭他人或 Agent 负责的会话仍然失败。
- 显式重新打开继续沿用现有语义，由执行操作的成员负责，不自动恢复给此前 Agent。
- 如需重新交给 AI，继续使用通用转交命令。
- 访客在已关闭 Conversation 中发送新消息时，按现有渠道路由创建新的 ServiceSession；若新负责人是合格 Agent，首条消息自然触发自动履职。

## 7. 数据模型调整

### 7.1 不在 P1b 创建 Policy 表

P1b 不创建 `conversation_agent_policies`。自动履职资格实时从 ServiceSession、负责人身份、Agent Revision 和渠道能力派生，避免 `Policy.enabled` 与 `assignee_identity_id` 形成两份可能漂移的当前负责人状态。

完整 P1 引入 `allowed_tools`、通用 `response_policy` 等真实策略需求时，再落完整 Policy 模型。

### 7.2 State 支持同一会话中的多个 Agent

将 `conversation_agent_states` 的主键从单列 `conversation_id` 调整为：

```text
(conversation_id, agent_identity_id)
```

Trigger 序号按“Conversation + Agent”连续增长，不按 ServiceSession 重置。Agent 离开后再次负责同一 Conversation 时继续使用自己的 State，避免与历史序号冲突。

P1a Agent Direct 的 upsert 冲突目标同步改为复合键，并保持现有执行行为不变。

### 7.3 Trigger 和 Run

为 Trigger 和 Run 增加：

- `trigger_type`，首轮值为 `agent_direct` 或 `customer_auto`；
- nullable `service_session_id`，Direct 为空，客户自动回复必须填写。

`mention` 本阶段不创建，留到群聊 Agent 阶段。

为 Run 增加：

- `cancelled` 终态；
- nullable `error_code`，P1b 使用 `assignee_changed` 和 `session_closed`。

迁移必须把现有 Trigger 和 Run 回填为 `trigger_type = agent_direct`，保持 `service_session_id = NULL`，并保留现有 `last_error`。`cancelled` 不属于现有 `(conversation_id, agent_identity_id) WHERE status IN ('queued', 'running')` 活跃唯一索引范围。

删除“同一 Conversation、Agent 和触发消息只能创建一次 Trigger”的永久唯一限制，仅保留“Conversation、Agent 和 Trigger 序号”唯一。消息幂等由消息首次插入事实保证。

这一调整用于支持以下合法路径：Agent A 已收到客户消息 M，随后会话被真人接管，再次转回 Agent A 时，A 仍然能够处理尚未答复的 M。

## 8. 事务与并发

所有客户自动回复相关路径统一采用以下加锁顺序：

```text
ServiceSession -> ConversationAgentState -> AgentRun
```

适用范围包括：

- website 入站消息调度；
- 领取和接管，以及 Transfer/Close 中不放宽权限的防御性取消；
- `customer_auto` Run 的 Claim、complete 和 fail；
- 完成后判断是否需要创建下一 Run。

Agent Direct 不读取 ServiceSession，继续使用 `State -> Run`，禁止执行过程中反向补锁 ServiceSession。

具体约束：

- 共用入站在选择 ServiceSession 时必须先 `FOR UPDATE` 锁定该 Conversation 的最新处理周期；无论向 open 周期追加，还是基于 closed 周期创建下一周期，都不能使用未加锁的快照；
- Website Action 必须在同一事务且锁仍持有时重新读取负责人并调度；Telegram 只参与共用锁定和消息写入，不调度 Agent；
- `cancelled`、`succeeded` 和 `failed` 都是终态，begin、complete、fail 和失败收敛都不能覆盖终态；
- 在途 Run 存在时，新消息写入 Trigger 并推进 `desired_*`，不创建第二个 Run；
- 创建 queued Run 时只冻结 `trigger_start_seq = processed_seq + 1`，`trigger_end_seq` 由每次 TurnLoop Claim 按当时连续可用的 `desired_seq` 扩展并绑定 Trigger；
- Tool 或模型调用期间到达的新消息由 Eino 在下一个安全点 Push，下一 Turn 再次 Claim 最新连续边界，因此仍属于同一 Run；只有最终回复事务已经取得 ServiceSession 锁并越过完成边界后到达的消息才进入下一 Run；
- customer_auto complete 或 fail 后，只有重新执行第 5 节资格判断仍成立且 `desired_* > processed_*` 时才能创建下一 Run，不复用 Direct 的无条件补 Run 逻辑；
- Provider 失败推进当前批次后，不自动重跑同一序号；
- 写回消息前必须在同一事务重新锁定并核验 ServiceSession、State 和 Run；
- 任一门禁失败时必须把当前 Run 收敛为 `cancelled` 或 `failed`，让出活跃唯一索引，不能遗留 `running`。

## 9. Website 调度入口

Website 和 Telegram 共用的入站逻辑负责在 `selectServiceSession` 中锁定最新处理周期并写入消息。Website Action 在同一事务、锁仍持有时重新读取 ServiceSession 负责人，再执行自动回复资格判断；不能使用入站函数返回的 assignee 内存快照。

调度不能直接放进网站和 Telegram 共用的入站消息函数，否则会让尚未具备外发闭环的渠道误触发 Agent。Telegram 只获得正确的 ServiceSession 并发保护，不创建 `customer_auto` Trigger。

幂等重放命中已有 Message 时直接返回，不创建 Trigger、Run 或 Task。

通用 ServiceSession 命令通过事务内的 Agent 运行协调器处理负责人变化。协调器只消费旧负责人、新负责人、会话状态和最后一条消息，不改变现有 API，也不向前端暴露 Agent 专属命令。

协调器使用依赖倒置接入：客服 Action 依赖最小事务接口，Agent 运行模块提供实现，并在应用装配层注入。接口只传递当前事务和必要的会话/身份标识，不让客服包依赖 Agent Run 的具体模型，避免形成包循环依赖。

## 10. Customer Auto Runtime

`customer_auto` 与 Agent Direct 共用运行账本和 Eino TurnLoop，但使用独立的持久输入 Feed、客户上下文加载、资格门禁和写回逻辑：

- 非流式；
- 一个 Run 可以经过多次模型规划或测试 Tool 调用，但只写入一条最终文本 Message；
- queued 时只冻结起点，每次 Claim 保存当前实际消费的连续 `[trigger_start_seq, trigger_end_seq]`；
- 运行中到达的客户消息在下一个 Tool 或模型安全点进入同一 Run 的下一 Turn；
- calculator 是开发期验证安全点和并发边界的临时测试 Tool，可通过 `delayMilliseconds` 控制执行时长，正式发布前删除；它不代表 P1b 开放产品 Tool；
- 业务、设备和有副作用 Tool 仍不进入 P1b。

Customer Auto 复用通用 TurnLoop 的 Push 和安全点语义，不复用 Direct 的数据库 Claim、完成、失败和补 Run 逻辑。共享 begin、complete、fail 和失败收敛入口必须把 `cancelled` 视为已终结并成功返回，避免 Task 重试把已取消 Run 转为永久错误或重新激活。Provider 在首次 Claim 前失败时，失败边界至少推进到 `trigger_start_seq`；已有 Claim 时推进到 Run 已记录的 `trigger_end_seq`，不会对同一序号无限重试。

客户会话上下文角色映射为：

- contact 消息映射为 `user`；
- 任意企业身份，包括真人、当前 Agent 和其他 Agent，映射为 `assistant`。

客服上下文加载不能复用只读取 `organization_identity` 的 Direct 查询，否则客户消息会从模型上下文中消失。

上下文按 Conversation 读取，可以包含旧 ServiceSession 的历史；每次 Claim 的上界是本次 `trigger_end_seq` 对应消息的 `(originated_at, source_order, id)`，条数上限沿用 Direct 的 100 条。后到消息必须先形成新 Trigger 并被下一次 Claim 纳入，不能越过持久边界直接进入模型上下文。

## 11. 最终消息写回

模型调用完成后，在写入最终消息前重新核验：

1. Run 仍为可完成状态且没有被取消；
2. ServiceSession 仍为 open；
3. Run 的 `service_session_id` 与当前处理周期一致；
4. 当前 `assignee_identity_id` 仍是该 Agent；
5. Agent 身份仍为 active 且仍具有客服角色，Run 冻结的 Revision 仍是有效的 managed v1 Revision；Agent 的 active Revision 后续切换不废弃本 Run；
6. State 的批次边界仍与 Run 一致。

核验通过后：

- 确保 Agent 对应的 ChatSubject 和 ConversationParticipant 存在且有效；
- 以普通 Agent 企业身份写入 Message；
- 使用 `agent:<agent_run_id>` 作为最终 Message 的业务幂等键；
- 将 Message 绑定到当前 `service_session_id`；
- 在同一事务更新 Run 终态和 State 水位；
- 使用现有客服摘要的 `(last_message_at, last_message_source_order, last_message_id)` 比较规则更新 Conversation 与 ServiceSession 摘要；
- 在尚无员工响应时更新 `first_response_at`。

核验失败时不写客户时间线，将 Run 收敛为 `cancelled` 或 `failed`，并保留取消或抑制原因用于审计和日志。

## 12. 前端方案

P1b 不增加独立的 AI 客服产品状态，原则上不需要新增收件箱业务入口。

- 继续使用排队中、我负责的、同事和已关闭视图。
- 真人和 Agent 使用同一个负责人筛选器和转交组件。
- 支持 Agent 外发的渠道在转交候选中混合展示真人与 Agent；仅在容易重名或需要辨识时显示轻量“AI”标签。
- 不支持 Agent 外发的渠道只展示真人候选，Transfer 后端使用同一渠道能力规则校验，不能仅靠前端隐藏。
- Agent 负责时，输入框与另一个真人负责时保持相同：当前成员不能回复，只能接管。
- 不在客户列表或会话头展示 Agent Run 的 queued、running、failed 或 cancelled。
- 不把右侧“AI 助手”页签连接到 Policy 或 Run。

当前列表项和会话头不单独展示负责人，P1b 不为 AI 新增负责人头像区。以后如果增加负责人展示，真人和 Agent 必须使用同一组件和位置。

未来如果需要“正在输入”，应接入真人和 AI 共用的 presence/typing 机制，不把后台 Run 状态直接投影为客服状态。

## 13. PR 拆分

### 13.1 PR P1b-1：统一客服负责人运行基础

建议标题：`统一 AI 客服与真人客服的负责人运行基础`

范围：

- 统一路由保存、运行时路由、负责人候选和转交候选的客服身份校验；路由与 Transfer 都叠加渠道执行能力校验；
- 将渠道接待设置下拉改为同一套 active customer service identity 候选；
- 增加渠道 Agent 外发能力判定，首轮只允许 website；非 website 保存 Agent 路由失败，历史配置在运行时回退并记录 `WARN`；
- 将 State 主键调整为 `(conversation_id, agent_identity_id)`；
- 为 Trigger 和 Run 增加 `trigger_type` 与 `service_session_id`；
- 为 Run 增加 `cancelled` 和 `error_code`，保留 `last_error`，并回填现有 Direct 数据；
- 放宽 Trigger 的消息永久唯一约束；
- 在通用接管事务中按 Conversation 精确取消原 Agent 的执行；State 或 Run 不存在时正常空转，Transfer/Close 只保留不放宽权限的防御性处理；
- 建立统一锁顺序和终态保护；
- 让共享执行入口识别 `cancelled` 终态，并把 Direct upsert 调整为复合键；Direct 产品行为保持不变。

该 PR 不生成客户自动回复，重点验证数据模型、取消语义、转接并发和 Direct 回归。

### 13.2 PR P1b-2：支持 AI 员工自动接待网站客户会话

建议标题：`支持 AI 员工自动接待网站客户会话`

范围：

- website 客户消息首次插入后的 `customer_auto` 调度；
- 转交给 Agent 时对待回复客户消息补触发；
- 复用 Eino TurnLoop、安全点 Push，并实现独立 Customer Feed、完成和失败路径；
- 使用可控延时 calculator 覆盖 Tool 执行期间的连续消息并发测试，正式发布前删除该测试 Tool；
- 客户上下文角色映射；
- ServiceSession、负责人和 Run 的完成门禁；
- Agent 参与者建立、最终消息写入、摘要和首响更新；
- 访客 `after` 轮询读取最终回复；
- 修订 `agent-roadmap.md` 和 `chat-roadmap.md` 中旧的默认 Agent、最小 Policy、暂停和恢复表述。

该 PR 不新增客服前端状态，最多补充通用负责人组件中的轻量身份类型标签。

## 14. 验收清单

### 14.1 身份与界面同构

- 真人和 Agent 返回相同结构的负责人 DTO。
- 同事筛选可以按任意真人或 Agent 身份查看会话。
- 负责人筛选和转交组件对真人与 Agent 使用同一展示结构。
- Agent 负责时，输入框和接管操作与另一真人负责时完全一致。
- 客服前端不存在 AI 专属状态、Tab 或处理命令。

### 14.2 路由与触发

- website 路由到合格 Agent 后，首条客户消息触发自动回复。
- 路由到真人或公共队列时不触发 Agent。
- 非客服身份不能成为渠道负责人或转交目标。
- 渠道接待设置只展示合格客服身份，website 以外不能保存 Agent 路由。
- Telegram 和其他未开放外发能力的渠道遇到历史 Agent 路由时回退，不把负责人设为 Agent，也不触发 `customer_auto`。
- Telegram 和其他未开放外发能力的渠道不在转交候选中展示 Agent，直接向 Transfer API 提交 Agent 目标也会被拒绝。
- 同一次客户入站的幂等重放不创建第二个 Trigger 或回复；负责人离开后再次转回同一 Agent 时，允许对仍待回复的同一消息创建新序号 Trigger。

### 14.3 转接与取消

- 真人转给 Agent，最后一条为客户消息时立即处理。
- 真人转给 Agent，最后一条为员工消息时等待客户下一条消息。
- Agent A 被真人接管后，A 的迟到结果不能写入会话。
- 接管客户会话时，同一 Agent 在其他 Conversation 中的 Direct 或客户 Run 不受影响。
- Agent 负责但尚无 State/Run 时，领取、接管、转交和关闭的既有权限行为不受影响。
- Agent A 经真人接管后转给 Agent B，B 使用独立 State 和 Run。
- Agent A 转给真人后再转回 A，同一条仍待回复的客户消息可以重新触发。
- Agent 负责时必须先接管再关闭；直接关闭他人或 Agent 负责的会话仍然失败。
- 重新打开不自动还给此前 Agent。
- 转接、接管、关闭和模型完成并发时无死锁、重复回复或迟到发言。

### 14.4 Runtime 与写回

- 客户消息以 `user` 角色进入模型上下文，企业员工消息以 `assistant` 角色进入。
- Run 运行期间的新消息在下一个安全点进入同一 Run 的下一 Turn，并扩展实际 `trigger_end_seq`。
- 最终回复完成边界之后到达的新消息创建下一 Run，不丢失唤醒。
- 失败不自动重跑同一序号，也不会让 Agent Run 永久停在 running。
- cancelled Run 的 Task 重试成功收敛，不会恢复执行或被改写为永久错误。
- 最终消息使用真实 Agent 企业身份并绑定 ServiceSession。
- 最终消息使用 `agent:<agent_run_id>` 幂等键，Task 重试不会双写。
- 首次响应时间以及使用客服三元组排序的 Conversation、ServiceSession 摘要正确更新。
- 访客可以通过现有轮询读取 Agent 最终回复。
- Agent Direct 的复合键 upsert、触发、连续消息、计算器 Tool、幂等、失败收敛和最终写回行为保持不变。

## 15. 日志与可观测性

以下情况记录结构化日志：

- Run 开始和终结沿用 Direct 的结构化日志，并增加 `trigger_type = customer_auto`；
- Customer Feed 每次 Claim 记录 `agent_run_id`、前后 Trigger 边界和上下文消息数，用于确认运行中输入是否在下一 Turn 纳入同一 Run；
- Eino 通用 Tool Middleware 记录实际调用的开始、成功或失败、`agent_run_id`、`tool_name`、`tool_call_id` 和耗时，用于人工验证 Tool 安全点；
- 临时 calculator 仅补充记录同一 `tool_call_id` 的非敏感执行配置 `operation` 和 `delay_ms`，不重复记录调用生命周期；
- 取消在途 Run：`INFO`，包含 `agent_run_id`、`service_session_id`、`conversation_id` 和原因；
- 因负责人变化或会话关闭抑制迟到写回：`WARN`；
- Website 调度发现消息幂等命中时记录 `DEBUG`，避免第三方渠道重放产生高频 `INFO`；
- 发现非法负责人、无效 Revision、终态被重复处理或锁后资格变化：`WARN`；
- Provider 调用失败：沿用 Agent Run 失败日志并包含 Trigger 批次边界。

日志不记录客户消息正文、模型完整上下文、原始 Tool arguments、Tool 返回内容、calculator 操作数、访问令牌或 Provider 密钥。完整的 Step、Tool Invocation 和调用事件持久化留到后续可观测性 PR。

## 16. 非目标

以下能力不进入 P1b：

- Agent 专属暂停、恢复或人工接管状态；
- 通用 Tool Policy、产品 Tool 和有副作用工具；
- 流式输出和 Realtime；
- Telegram 或其他第三方渠道 Delivery；
- 群聊 `@Agent`；
- Agent 主动转交或主管一步改派；
- Team 自动挑选某个 Agent；
- 完整 Policy、Step、Tool Invocation 和费用快照；
- 客服指标、满意度和 SLA。

## 17. 路线图修订要求

P1b-2 实现时同步修订 `agent-roadmap.md` 和 `chat-roadmap.md`：

- 将“渠道默认 Agent”改为“渠道路由快照将 ServiceSession 负责人设置为 Agent”；
- 删除 P1b 创建最小 Policy 的要求；
- 删除人工接手、暂停和恢复的 Agent 专属命令；
- 删除 `paused_at`、`human_takeover` 和 `manual_pause` 作为 P1b 门禁的描述；
- 明确自动执行直接消费通用负责人和转交结果；
- 明确客户会话不增加 AI 专属状态；
- 删除“同一消息对同一 Agent 永久只能产生一个 Trigger”的要求，改为由消息首次插入和 Trigger 序号保证单次调度幂等；
- 明确 `customer_auto` 在 queued 时只冻结起点，运行中消息在 Eino 安全点由同一 Run 的下一 Turn 吸收；
- 删除聊天域把 `conversation_agent_states.paused_at` 作为 P1b 会话控制事实的描述；
- 保留完整 P1 再引入通用响应策略、工具策略和完整审计模型。
