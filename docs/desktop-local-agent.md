# Cervi 桌面端本地 Agent 运行实施方案

状态：待实施  
日期：2026-09-20  
范围：桌面端本地 Agent 运行（P4），覆盖 AI 单聊和只有群主一个真人的群聊  
关联：本文是 [Cervi 路线图](roadmap.md) 登记的独立方案，设计细节只保留在本文

用户坐在自己电脑前，让 AI 员工在本机读写代码、执行命令和操作界面。本文是主要参考，不构成强制约束，实施中按更合适的做法调整并同步更新；交付完成后连同路线图中的对应条目一并删除。

## 架构

```text
服务端
├── 发送事务与群聊轮转：建立 Run 并写入执行设备与执行工作区，不投递服务端任务
├── 设备工作通知：提交后经成员事件流推送 device_work_advanced
├── 领取与续租：设备领取 Run 并取得租约，续租响应回带取消状态
├── 运行期服务：Peek／Claim 读取 Lane 输入、知识检索、附件读取
├── 模型代理：校验 Run、设备与租约后注入供应商凭据并透传流式响应
└── 收尾：result 与 failure 两个入口进入既有结果事务，群聊在同一事务提取点名并轮转

桌面端 Go
├── 设备注册与领取循环：独立定时器加事件流唤醒
├── agentruntime：与服务端同一份代码
├── 远程适配器：InputFeed、知识检索、附件读取、模型 BaseURL 指向服务端代理
├── 本机工具：workspaceBackend 实现 filesystem.Backend，workspaceShell 实现 filesystem.Shell
├── 审批中间件：拦截工具调用并经 Wails 事件请求本机确认
└── 流式增量：经 Wails 事件交给本机前端渲染
```

## 不变量

- **运行时同一份。** 服务端与设备执行同一份 `internal/integration/agentruntime` 代码，有效配置（场景、依据策略、指令、模型品牌与参数、输入模态、工具清单、MCP 与知识库绑定）由同一个解析器 `ResolveAssignment` 产出。执行侧能力只影响工具清单、指令中的工具说明和 MCP 服务名称。
- **业务事实只在服务端。** Run、Lane、Input、内容块、结果消息幂等和群聊轮转全部在服务端事务内完成，设备不持久化业务事实，本地只保存设备注册、工作区路径和会话内命令许可。
- **凭据不下发。** 设备取得的 Revision 配置不含模型凭据，模型调用一律经服务端代理。
- **路径不上行。** 工作区真实路径只存在设备本地数据库，服务端只保存工作区编号与用户可见显示名。
- **能力按交集。** 有效工具集是 Agent 策略、会话绑定、设备能力清单和触发者授权的交集；任一侧出现对方不认识的能力名一律丢弃。
- **有意终止是终局。** 停止和租约过期后的 Run 不自动续跑，本机命令可能已产生副作用。

## 数据结构

服务端新增三张表，每张表一个建表迁移；`agent_runs` 的列与索引变更单独一个迁移。

`devices` 的列清单与认证要求见路线图的「设备注册与认证」。P4 落地 `id`、`organization_id`、`user_id`、`name`、`platform`、`work_seq`、`last_seen_at`、`revoked_at` 和时间戳，并增加三列：`install_id` 由桌面端首次安装时生成并保存在本地，配合唯一索引 `(organization_id, user_id, install_id)` 使同一台机器重装后不产生重复设备；`runtime_version` 是设备侧运行时版本；`tool_manifest jsonb` 是设备最近一次上报的本机工具能力广告，不是允许集。`trust_level` 与服务端 Agent 调用设备用的 `capability_manifest` 仍随 P2 增加。

`device_workspaces`：`id`、`organization_id`、`device_id`、`label`、`last_used_at`、时间戳。工作区是一等实体，一台设备可注册多个工作区，一个工作区可被多个会话绑定。

`conversation_device_bindings`：`id`、`organization_id`、`conversation_id`、`device_id`、`workspace_id`、`peer_trigger_capability`、`bound_by_user_id`、时间戳。唯一索引 `(organization_id, conversation_id)`，一个会话最多绑定一台设备的一个工作区。`peer_trigger_capability` 表示本人以外的成员触发时的能力等级，取值 `off`、`read_only`、`write_with_approval`、`same_as_owner`，首版固定 `off`。

`agent_runs` 增加：

| 列 | 含义 |
| --- | --- |
| `execution_device_id` | 执行设备，为空表示服务端执行 |
| `execution_workspace_id` | 执行工作区，派发时按会话绑定解析 |
| `claimed_at` | 设备领取时间 |
| `lease_expires_at` | 当前租约过期时间 |

新增唯一索引 `agent_runs_running_workspace_unique`：`(organization_id, execution_workspace_id) WHERE execution_workspace_id IS NOT NULL AND status = 'running'`。同一工作区同一时刻只有一个运行中的 Run，跨会话绑定同一目录时由数据库串行化，设备稍后重试领取。既有的 `agent_runs_active_scope_unique` 继续承担执行范围互斥，不新建第二套。

桌面端 SQLite 新增两张表：`device_registrations`（`server_url`、`organization_id`、`device_id`、`install_id`）和 `agent_workspaces`（`workspace_id` 主键、`server_url`、`organization_id`、`path`、`created_at`）。桌面端与移动端 SQLite 迁移各自独立，移动端不建这两张表。

## 契约

`appservice.Backend` 新增下列方法，全部 `auth=member`，运行期方法额外要求 `X-Cervi-Device` 头并在分发层校验设备属于当前用户且未撤销；写运行期方法再校验租约有效。

| 路由 | 作用 |
| --- | --- |
| `POST /devices` | 注册或更新本设备与能力清单，返回设备编号与当前工作水位 |
| `GET /devices` | 返回本人设备列表 |
| `DELETE /devices/:deviceID` | 撤销本人设备 |
| `POST /devices/current/workspaces` | 注册工作区并返回工作区编号 |
| `GET /devices/current/work` | 比较工作水位，返回待领取的 Run 摘要 |
| `POST /agent-runs/:runID/claim` | 领取 Run，返回执行指派与租约过期时间 |
| `POST /agent-runs/:runID/lease` | 续租，响应回带取消状态 |
| `GET /agent-runs/:runID/inputs` | Peek 输入信号 |
| `POST /agent-runs/:runID/inputs/claim` | Claim 输入并取得截至该边界的上下文消息 |
| `POST /agent-runs/:runID/knowledge/search` | 沿用服务端知识检索实现 |
| `GET /agent-runs/:runID/attachments/:messageID` | 读取本次运行的附件内容 |
| `POST /agent-runs/:runID/result` | 成功收尾 |
| `POST /agent-runs/:runID/failure` | 失败收尾 |
| `POST /conversations/:conversationID/device-binding` | 绑定设备与工作区 |
| `DELETE /conversations/:conversationID/device-binding` | 解除绑定 |

模型代理不进 `Backend` 契约：它是带流式的字节透传，不是类型化业务调用。代理在 `internal/api` 下手写 Gin 路由，认证复用登录 Token，额外校验 `runId` 属于当前用户且由本设备持有有效租约，按 Revision 锁定的品牌注入凭据、强制模型标识与 Revision 一致后透传流式响应。设备侧把 `ModelConfig.BaseURL` 指向该路由、`APIKey` 换成登录 Token。

成员事件流新增 `device_work_advanced`，只携带设备编号与最新 `work_seq`，首版复用用户 NATS Subject 并由 Realtime Gateway 按已认证 `device_id` 过滤，与变更通知共用发送队列并按设备合并为最新水位，不参与会话版本与 `GetSyncHeads`。

## 派发、领取与租约

- 建立 Run 的三个入口（发送事务、完成事务中的轮转、点名接力）在会话已绑定设备时写入执行设备与执行工作区、推进设备 `work_seq`，不投递服务端任务。`scheduleNextRun`、轮次语义和停止语义不变。
- 设备在启动、事件流重连和一个独立定时器上比较工作水位并领取。该定时器不依赖当前是否有 Run 在执行，也不依赖运行时是否已初始化。
- 设备按工作区串行安排领取顺序；被数据库唯一索引挡回时按退避重试，本机前端展示「等待同一工作区的其他任务完成」，其他端展示普通排队状态。
- 领取即取得租约，执行期定期续租。租约过期由服务端到期检查将 Run 置为失败并写入 `agent_error`，群聊继续轮转。设备重启后不得重新领取自己已丢失租约的 Run。
- 停止沿用现有停止事务。设备经事件流通知或续租响应得知取消后中断 Agent 循环，并终止该 Run 启动的整个命令进程组。
- 设备离线时 Run 排队不设超时，各端显示等待该电脑上线。设备被撤销、用户停用、会话解绑或工作区注销后，排队中的 Run 以明确错误码失败。
- 设备侧不引入 `task/client`：领取循环是一个定时器加事件流唤醒，不需要第二套任务框架。

## 运行时对齐

- `internal/integration/agentruntime` 与 `internal/integration/knowledgeretrieval` 去掉 `server` 构建标签，编入桌面端；两者只依赖 `common`、`domain` 和已无构建标签的 `integration/mcp`，检索源由调用方以闭包注入。移动端不编入。
- `RunRequest` 的依赖在设备侧实现为 API Proxy 调用：`InputFeed` 对应 Peek 与 Claim 两个路由，`KnowledgeSearch`、`ReadAttachment` 对应各自路由，`OnStream` 改为发 Wails 事件。运行时代码不因此改动。
- 行为快照在**领取**时写入，不在派发时写入：工具清单取决于领取设备的能力清单。快照写定后，重试落在不再提供这些工具的设备上时直接失败。
- 本地 Run 的时限单独配置，不沿用服务端的 `agentRunTimeout`。
- 流式增量只在本机前端渲染，其他端展示运行状态与最终回复，状态变化沿用会话版本推进。
- 首版不加载 Revision 绑定的企业远程 MCP 服务。

## 本机工具

首批工具由 `adk/middlewares/filesystem` 提供定义、提示词和大结果转存，Cervi 只实现两个接口。官方只提供内存后端，本地磁盘后端自行实现。

`workspaceBackend` 实现 `filesystem.Backend`：

- 所有路径先相对工作区根规范化，再经 `filepath.EvalSymlinks` 解析，解析后仍必须位于工作区内，`..` 或符号链接越出工作区的路径被拒绝。
- `GrepRaw` 用 Go 正则实现，工具描述改写为实际支持的语法，描述与实现保持一致。
- 实现 `MultiModalReader`，支持读取图片与 PDF。

`workspaceShell` 实现 `filesystem.Shell`：

- 工作目录固定为工作区根，以独立进程组启动。超时、取消和 Run 结束三条路径都终止整个进程组。
- 输出按字节上限裁剪并置 `Truncated`；命令预算由后端自己计时并置 `TimedOut`。
- 环境变量按白名单传递，不透传桌面进程的完整环境。
- 首版只在 macOS 与 Linux 注册命令执行工具；Windows 只提供文件工具，界面说明原因。

四个超时各自命名，互不复用：设备租约续期间隔、本地 Run 总时限、单次命令预算、单轮模型静默。

### 能力组与风险类别

能力在绑定和 Agent 策略上按组表达，单个工具映射进组。已有组内增加工具不改配置与界面。

| 能力组 | 工具 | 风险类别 | 审批 |
| --- | --- | --- | --- |
| 文件读取 | `ls`、`read_file`、`glob`、`grep` | 只读 | 自动允许 |
| 文件写入 | `write_file`、`edit_file` | 改本机状态 | 每次确认 |
| 命令执行 | `execute` | 执行任意代码 | 每次确认 |
| 界面操作 | 截图、鼠标与键盘输入 | 操作界面 | 每次确认 |

每个工具携带声明的风险类别，审批中间件按类别判断，**没有声明类别的工具默认需要审批**。注册表覆盖测试断言每个工具都有类别，且只读之外的工具都会触发审批。

### 工具扩展规则

- 设备在注册和启动时上报 `tool_manifest`，服务端按交集计算有效集，两侧未知的能力名一律丢弃。
- 服务端可为工具声明所需的最低设备运行时版本，低版本设备直接得不到该工具，不产生业务兼容分支。
- 审批中间件必须同时实现 `WrapInvokableToolCall` 和 `WrapEnhancedInvokableToolCall`。返回多模态结果的工具走 Enhanced 路径，只实现前者会让这类工具经基类直通默认实现而不触发审批。
- 启用后台命令时，后台任务生命周期绑定在 Run 上，Run 结束即终止。
- 工具总数使上下文中的工具定义成为负担时，改用 `adk/middlewares/dynamictool/toolsearch` 按需暴露。

## 审批

- 读取类工具自动允许；写入、编辑和命令执行每次确认。
- 审批中间件在工具执行前发 Wails 事件并阻塞等待本机确认；运行时与界面同进程，不需要 CheckpointStore 与中断恢复。拒绝时把拒绝原因作为工具错误交回模型，模型可以改方案重试。
- 确认界面标明发起的 AI 员工、工作区显示名和将要执行的完整命令或目标路径。可选「本会话允许同一命令」，命中范围按规范化参数哈希，不按前缀。
- 审批决定、工具参数、参数哈希和裁剪后的结果写入 Run 内容块，随收尾回写服务端审计。

## 工作区

- 工作区是设备上的一个目录，Agent 的文件工具与命令都在其中执行。
- 绑定是会话级的：每个 AI 单聊和群聊各自选择设备与工作区，允许多个会话绑定同一个目录。同一目录上的并发由 `agent_runs_running_workspace_unique` 串行化；需要并行时使用不同目录。
- 绑定了工作区的会话，其成员就是该工作区内容的披露范围。把一个工作区绑定到第二个会话时，界面列出它已有的绑定与各自的可见成员。
- 群聊由群主本人绑定，绑定期间群内只有群主一个真人；加入其他真人前先解绑，群主转让或退群时自动解绑。客户会话不支持绑定。
- 领取前必须能把工作区编号解析到仍然存在的目录，否则拒绝领取，服务端以明确错误失败。

## 安全边界

- 首版不提供操作系统级沙箱，命令执行以本机逐次审批和工作区围栏为防线，残余风险明确承认。
- 设备认证是登录 Token 加设备编号加撤销位，挡不住 Token 泄露后的设备冒用。后续强化路径是设备密钥对与请求签名。
- 绑定会话中的 Agent 以设备主人的本机权限执行工具，因此绑定期间群内只有群主一个真人。放开他人触发时按 `peer_trigger_capability` 逐档开启，`read_only` 一档不发起审批，可以先行放开；`write_with_approval` 要求主人在线，主人不在线时 Run 立即以「需要设备主人确认」失败，不挂起等待。
- 能力按触发者计算时，一个 Run 认领的输入窗口内混有多个触发者时取权限最低者的交集；Run 不得通过吸收新输入扩大能力；新输入会降低能力时，Run 停在当前边界不吸收，交给下一个 Run 处理。
- 引入跨 Run 长期记忆时，每条记忆必须携带来源受众标签，只返回给同受众的运行。

## 交付批次

**第 1 批 设备与派发。** `devices`、`device_workspaces`、`conversation_device_bindings` 三张表与 `agent_runs` 列变更；设备注册、撤销、工作水位、事件流通知、领取、续租、租约过期失败；运行期与收尾路由连同设备头校验和租约校验一起上线；AI 单聊会话绑定。取指派一段中与执行位置无关的部分在本批分出，访客提示、进程内取消登记与服务端运行时限留在服务端一侧。桌面端先跑返回固定文本的假运行时，把分布式正确性与运行时可移植性分开验证。验收：设备离线时 Run 排队不丢、上线后领取执行；杀掉桌面端后 Run 按租约失败且群聊轮转继续；停止后本机循环终止；设备撤销后不能领取。

**第 2 批 运行时编入。** 去掉两个包的 `server` 标签、远程适配器、模型代理、本机流式渲染。此时没有本机工具，本地 Run 与服务端 Run 结果等价，可直接对比验证。验收：设备侧不出现模型供应商凭据，服务端不出现工作区真实路径。

**第 3 批 只读本机工具。** `workspaceBackend`、工作区注册与绑定、工作区身份校验、路径围栏、能力清单上报与交集。验收：`..`、符号链接、已移动的工作区全部被拒；低版本设备得不到未支持的工具。

**第 4 批 写入、命令与审批。** `workspaceShell`、进程组终止、审批中间件两条路径、会话内命令许可、决定入块。验收：拒绝后模型能改方案重试；停止时命令进程组终止；多模态结果工具同样触发审批。

**第 5 批 群聊绑定。** 成员限制、排队状态展示、确认界面标明 AI 员工。

**第 6 批 界面操作。** 截图、鼠标与键盘输入，随 macOS 辅助功能与屏幕录制授权引导一起交付。

## 验收边界

- 执行设备离线时 Run 排队不丢失，上线后领取执行。
- 应用退出或崩溃后 Run 按租约进入失败，群聊轮转继续，不自动续跑。
- 停止后本机 Agent 循环和命令进程组终止。
- 设备侧不出现模型供应商凭据，服务端不出现工作区真实路径。
- 工作区外路径被拒绝；工作区目录不存在时拒绝领取。
- 同一工作区同一时刻只有一个运行中的 Run；同一执行范围同一时刻只有一个 Run。
- 绑定群聊按现有轮次语义在本机依次执行。
- 设备撤销后不能领取 Run 或调用模型代理。
- 服务端与设备对同一 Run 解析出同一份有效配置。
- 未声明风险类别的工具不会被自动放行。

## 不做的事

操作系统级沙箱、ACP 两个方向的接入、移动端本地运行、设备端本地模型与离线运行、跨设备迁移进行中的 Run、远程指挥不在场的电脑、设备侧本地知识库。

## 未决

- 设备侧运行时崩溃与桌面端主进程崩溃的区分上报方式。
- Windows 命令执行的进程组方案与开启条件。
- 附件在设备与模型代理之间的二次搬运是否需要由代理直接注入。
