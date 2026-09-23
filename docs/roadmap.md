# Cervi 路线图

日期：2026-09-22。

本文是项目的主计划文档，只记录尚未交付的事项；已交付能力以代码为准。事项按领域分组，已确定范围的写成 PR 计划并保留设计细节，其余写粗略计划与启动条件。某个 PR 或计划完成后，直接删除本文对应内容和已实现完的独立方案文档，不另行记录交付情况。

规模较大、需要独立评审的方案另开文档：本文保留范围、关键决定、阶段划分和与其他事项的关系，完整契约、数据模型与验收留在该文档。当前有 [官方托管服务建设方案](saas-hosting-plan.md)、[助理与桌面端本地运行实施方案](desktop-local-agent.md) 和 [工作台信息架构与服务对象扩展方案](workbench-ia.md)。

本文与 `docs/` 下的方案文档是主要参考，不构成强制约束。实际开发中经常出现新的想法和变动，出现更合适的做法时按新做法实施，并同步更新对应文档；文档里已经写下的设计本身不是必须照做的理由。

## 总体顺序

1. 客服闭环：解决与关单、会话小结、运营报表与知识缺口、网站显式评价 → 访客与客户信息 → 第一批渠道与 AI 翻译 → 网站 Messenger 占位能力、AI 满意度与质检、联系人资料、常用语 → 第二批渠道、自定义 API 渠道、开放接口与权限。
2. 工作台信息架构：阶段 A 的前端整理与客服闭环并行，阶段 B 起在客服闭环交付后启动，见 [工作台信息架构与服务对象扩展方案](workbench-ia.md)。
3. 服务对象扩展：员工服务台（IT 先行），伙伴服务台在出现真实客户时启动，同上方案。
4. Agent：助理与桌面端本地运行（P4）。
5. 官方托管服务：按 [官方托管服务建设方案](saas-hosting-plan.md) 的阶段推进，与客服闭环并行，不占用同一批业务改动。
6. 其余事项按各自的启动条件开启；安全、容量与性能在准备正式上线时集中处理。

## 1. 聊天与实时

### 聊天能力补齐

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 移动端离线推送 | 只在应用进程存活期间投递本地通知；iOS 挂起或退出后到达的消息回到前台才同步；vivo 等国内厂商系统只有经厂商推送通道下发的通知才带桌面角标 | 按 APNs、FCM 与厂商推送阶段交付 |
| 移动端通讯录分类 | AI 员工、团队和外部联系人分类是占位 | 移动端需要按这些分类找人 |
| Telegram 未关联引用定位 | 原消息不在 Cervi 时展示发送者名称与原文快照，不提供原消息定位 | 出现定位需求 |
| 网站渠道单入站会话策略 | 同一访客可同时有多条进行中会话，Conversation 为公开主键 | 出现真实需求时增加「只允许一个入站会话」的可选渠道策略 |

### 实时同步（按证据开启）

当前做法：会话版本 + SSE 通知 + 30 秒兜底校验，客户端收到通知后重读业务 Query。以下事项只在出现需求或实测证据时开启，具体设计在启动时单独成文。

#### 待开启事项

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 网站挂件网络与窗口状态拆分为 TS 模块 | 请求、身份、线程目录、消息窗口与发送状态都在 `internal/publicweb/chat.js` 内，事件流通知触发既有拉取逻辑 | 挂件需要与成员端共用传输代码，或修改 `chat.js` 状态逻辑时出现可复现的回归 |
| 实时链路指标与启动就绪顺序 | 认证失败、慢连接结束与 NATS 发布失败只记录 `WARN` 日志；服务端没有对外 ready 阶段 | 出现需要监控实时链路的真实部署，或已观察到启动就绪顺序导致的连接失败 |
| 会话列表虚拟化与窗口裁剪 | 列表按已加载窗口全部渲染，控制器保存双向边界与锚点 | 真实列表规模出现可复现的渲染或滚动问题，并记录实际节点数与滚动耗时 |
| 实时链路容量与故障验证 | 只有集成测试覆盖撤销、慢连接、最长存活与服务端下线；没有容量数据 | 出现真实部署规模，或已观察到连接、队列、锁争用的具体问题 |

#### 按证据恢复的原设计

以下设计在范围评审中删除或推迟，当前由更简单的做法承担，出现对应证据时再恢复。

| 原设计 | 当前做法 | 启动条件 |
|---|---|---|
| 持久用户／客服／访客水位与压缩唤醒行 | 客户端内存投影刷新即重建，`GetSyncHeads` 实时聚合可见会话的版本哈希和 | `GetSyncHeads` 聚合在真实数据量下实测过重，或确定建设持久离线库 |
| 完整同步索引、重建基线、保留窗口与恢复 epoch | 重连等价于重读已加载窗口 | 客户端需要跨会话持久游标，即离线库落地时 |
| 会话变更事件日志与 `sync_seq` | 以 `conversations.version` 表达会话有变化，客户端重读业务 Query | 按会话版本重读被实测证明过重，需要细粒度失效 |
| Protobuf 协议与代码生成工具链 | 手写 JSON 事件，Go 与 TypeScript 以共用夹具锁定格式 | 事件载荷显著变大变热，或协议需要提供给第三方 |
| 一次性连接票据 | 原生端由 Go 侧 `apiproxy` 持有事件流，Web 端请求头携带既有 Bearer | 原生端不再由 Go 侧持有连接，或出现新的凭据边界要求 |
| 事务内 `realtime_outbox` 与租约发布器 | 事务内登记通知，提交后异步发布 Core NATS，丢失由 30 秒兜底校验恢复 | 实测提交后发布丢失导致的感知延迟不可接受，或兜底校验间隔需要显著拉长 |
| 跨执行节点运行快照 | Gateway 与 Agent 执行在同一进程内定位 Run | Gateway 与 Worker 拆分部署 |
| 多节点 Drain、随机重连分散与多节点验收 | 单实例部署，收到退出信号即拒绝新事件流并有界结束现有事件流 | 服务端拆分为多实例部署 |
| 服务端定期复核授权与群失权控制通知 | 事件流最长存活 1 小时；运行过程流逐次校验阅读资格；群失权由 `conversation_removed` 通知客户端关闭运行过程流 | 实测 `conversation_removed` 丢失导致失权后仍收到 AI 流、最长存活时间限定的撤销窗口不可接受，或出现需要即时收回的其他受限事件 |
| 客户端实体版本比较与串行追赶 | 客户端不保存实体版本，收到通知即失效对应 Query 并按短窗口合并重读 | 通知触发的重复重读被实测证明过重 |
| WebSocket 双向实时连接 | SSE 只承载下行事件，上行全部走普通 HTTP | 出现事件流与 HTTP 无法满足的高频双向需求，届时下行事件一并迁移 |
| 挂件跨标签页身份发现、本地通知跨标签页前台协调、NATS 恢复时主动校验探针、前端事件处理队列上限 | 各标签页独立连接与拉取，NATS 恢复场景由 30 秒兜底校验覆盖 | 出现实际重复通知、身份发现遗漏或发送堆积反馈 |

#### 启动时须遵守的约束

- **挂件拆分：** 经现有 messenger 构建入口迁入 `frontend/src/publicweb`，按网络身份、线程目录、消息窗口与待发送状态拆分，DOM 事件转成显式操作；保留 Go 模板、主题与现有 DOM 外观，产物继续由 Go 嵌入链路加载，bundle 不包含成员登录模块或内部 Agent 详情。先验证与现有事件流及拉取逻辑等价，再删除迁出的原实现。
- **指标与就绪顺序：** 启动顺序为依赖就绪 → 路由与订阅可服务 → 对外 ready，未就绪时不接受事件流请求；指标按结构化原因码聚合认证失败、慢连接关闭、发布失败与探针不一致，不记录凭据或正文。多节点 Drain 与随机重连分散在拆分部署时另行处理。
- **列表虚拟化：** 先测量节点数与滚动耗时并记录必要性；只替换可见行渲染，数据分页、选择与同步契约沿用；行 key 固定为会话 ID，裁剪保留可恢复的前后边界与语义锚点，只保留一个滚动补偿所有者；不以固定截断条数代替分页或容量治理。
- **容量与故障验证：** 以可重复的 Task 场景覆盖同用户多会话、小群扇出、共享客服受众、资料批量失效、突发重连和持续热点，记录数据量、在线连接数、受众大小、持续时间与机器配置；故障注入点只用于测试，不成为生产接口；合成测试数字不写成产品承诺。锁优化、普通性能索引与大群 Shared Fanout 按结果另行开启。

#### 待补的验证记录

| 事项 | 当前状态 | 补记时机 |
|---|---|---|
| iOS 与 Android 真机上的 SSE 长响应、事件流重连与本地通知 | 只在桌面开发的 mobile-preview 验证过原生端 SSE 长响应 | 各自真机接入时 |
| Cloudflare Tunnel 与 HTTPS／HTTP/2 部署下的 SSE 下发 | WebSocket 探针阶段验证过隧道，改为 SSE 后未记录 | 首次在隧道或正式 HTTPS 部署下验证时 |
| 移动端置顶排序：从手柄开始的触摸不切换范围、排序中打开的菜单关闭后才结束排序 | 审核修正后没有界面复验记录 | 下次涉及移动端置顶的界面验证时 |
| 会话搜索「查看全部」：移动端返回恢复已加载窗口与滚动位置、移动端首次读取失败重试、修改或清空检索词回到分组结果 | 审核修正后没有界面复验记录 | 下次涉及消息页检索的界面验证时 |


### 第三方用户账号接入

先落地平台无关的最小账号接入骨架，再以 Telegram + TDLib 作为首个适配器验证。此阶段包括：

- 创建精简的 `messaging_provider_connections`，使用稳定的 `external_scope_id` 表示远端身份命名空间；连接端点允许为空和变更，不承担账号身份。
- 在代码适配器目录中登记 `provider`、`adapter_kind`、认证方式和理论能力；实际能力取适配器能力、授权范围、账号观测状态和企业策略的交集，不建立数据库能力矩阵。
- 创建 `user_messaging_accounts`，明确 `owner_user_id`、连接、授权状态、账号级凭据引用、投影游标和最近投影时间。授权成功后以 `(organization_id, connection_id, provider_account_id)` 唯一识别远端账号。
- 一个 Cervi 账号可以绑定多个第三方账号；同一聊天列表展示各账号会话并明确标识收发账号。
- 创建 `connected_chats`、`external_senders` 和 `connected_message_records`，子表通过账号和连接确定平台，不重复保存 `provider`。
- 第三方标识和游标统一使用文本；仅在平台提供可靠顺序时保存可空 `source_order`，不为缺失顺序制造伪值或唯一约束。
- 联系人和会话投影、文本收发、编辑、删除、已读、历史补拉、断线恢复、限流、幂等和账号隔离均按适配器声明的实际能力降级。
- 账号投影游标、远端会话投影游标和平台已读游标分别建模，不能用一个通用 `sync_cursor` 混合表达。
- `connected_message_records` 保存账号、远端会话、消息映射和外发状态，不复用客服渠道的 `customer_message_deliveries`。
- 外发结果不确定时先进入 `uncertain`，通过平台历史或远端查询对账；无法确认前禁止直接自动重发。
- 平台更新使用账号或远端会话投影游标，不进入通用入站 Inbox 表。
- 凭据只通过连接级或账号级 `credential_bundle_ref` 引用加密密文，不进入 DTO、日志和普通任务载荷；运行时租约只限制同一账号记录的并发连接。

Telegram 首个实现额外验证 TDLib 会话托管、FloodWait、远端历史对账和明文投影边界。本阶段不建设通用 OAuth 产品、连接管理页面、账号共享绑定表或数据库能力矩阵；这些能力应在第二个平台或真实共享需求出现后再提炼。

#### 接入分类与产品边界

第三方用户消息账号表示 Cervi 用户本人授权、能够以该用户身份读取和发送消息的外部账号。Telegram + TDLib 是第一个实现，但数据模型不以 Telegram 的编号、会话文件或能力范围作为通用假设。

外部平台按授权主体和产品用途分流：

| 类型 | 典型场景 | Cervi 模型 | 规则 |
| --- | --- | --- | --- |
| 完整客户端协议 | Telegram TDLib、Matrix Client-Server API、XMPP 用户账号 | Provider Connection + 用户消息账号 | 可以按平台能力逐步替代官方客户端 |
| Workspace 委托授权 | Slack 用户 OAuth、Teams 委托 Graph | Provider Connection + 用户消息账号 | 只提供官方授权实际允许的范围，不承诺完整客户端能力 |
| 企业客服 API | 网站、微信公众号、Telegram Bot、WhatsApp Business 等 | `channels` + 客户会话 | 企业拥有的客服入口，不是员工个人账号 |
| 非官方用户自动化 | 个人微信 Hook、Discord Selfbot、盗用 User Token 等 | 默认拒绝 | 不因技术上可模拟登录就纳入用户账号模型 |

同一平台可以同时拥有 Channel Adapter 和 User Messaging Adapter，但两者属于不同领域。例如 Telegram Bot 继续进入 `channels`，Telegram TDLib 用户会话才进入本章模型。`ChannelType` 不表示个人消息账号，也不与 User Messaging Adapter 共用存储枚举。

#### Provider、Adapter 与外部命名空间

`provider` 只表示品牌或协议族，例如：

```text
telegram
matrix
slack
teams
xmpp
```

真正决定鉴权方式和理论能力的是代码中的 `adapter_kind` Catalog，例如：

```text
telegram_tdlib_user
matrix_client
slack_user_oauth
teams_delegated_graph
xmpp_client
```

Catalog 必须声明 Adapter 所属领域为 `user_messaging`，防止 Bot、客服 API 或非官方自动化被写入用户账号表。`adapter_kind` 表示稳定协议契约，不表示某个连接器进程或实现版本。

远端账号、会话和发送者编号的唯一作用域不是整个 Provider，而是平台中的稳定外部命名空间。因此本阶段增加薄 `messaging_provider_connections`：

```text
messaging_provider_connections
├── id
├── organization_id
├── provider
├── adapter_kind
├── external_scope_id
├── display_name
├── endpoint
├── credential_bundle_ref
├── status
├── created_at
└── updated_at
```

字段语义：

- `external_scope_id` 是平台规范的稳定身份命名空间，不是当前网络地址。
- `endpoint` 是可空、可变的连接地址或发现结果，不参与身份唯一性。
- `credential_bundle_ref` 是可空的安装级密钥引用，不保存密钥正文。
- `organization_id`、`provider`、`adapter_kind` 和 `external_scope_id` 创建后不可修改。
- `display_name`、`endpoint`、`credential_bundle_ref` 和 `status` 可以按平台状态更新。

核心唯一性：

```text
UNIQUE (organization_id, provider, external_scope_id)
INDEX  (organization_id, adapter_kind, status)
```

平台映射示例：

| 平台 | `external_scope_id` | 可变 `endpoint` |
| --- | --- | --- |
| Telegram 公共网络 | 固定 `telegram` | 空 |
| Matrix | MXID 中的 `server_name` | 发现得到的 Client-Server API Base URL |
| Slack | `team_id` | 空 |
| Teams | Entra ID `tenant_id` | 空或平台发现结果 |
| XMPP | JID `domainpart` | SRV 解析得到的主机与端口 |

Connection 是外部 ID 命名空间和安装授权边界，不是 Connector Worker、TDLib 进程、数据中心或设备会话。运行时租约不能通过创建更多 Connection 表达。

#### 用户账号、所有权与共享

本阶段的用户账号目标结构：

```text
user_messaging_accounts
├── id
├── organization_id
├── connection_id
├── owner_user_id
├── provider_account_id
├── display_name
├── status
├── credential_bundle_ref
├── projection_cursor
├── last_projected_at
├── created_at
└── updated_at
```

规则：

- `owner_user_id` 指向 `users.id`，不能指向 `organization_identities.id`，避免 AI 智能体成为外部账号授权主体。
- 本阶段一个账号只有一个 Owner。一个 Cervi 用户可以拥有多个不同外部账号。
- `provider_account_id` 使用 Adapter 规范化后的稳定 `text` 编号；认证进行中允许为空，账号进入活动状态前必须写入。
- `projection_cursor` 是账号级、不透明的 `text` 投影进度，不使用 `last_projected_at` 代替协议游标。
- `credential_bundle_ref` 是账号级会话密钥引用，不保存 Session、Token、密码或 TDLib 数据库密钥正文。
- `organization_id`、`connection_id` 和已确认的 `provider_account_id` 创建后不可修改。

完成认证后的永久唯一性：

```text
UNIQUE (organization_id, connection_id, provider_account_id)
    WHERE provider_account_id IS NOT NULL
```

该约束防止同一企业内两个 Cervi 用户分别启动同一外部账号的重复运行时。不同企业之间不创建全局唯一或密钥指纹互斥：同一外部账号可以在不同租户中建立独立授权会话，且不能泄露它是否已在其他企业绑定。

账号生命周期：

- 解绑时撤销或销毁凭证，把状态改为 `disconnected`，但保留账号行和历史投影，不释放外部账号唯一性。
- 只有当前 Owner 可以在原账号行上重新授权；不能通过重新插入账号实现接管。
- Owner 停用不删除账号或历史消息，但重新授权、销毁凭证和所有权变更需要专用 Action。
- 所有权转移只允许在同一企业内转给有效用户，不修改外部账号、会话凭证和历史发送者。

未来确有共享账号需求时再增加：

```text
user_messaging_account_bindings
├── organization_id
├── messaging_account_id
├── user_id
├── role
├── created_at
└── updated_at
```

Binding 只表示额外查看或代发授权，Owner 仍以 `owner_user_id` 为唯一真相。本阶段第一版不创建该表，团队成员或客服角色也不自动获得账号访问权。

#### 会话、发送者、消息和账号状态

进入本阶段时按实际能力创建以下扩展表，子表不重复保存 `provider`，统一通过 `messaging_account_id -> connection_id` 解析：

```text
connected_chats
├── id
├── organization_id
├── conversation_id
├── messaging_account_id
├── provider_chat_id
├── remote_type
├── title_snapshot
├── projection_cursor
├── created_at
└── updated_at

external_senders
├── id
├── organization_id
├── messaging_account_id
├── sender_kind
├── external_id
├── display_name_snapshot
├── avatar_snapshot
├── created_at
└── updated_at

connected_message_records
├── id
├── organization_id
├── messaging_account_id
├── connected_chat_id
├── message_id
├── provider_message_id
├── provider_sender_id
├── direction
├── delivery_status
├── client_operation_id
├── source_order
├── last_error
├── provider_payload_reference
├── created_at
└── updated_at

conversation_account_states
├── id
├── organization_id
├── conversation_id
├── messaging_account_id
├── provider_read_cursor
├── local_read_message_id
├── pinned
├── archived
├── muted_until
└── updated_at
```

核心唯一性：

```text
connected_chats:
  UNIQUE (organization_id, messaging_account_id, provider_chat_id)
  UNIQUE (organization_id, conversation_id)

external_senders:
  UNIQUE (organization_id, messaging_account_id, sender_kind, external_id)

connected_message_records:
  UNIQUE (organization_id, messaging_account_id, connected_chat_id, provider_message_id)
    WHERE provider_message_id IS NOT NULL
  UNIQUE (organization_id, messaging_account_id, client_operation_id)
    WHERE client_operation_id IS NOT NULL

conversation_account_states:
  UNIQUE (organization_id, messaging_account_id, conversation_id)
```

`connected_chats` 只负责远端会话映射和系统投影进度。`conversation_account_states` 负责已读、置顶、归档和静音等账号视图状态，不能为了少一张表混入映射记录；只有第一种真实账号状态能力落地时才创建物理表。

`external_sender` 创建对应 `chat_subject` 后才能成为参与者和消息发送者。`sender_kind` 是具体 Adapter 的闭集，不建设跨所有平台的全局分类学；Telegram 首期使用 `user | chat`。

#### 外部编号、游标和来源顺序

所有稳定远端标识使用 `text`：

```text
provider_account_id
provider_chat_id
provider_message_id
external_id
client_operation_id
projection_cursor
provider_read_cursor
```

Adapter 负责平台规范化，业务 Action 只校验非空、长度和作用域。禁止统一 `lower()`，也不把用户名、手机号、邮箱、工作区名称或 Endpoint 当作稳定账号主键。

第三方用户账号映射中的 `connected_message_records.source_order` 是可空 `bigint`，只在平台提供同一远端会话内稳定、可比较的整数顺序时填写：

- Telegram 可以使用整数消息编号。
- 没有稳定整数顺序的平台保持为空，时间线退回 `(originated_at, id)`。
- 不得使用时间戳转换、Hash、填零或平台内部不可公开的深度值伪造顺序。
- 不为 `(connected_chat_id, source_order)` 创建唯一约束。
- 客服和站内时间线的 `messages.source_order` 已为 Telegram Bot 私聊增加，使用非空 `bigint`，没有来源顺序时统一为 `0`；它与第三方账号映射字段不共用约束。

账号级 `projection_cursor` 表示当前账号投影进度，会话级 `connected_chats.projection_cursor` 表示单个远端会话历史补拉进度，`provider_read_cursor` 表示平台已读位置。三者语义不同，不能共用时间戳或消息编号字段。

#### Adapter 能力契约

理论能力由代码中的 `adapter_kind` Catalog 定义，不创建数据库能力位图表。至少按需声明：

```text
contacts
direct_chats
group_chats
history_backfill
send_text
edit_message
delete_message
reactions
read_receipts
files
channel_identity_send
```

具体账号可用能力由以下交集决定：

```text
Adapter 理论能力
  ∩ Connection 安装授权和 Scope
  ∩ Account 当前运行状态及限制
  ∩ 企业策略
```

统一 Adapter 契约围绕真实能力逐步增加，例如授权或恢复会话、列出和补拉会话、串行投影 Update、发送文本、编辑、删除、已读，以及把平台结果分类为确定成功、确定拒绝、可重试或结果不确定。没有真实需求的能力不预建空接口。

前端根据当前 Connection 和 Account 的实际能力展示操作，不能通过 `provider = telegram` 等条件写死功能。Slack、Teams 等 Workspace 授权即使使用相同表，也只能展示官方 Scope 和 API 实际允许的能力。

#### 凭证、会话与运行时隔离

凭证按作用域分为：

- 部署级配置：例如 Cervi 默认 Telegram 应用的 `api_id/api_hash`，由服务端配置管理，不复制进企业业务表。
- Connection 级凭证：例如 Workspace App 安装、Bot Token 或企业自带应用配置，通过 `messaging_provider_connections.credential_bundle_ref` 引用。
- Account 级凭证：例如 TDLib Session 与数据库密钥、委托用户 Token、Matrix Access Token 和设备编号，通过 `user_messaging_accounts.credential_bundle_ref` 引用。

Credential Bundle 存放在独立密钥存储或受管数据卷中，业务表只保存不透明 UUID 引用：

- 存储模型中的引用不序列化，appservice 和前端 DTO 中不存在该字段。
- 任务 Payload 只携带 `organization_id` 和 Connection、Account 或 Chat 编号，Worker 执行时按引用读取凭证。
- 普通日志、错误体和 Provider Payload 不记录凭证、TDLib 数据库路径或会话内容。
- 轮换时先创建新 Bundle，再用短事务切换引用；旧 Bundle 在宽限期后清理，不原地修改密文。
- Connection 和 Account 的 Bundle 不能复用同一条记录，避免卸载平台安装和解绑个人账号时互相删除密钥。

同一个 `user_messaging_accounts.id` 同时只允许一个 Connector Runtime 持有运行租约，保护账号投影游标和外发对账。租约不跨账号行或企业全局互斥，也不能通过 Connection 表达进程拓扑。

#### 会话映射不变量与平台例外

跨平台保持以下存储和发送路由不变量：

```text
一个绑定账号下的一个远端会话
    -> 一个 connected_chat
    -> 一个 Cervi Conversation
```

每次发送通过 `conversation_id -> connected_chats.messaging_account_id` 唯一解析账号，禁止使用用户默认账号或跨账号聚合结果猜测发送路由。

平台差异通过 Adapter 规则处理：

- 两个账号加入同一个远端群时保留两个账号视图，不合并存储。
- 平台把 Thread 表达为父会话内消息关系时使用 `thread_root_message_id`；平台分配独立 Chat ID 时创建新的 `connected_chat`。
- 群升级、Room Tombstone 等平台迁移使用显式迁移关系，不按标题或成员自动合并。
- 不允许投影的 Secret Chat、受保护内容或授权范围外资源不创建连接会话。
- 跨账号统一收件箱只做只读查询聚合，不能改变账号所有权、已读状态或发送路由。

Telegram 首个 Adapter 的特定映射：

- 每个企业确保一条 `provider = telegram`、`adapter_kind = telegram_tdlib_user`、`external_scope_id = telegram` 的薄 Connection，不提供独立 Connection 管理页面。
- `provider_account_id` 使用认证完成后取得的 Telegram 用户编号十进制字符串，手机号和用户名只作可变别名，不作主键。
- TDLib Session 目录和数据库密钥属于 Account Credential Bundle；数据中心和设备授权不是新的 Connection。
- Telegram Bot Token 永不进入用户消息账号，继续走 `channels`。
- 容器和发送主体分离：讨论组使用自己的 `provider_chat_id`，以频道身份发言和匿名管理员使用 `sender_kind = chat` 的 `external_sender`。
- Saved Messages 是合法连接会话；频道与讨论组是两个远端会话，不自动合并。
- 基础群升级超级群时按平台迁移事件建立显式映射；Secret Chat 和受保护内容默认不投影。

#### 发送、同步与生命周期规则

- 一个 Cervi 用户可以绑定多个不同平台或同平台账号，聊天列表必须清晰标识接收和发送账号。
- 第三方用户账号不能写入 `channels` 或 `customer_conversations`，不能使用 `chmsg:` 幂等键；除用户明确执行“保存为外部联系人”外，也不写入 `contacts` 或 `contact_channel_identities`。
- 本地发起的远端操作使用稳定 `client_operation_id`，发送状态和远端消息编号保存在 `connected_message_records`，不依赖 `task_runs` 作为业务投递账本。
- 外发调用出现超时、连接中断或进程崩溃时先标记结果不确定，通过平台本地状态、历史或权威查询对账；只有确认远端未发送后才能重试。
- Connector 按账号串行推进投影游标，支持断线重连、限流、幂等写入和历史补拉；平台瞬时 Update 不默认全部写入 PostgreSQL。
- 账号解绑不删除已沉淀消息；保留、脱敏或删除由企业策略和用户授权决定。
- 编辑、删除、反应、置顶、草稿和已读按 Adapter 能力逐项接入，不承诺所有平台都能双向同步。


### 受管外部协作与企业联邦

实现合作方企业、受管访客、定向邀请、外部协作门户、受管单聊群聊、文件访问和审计。

实现企业信任连接、联邦身份投影、跨企业单聊群聊、成员与消息事件同步、断线补拉和访客身份升级。同步协议使用独立的联邦 Inbox/Outbox，以对等部署和协议事件编号永久防重；不复用 `task_runs`、客服 Delivery 或客户端实时事件定义。联邦编码届时按服务端协议独立确定。

#### 受管外部协作

伙伴的一对一咨询先由伙伴服务台承担，本节只处理伙伴进入 Cervi 内部会话协作的场景，启动条件见 [工作台信息架构与服务对象扩展方案](workbench-ia.md)。

未部署 Cervi 的供应商或合作伙伴由当前企业托管访客账号，只能访问被邀请的会话和文件。受管访客不能浏览完整通讯录、加入未邀请群、查看内部备注或管理企业资源。

默认使用定向、一次性、短期有效邀请。入口优先使用响应式 Web/PWA 和安全链接。小程序只是客户端壳，不是新的消息渠道。

访客来源记录创建 `kind = guest` 的 `chat_subject` 后即可使用通用参与者和消息模型，但登录、邀请和访问范围仍由访客域负责。

#### 联邦通信

联邦通信遵循：

- 不假设所有参与者存在于同一数据库。
- 不使用数据库自增编号作为跨部署标识。
- 服务器请求需要认证、签名、幂等和重放保护。
- 远端成员只保存必要投影，不创建成本地企业成员。
- 会话、消息、成员和文件同步都显式带协议版本和全局标识。
- 首期不实现分布式事件图或多主冲突解决。

未来联邦用户投影创建 `kind = federated_user` 的 `chat_subject`。受管访客升级为联邦身份时不重写历史发送者，只新增经过双方确认的身份关联供界面聚合，新消息使用新的联邦主体。

联邦阶段使用独立的协议入站和出站事件表，以 `(peer_deployment_id, event_id)` 等协议稳定编号永久防重，并保存签名、协议版本、投递确认和重放窗口。联邦协议事件不能复用 `task_runs`、客服投递表或客户端同步日志。


### 结构化大型协作

根据真实需求选择公开群、公告群、话题模式、独立子讨论空间和跨企业共享的服务会话。大型群 Shared Fanout、会话级订阅引用计数和容量治理按真实规模证据落地。

## 2. 客服运营与渠道

本节只记录事项、做法方向和优先级，具体设计在启动时单独成文。优先级 P0–P3 表示能力的建设先后；渠道使用独立的接入批次。

### 产品原则

Cervi 是 Agent First 的客服系统：客户的第一接待默认由 AI 员工完成，真人客服负责兜底、监督和处理 AI 解决不了的问题。客服是第一个交付的服务对象，员工与伙伴复用同一套业务层，见 [工作台信息架构与服务对象扩展方案](workbench-ia.md)。

- 能由 AI 完成的传统客服功能，直接以 AI 形态实现：分类、小结、资料录入、质检和意图分配都属于此类。
- 优先补齐 AI 依赖的底座和 AI 无法替代的环节：转人工后的承接、会话结束口径、访客身份与上下文。
- 不建设通用规则引擎；只提供确定性规则，即已上线的负责人未响应提醒与回收，以及「解决与关单」超时关单。
- 运营闭环以 AI 表现为中心：AI 解决了多少、为什么转人工、缺哪些资料，并把缺口回流到知识库。
- 先用网站与 Telegram 验证 Agent First 闭环：AI 接待、转人工承接、解决口径和知识缺口回流。微信是国内可售的门槛，列入第一批渠道，不作为闭环验证的前置条件。

### 解决与关单（P1）

超时跟进与超时关单只作用于仍由 AI 负责、且最后一条对客消息来自 AI 的客服周期，包括普通回答、追问和解决确认。客服周期进入队列或由真人负责后，AI 不再发言，客户失联由客服关单，负责人未回复按企业设置的未响应提醒与回收处理。

| 事项 | 做法 |
|---|---|
| AI 解决确认 | AI 判断问题已解决时向客户确认；客户确认后关闭客服周期，结束方式记为 AI 解决 |
| 客户否认 | AI 继续处理或转人工 |
| 超时关单 | 客户超时未答复时，AI 跟进一次并请客户确认问题是否已解决；仍无回复则关闭客服周期，结束方式记为客户失联。AI 请求确认后客户未答复同样记为客户失联 |
| 人工关单 | 真人负责的客服周期由负责人关闭；团队队列或公共队列中无人负责的客服周期由任一客服关闭。结束方式均记为人工关闭 |
| 结束方式 | AI 解决、客户失联、人工关闭作为持久事实保存在客服周期上，只表示客服周期如何结束 |

AI 请求关闭客服周期调整 AI 客服角色行为方案中「AI 回答后不自动关闭」的约定，启动时同步修订该方案。

### 会话小结与分类（P1）

- 客服周期关闭时由 AI 生成小结，并标注咨询分类、客户情绪和是否解决；客服可修改。结束方式为客户失联时，是否解决留空。
- 企业只维护一份咨询分类目录，同时用于小结、报表和意图路由。
- 意图路由：AI 转人工时从目录中选择分类，分类映射到团队；未配置映射时使用渠道备用路由。
- 咨询分类、客户情绪和是否解决由判断模型标注，小结正文由对话模型生成。转人工时的意图分类和业务原因在 `handoff_to_human` 参数中以枚举给出，不另调判断模型。

#### 判断模型

判断模型用于在固定选项内做出判断，不生成文本。它与供应商无关，调用方只依赖判断接口。

- 模型类型新增 `decision`，与 `chat`、`embedding`、`rerank` 并列。
- 只提供三种判断：是否、单选（最多 255 个选项）、有序评分（2–10 级）。每个结果都附带概率。
- 默认实现使用企业已配置的对话模型，通过结构化输出返回结果和概率；也可以配置 Jev 等专用判断模型。
- 概率低于阈值的结果不自动写入，留空或进入待复核。
- 需要生成或抽取原文的任务使用对话模型，例如小结正文、联系人资料、交接卡片。

### 运营报表与知识缺口（P1）

| 维度 | 指标 |
|---|---|
| 响应与处理 | AI 首响；转人工后真人首响；AI 处理时长（进线到转人工或 AI 关单）；人工处理时长（接手到关单）。现有 `first_response_at` 含 AI 回复，只作为整体首响 |
| 结束方式 | 按「解决与关单」的结束方式统计 AI 解决、客户失联、人工关闭的占比 |
| 解决情况 | 按「会话小结与分类」中小结的是否解决统计解决率，可按负责人类型、渠道和分类拆分；是否解决留空的周期单列 |
| AI 独立解决率 | 结束方式为 AI 解决（客户已确认）且该周期内未发生真人领取、接管、转交或真人对客回复的周期占比，作为对外口径 |
| 客户评价 | 「满意度与质检」访客评价单独统计，与结束方式和小结对照展示，不覆盖二者 |
| 转人工原因 | `handoff_to_human` 增加业务原因分类：知识不足、客户要求真人、需要人工判断、投诉；与 `outcome_reason` 的系统原因（依据不足、预算耗尽、运行失败等）分别统计 |
| 知识缺口 | 业务原因为「知识不足」和系统原因为「依据不足」的转人工会话聚类为问题清单，标注出现次数与示例会话，可一键进入知识库补充问答 |
| 工作量与分布 | 客服接待量、各渠道会话量、咨询分类分布 |

知识缺口清单、回答质量评测和经验沉淀在本节统一规划，AI 客服运行结果是其事实来源。报表上线时对企业成员全员可见，数据范围随权限统一建设。

### 满意度与质检

| 事项 | 优先级 | 做法 |
|---|---|---|
| 网站显式评价 | P1 | 客服周期结束后请访客评价「是否解决」并可填写评语；评价挂在已关闭的客服周期上，访客下次打开 Messenger 时仍可评价。与报表同期交付 |
| 其他渠道评价 | P2 | 渠道不支持交互时降级为文本回复 |
| AI 推断 | P2 | 由判断模型推断满意度与解决情况，覆盖全部客服周期 |
| AI 质检 | P2 | 真人与 AI 客服的会话都检查，由判断模型标记答错、态度问题、应转人工未转等情况；标记结果与低概率结果形成待复核清单 |

### 访客与客户信息

| 事项 | 优先级 | 做法 |
|---|---|---|
| 访客上下文 | P1 | 网站 Messenger 采集来源页、当前页、设备和地区，在客服侧栏展示并注入 AI 上下文 |
| 登录用户识别 | P1 | 企业网站用服务端签名把已登录用户身份传给 Messenger，关联到联系人；为 AI 读取客户档案和业务数据提供身份 |
| 离线回复通知 | P1 | 访客留有邮箱且不在线时，客服回复通过邮件通知访客；与邮件渠道相互独立 |
| 黑名单 | P1 | 客服屏蔽联系人或渠道身份，屏蔽后不再创建客服周期。启动时确定进行中客服周期的处理、屏蔽后入站消息是否回复，以及网站 Messenger 是否仍可打开 |
| 信息收集 | — | 由 AI 通过 `ask_customer` 在对话中收集，不做开聊前表单 |
| 联系人资料 | P2 | AI 从对话中抽取姓名、电话、订单号等更新联系人资料；支持联系人标签与自定义字段 |
| 重复联系人 | P2 | AI 提示疑似重复，客服确认后手动合并 |

### 网站 Messenger（P2）

网站 Messenger 按 Intercom 的结构搭建，以下内容当前为占位。访客侧文案按渠道当前有效的接待配置展示：首选路由为可接待的 AI 员工时表达 AI 可立即回答，工作时间与等待时长只在转人工后说明；首选路由为真人、团队或公共队列时按真人接待表达。

| 占位内容 | 当前行为 | 做法 |
|---|---|---|
| 帮助中心 | 首页「探索帮助」卡片和「帮助」页签（文章合集、搜索、文章详情、仍需帮助入口）使用固定示例文案 | 文章与合集从知识库发布；访客搜索时由 AI 直接回答并附相关文章，未解决时进入对话。只发布标记为公开的内容，知识库不默认公开；启动时确定 AI 回答是否创建客服周期、是否计入「运营报表与知识缺口」 |
| 在线标识 | 客服头像固定显示在线圆点 | 成员在线与离线由客户端活跃状态维护，只用于在线标识，不作为分配条件。渠道由 AI 员工首接待时，未开聊和 AI 接待期间展示该 AI 员工并始终可用；真人接待时按承接客服的实际在线状态显示；渠道由真人首接待且未开聊时按可分配客服的在线情况显示 |
| 回复时间 | 固定显示「我们通常会尽快回复」 | 渠道由 AI 员工首接待时表达 AI 可立即回答；真人接待或转人工后按「运营报表与知识缺口」真人首响统计和企业客服工作时间说明预计等待时间 |
| 语音消息 | 录音只在预览模式可用 | 访客发送语音消息，AI 转写后进入客服与 AI 上下文 |

### 常用语（P2）

- 个人与企业两级常用语，纯文本，在对客输入区快速检索并插入草稿。
- 不做变量、多级分类、附件和使用统计。
- 企业级常用语可转入知识库问答条目，供 AI 客服与 AI 写回复使用；个人常用语保持私有。

### AI 翻译（P2）

- 客服侧显示客户消息的译文，客服用自己的语言回复，发送时翻译为客户语言。
- 原文与译文同时保存，时间线可切换查看。
- 与海外渠道同期建设。

### 开放接口与权限（P3）

- 开放 API 与 Webhook：企业业务系统读写联系人、会话和消息，订阅会话事件。
- 权限：已有角色权限参与鉴权，并增加会话与报表的数据范围（全部、本团队、本人）。
- 工单不单独建设：跨部门办理由服务会话、AI 交接卡片和内部协作承担，启动条件见 [工作台信息架构与服务对象扩展方案](workbench-ia.md) 的暂缓事项。


### 渠道接入

#### 接入完成标准

会话型渠道（一对一客服对话）接入需同时具备：入站验签与幂等、文本收发与外发投递、AI 员工接待资格（`ChannelSupportsAgentAssignee`）、平台支持时的附件与引用。

通知型渠道（短信等）只承担单向通知，不创建客户会话，不要求 AI 接待资格。

只接入企业官方客服接口，个人账号自动化不作为客服渠道。平台的回复窗口、消息模板和资质要求在启动对应渠道前按官方文档核实。

#### 国内

| 批次 | 渠道 | 要点 |
|---|---|---|
| 第一批 | 微信客服（企业微信） | 国内战略入口，一个入口覆盖视频号、小程序、公众号菜单、App 和网页 |
| 第二批 | 微信公众号（服务号） | 服务未开通微信客服的客户；被动回复有 5 秒时限，AI 回复走异步客服消息接口，受 48 小时窗口约束 |
| 第二批 | 邮件 | IMAP／SMTP 或转发接入的会话型渠道 |
| 第三批 | 抖音企业号、小红书专业号、快手私信 | 开放能力与资质差异大，按客户需求启动 |
| 第三批 | 短信 | 通知型渠道 |
| 远期 | 淘宝／天猫、京东、拼多多 | 需要服务商资质 |
| 远期 | 电话与呼叫中心 | 独立的语音体系 |

#### 海外

| 批次 | 渠道 | 要点 |
|---|---|---|
| 第一批 | WhatsApp Business Cloud API | 24 小时窗口外只能发送审核通过的模板消息，AI 跟进与超时关单受此约束 |
| 第一批 | Facebook Messenger 与 Instagram 私信 | 同一套 Meta 接口 |
| 第二批 | LINE 官方账号 | 日本、台湾、泰国 |
| 第二批 | 邮件 | 同国内 |
| 第三批 | Discord Bot 私信 | 社区型产品 |
| 第三批 | Viber、KakaoTalk、Zalo | 按目标市场启动 |
| 第三批 | Twilio 短信 | 通知型渠道 |
| 远期 | X 私信、Apple Messages for Business | 接口成本或资质门槛高 |

Google Business Messages 已停止服务，不接入。

#### 内部 IM 机器人

员工服务台的渠道，都使用免公网回调的长连接方式，适合自托管部署。

| 批次 | 渠道 | 要点 |
|---|---|---|
| 第一批 | 钉钉企业内部应用机器人 | Stream 长连接；单聊与群 @；`senderStaffId` 可查部门；AI 卡片支持流式 |
| 第二批 | 飞书应用机器人 | 长连接事件订阅；群内可收全部消息；`user_id` 与部门需申请权限 |
| 第三批 | 企业微信智能机器人 | 长连接；非超级管理员创建时只返回加密 userid，查部门需另建自建应用并受可信 IP 限制 |

#### 通用接入

| 批次 | 渠道 | 要点 |
|---|---|---|
| 第二批 | 自定义 API 渠道 | 渠道自带入站接口与出站 Webhook，企业自行接入任意平台；与「开放接口与权限」开放 API 相互独立 |
| 第三批 | 移动 App SDK | iOS、Android、Flutter 内嵌客服 |


### 客户会话附件

|---|---|---|
| 访客端分片上传与断点续传 | 访客上传为单次直传，超过 20 MiB 的文件在创建上传时被拒绝 | 出现访客需要发送大文件的渠道场景 |
| 公开上传端点的频率限制与配额 | 访客上传只按访客令牌授权和单文件上限校验 | 准备正式上线时，随安全与容量专项统一补齐 |
| 微信公众号等其他渠道的媒体收发 | 附件能力只对网站与 Telegram 开放，其他渠道入口禁用 | 对应渠道的文本收发交付后 |
| 客服侧附件转存、内部备注附件、Copilot 线程向客户会话转发附件 | 附件只用于对客回复，内部备注只支持文本 | 出现客服内部流转附件的明确需求 |
| 图片压缩、缩略图生成和视频转码 | 图片预览直接使用原件地址 | 出现大图预览的带宽或加载时长问题 |
| 附件内容的病毒扫描与敏感内容审核 | 不扫描、不审核 | 准备正式上线时，随安全专项评估 |
| Telegram 媒体发送期间的渠道锁范围 | 投递 Worker 在发送期间持有渠道咨询锁与一条数据库连接，最长为 5 分钟的媒体发送超时，期间保存机器人凭据、停用渠道会等待 | 保存渠道配置被阻塞成为实际问题时；需要先确定锁只覆盖认领与写回时，更换机器人与在途发送的并发语义 |
| 移动端后台上传 | 上传队列在本次登录的应用页面生命周期内有效，系统挂起或进程退出后中断 | 与移动端内部聊天附件的后台传输一并建设 |

### 客服协作

#### 移动端

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 移动端内部备注输入与 @ 同事 | 移动端只读展示内部备注，提供 `@我的` 视图、提醒计数与提及导航 | 出现客服主要在移动端处理客户会话的场景 |
| 移动端 Copilot | 移动端客户会话不提供「AI 助手」页签，只提供 AI 写回复 | 与移动端内部备注输入一并评估 |

#### 协作与提醒

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 内部备注 `@所有人` | 内部备注只能逐个提醒企业真人成员 | 出现需要一次通知全体客服的协作场景；需要先确定「所有人」的范围 |
| 客户会话提醒的系统通知 | 客户会话提醒计入应用角标与客户范围圆点，不单独按提醒发送系统通知 | 出现协作者错过提醒的反馈；需要先确定与现有新消息通知策略的关系 |
| 新消息通知范围收窄 | 客户会话的新消息通知对全部客户会话生效，与是否本人负责无关 | 出现客服被非本人负责会话的通知打扰的反馈 |
| 团队级工作时间 | 工作时间只有企业级一份设置 | 企业出现不同时区或不同班次的团队。团队选择沿用企业设置或单独维护，转人工话术按会话进入的队列取用（团队队列取团队设置，公共队列取企业设置）；Messenger 的回复时间文案按渠道首选路由对应的队列推导。工作时间计算只接收一份设置，与设置来源无关 |
| Copilot 结论发为内部备注 | Copilot 结论保存在共享线程，只能填入对客草稿 | 出现需要把 AI 分析留痕到客户会话时间线的需求 |


## 3. 服务对象扩展与工作台

把客服闭环建立的业务层扩展到员工与伙伴，并按同一套概念整理工作台。完整设计见 [工作台信息架构与服务对象扩展方案](workbench-ia.md)。

关键决定：

- 产品只有两个概念：会话承载谁在和谁说话，服务周期承载一件需要组织负责的事；收件箱是服务周期的视图。
- 是否产生服务周期只看发起人是不是该 AI 员工的服务对象。渠道会话天然产生；成员对服务对象不含自己的 AI 员工是试聊，与助理的对话也不产生。
- 提出请求的一方看到聊天，处理的一方看到收件箱，同一条会话不复制数据。
- 转人工交出责任，交给队列或团队；请示保持责任，向专家要一句事实或口径，属于暂缓事项。
- 工作台一级导航为收件箱、AI 员工、通讯录，群聊与单聊直接列在一级栏；渠道与知识库归入 AI 员工模块。

| 阶段 | 内容 | 前置 |
|---|---|---|
| A 工作台整理 | 交接卡片与 AI 过程折叠、渠道页只显示已接入、收件箱改为待处理与全部并按等待起点排序（含窗口查询与 DTO 改动）、一级导航调整、AI 员工模块合并渠道与知识库 | 与客服闭环并行 |
| B 共同底座 | 服务对象、服务会话泛化、员工目录同步、身份范围注入、群话题服务周期、可见性按角色 | 阶段 A；客服闭环交付 |
| C 员工服务台 | 内部 IM 机器人渠道、IT 知识问答与转人工承接，其后 MCP 查询、身份系统执行与主管审批 | 阶段 B；审批依赖 P1.5 |
| D 伙伴服务台 | 伙伴组织与成员、身份绑定、按伙伴路由、业务系统查询类工具 | 阶段 B；需要开放查询接口的真实客户 |

与其他事项的关系：渠道按「渠道接入」的内部 IM 机器人批次交付；伙伴进入内部会话协作见「受管外部协作」；工具审批依赖「P1.5：服务端副作用、审批和恢复」；数据范围随「开放接口与权限」的角色权限一并收口。

## 4. AI 客服与客服 AI 辅助

AI 解决后关单与超时关单、运营报表、知识缺口、回答质量评测、经验沉淀与满意度见「客服运营与渠道」。

### 回答依据

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| MCP 作为回答依据 | 依据来源只登记 `search_knowledge`；MCP 查到的业务记录不能放行对客正文 | 出现依靠 MCP 查询订单、物流等记录直接答复客户的需求；需要确定工具级「可作为回答依据」开关、默认值与通用判定（调用成功、未标记错误、结果非空）|
| 外部业务系统接入 | 不提供外部系统接入配置、侧边栏查询与读写授权矩阵 | 出现需要接入企业自有业务系统的客户；接入方案同时定义读取工具的依据判定 |
| 客户历史作为依据 | `search_customer_history` 为不可用占位实现，不计入依据 | 客户历史查询实现后评估其结果能否作为依据 |
| 分段读回拼接 | 模型分多次读回同一转存结果时，各次读回都不计入依据 | 出现多行大结果的依据来源，且模型常按分段读回 |
| 依据范围校验 | 当前边界内任一依据来源有效即放行正文，不校验正文是否忠实于资料 | 建立回答质量评测后，按评测结果决定是否增加运行期校验 |

### 接待体验

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 客户档案注入 | 客服上下文只含当前周期的对客消息，不注入客户档案、渠道与历史会话摘要 | 客户历史查询或客户档案能力落地 |
| 跟进问题复用依据 | 每次认领新输入都要求重新查证，客户补充一句信息也会触发重查 | 统计显示重查显著拉长响应或抬高转人工率 |
| 表单式追问 | `ask_customer` 只发送自由文本 | 需要结构化收集订单号、联系方式等信息；需要先设计表单消息类型与客户侧渲染 |

### 配置与运行

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| AI 员工能力配置化 | 角色页的 AI 员工能力只读，由角色类型派生 | 与外部业务系统读写授权矩阵一并建设 |
| 配置页能力提示 | 客服 AI 未绑定知识库时只能追问或转人工，配置页未单独提示 | 出现管理员误以为 AI 客服异常的反馈 |
| 多语言内置指令 | 角色基线与场景规则只有中文版本，回复语言跟随客户 | 中文基线对其他语言客户的效果出现问题 |
| 模型故障切换 | 主模型不可用时按运行失败转人工 | 出现真实模型故障导致的批量转人工反馈 |

### 客服 AI 辅助

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 自动预生成候选回复 | 客服打开 AI 写回复弹层后才生成，客户新消息到达不自动生成 | 单次生成耗时成为客服处理瓶颈 |
| 渠道或团队默认 AI 员工 | AI 写回复与 Copilot 默认使用本人上次选择的 AI 员工 | 出现由管理员统一指定客服辅助 AI 员工的需求 |
| AI 辅助费用统计 | AI 写回复只在日志记录 Token 用量；Copilot 用量随运行过程展示 | 出现按企业或成员核算 AI 费用的需求 |
| Copilot 客户历史检索 | 背景只含客户会话最近消息，`search_customer_history` 尚未接入真实查询 | 客户历史检索接入真实查询后，为 Copilot 注入指向所属客户会话的检索工具 |
| Copilot 页签实时刷新 | 页签打开时前台轮询读取线程列表、线程消息和运行状态 | 客户端按会话变更通知失效 Copilot 相关查询 |

## 5. Agent 运行时与设备

### P4：助理与桌面端本地运行

成员在自己的电脑上创建助理，助理在这台电脑上读写代码、执行命令和操作界面。助理只进入主人所在的单聊和群聊，其他成员可以在群里使用它，写入、命令与界面操作都由主人审批；客服会话只使用 AI 员工。AI 员工只在服务端执行，助理只在主人绑定的电脑上执行，执行位置由归属决定。

设备是一类受用户控制的 Agent Worker：服务端保留 Run 的全部业务事实、输入认领、结果事务、群聊轮转和企业模型凭据，设备借出工作区和执行本机工具的进程。两端执行同一份 `agentruntime` 代码与同一个有效配置解析器，本机工具通过 Eino 的文件系统与 Shell 接口注入，审批在绑定电脑上同步完成。助理绑定一台电脑，工作区按「会话 × 助理」指定，真实路径只存在设备本地。

设备注册、派发领取、运行时编入、模型代理与助理身份已上线，剩余按五批交付：运行时补齐（知识检索、附件与本机流式渲染）、只读本机工具、写入与命令审批、群聊中的助理、界面操作。之后按需增加「本机 Agent」模型来源，由主人电脑上的外部编程 Agent 使用其自身登录与订阅。

产品定义、交互、数据模型、审批规则、安全边界与批次验收见 [助理与桌面端本地运行实施方案](desktop-local-agent.md)。


### 服务端 Agent 完整能力（P1）

- 补齐 Policy、State、Step 和 Tool Invocation。
- 扩展既有 Eino Adapter，接入服务端类型化 Tool。
- 支持通用自动响应策略、费用和失败审计。
- 工具只读或具有强业务幂等。
- 正式发布前删除开发期 `calculator` 工具。

验收边界：Agent 能以统一参与者身份可靠说话；崩溃或任务重复不会产生重复输出；运行期间的新消息不会丢失。

#### 会话级策略

完整 P1 有真实的通用响应与工具策略需求时增加 Policy：

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
```

`trigger_mode` 沿用 `mention`、`agent_direct`、`customer_auto`、`copilot` 的长期语义；需要合并多条输入的入口只有在显式响应策略允许时才能合并。Tool Policy 保存产品能力标识和策略，不保存 Eino Tool 实例或 Go 类型。

#### Run 审计快照与费用

- `agent_runs` 增加 `input_snapshot`：保存实际传给模型的有序输入、消息编号、内容版本或哈希、摘要引用和 schema 版本，不能只保存起止消息编号；按审计需求保存必要内容并限制大小，敏感字段按产品策略处理。
- `token_and_cost_usage` 在具备价格快照后记录费用；当前只记录输入、输出 Token 与耗时。

#### 语义步骤

完整 P1 使用 `agent_run_steps` 记录模型、工具、审批或交接等有界语义步骤：

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

#### 工具调用

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
- 服务端工具也通过接收 `AgentExecutionContext` 的类型化 Tool Adapter 调用 Action，不允许 Eino Tool 绕过组织边界直接拼 SQL，也不得伪造登录用户复用其完整权限。
- 一次 Tool Invocation 对应一次业务副作用意图；基础设施重试不能创建新的副作用意图。


#### P1 最小接入集

P1 只使用：

- ChatModelAgent。
- Runner 和流式事件迭代。
- 服务端类型化 Tool。
- `context` 取消。
- 必要的模型用量采集。

P1 工具只允许只读或具有强业务幂等的服务端能力。

P1 默认不使用：

- 跨 Run 常驻 TurnLoop；Conversation 的长期监听仍由 Trigger 水位和新 Run 表达。
- 持久 SessionStore。
- Automemory。
- BackgroundTask Store。
- DeepAgent 或本地 Filesystem 工具。
- 默认 CheckpointStore。

#### 按触发条件引入 v0.10 能力

| 能力 | 引入条件 | 业务边界 |
| --- | --- | --- |
| Checkpoint | 出现无法由 Run、Step、Tool Invocation 和审批事实重建的中间状态 | 只作可丢附件，不能替代业务状态或解决外部副作用 `uncertain` |
| TurnLoop | 同一个 Run 需要运行中 Push、新输入抢占或长期 idle 生命周期 | Conversation 的长期监听仍由消息游标和新 Run 表达 |
| SessionStore | 单个 Run 内确需框架工作集或 Middleware 回放 | 不作为 Conversation 或聊天历史 |
| Automemory | 出现明确的跨 Run 长期记忆产品 | 先确定来源、使用范围与遗忘交互；复用本地知识库的转换与向量管线及后台模型配置；Cervi 持久化事实、来源、纠正与遗忘状态 |
| BackgroundTask Store | 出现同一 Run 内子代理或长工具的框架级租约 | 服务端唤醒仍由 `task_runs` 承担，设备长任务由客户端工作项承担 |
| Reduction Middleware | 真实上下文或工具输出达到上限 | 业务层仍先限制输入和大结果，不依赖 Middleware 兜底 |

审批并不自动要求 Checkpoint。审批请求、参数哈希、决定和有效期必须先成为业务事实；只有审批后无法从这些事实安全重建 Runner 状态时，才增加 Checkpoint。


#### MCP 连接测试

外部平台统一通过 MCP 提供工具。MCP 连接测试复用 `internal/integration/connectiontest` 的执行语义：协议握手、能力发现和工具列表校验由 MCP 适配器实现；服务端 MCP 与设备本地 MCP 使用同一错误分类，通过 `location`（`server`／`device`）保留执行位置差异。

新增连接测试的步骤：

1. 在 `appservice` 增加该集成的强类型草稿输入和测试方法。
2. 在对应 Action 中复用保存配置时的字段校验。
3. 实现一个无副作用的 `connectiontest.Probe`，并规范化外部错误。
4. 使用共享 Runner 执行，按业务语境映射本地化错误。
5. 贯通 Gin、API Proxy、Wails 绑定和表单交互。

### P1.5：服务端副作用、审批和恢复

- 增加 Approval 事实和 Tool Invocation 状态机。
- 绑定 `invocation_id + arguments_hash`，实现审批过期和防重放。
- 先以服务端可变更工具验证取消、重试、`uncertain` 和人工接管。
- 只有无法从业务事实安全重建中间状态时才接入 CheckpointStore。

验收边界：副作用得到确定结果或进入明确的 `uncertain/needs_review`，不会因 Task 重试被盲目重复执行。


#### 审批事实

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

#### 工具调用状态

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

客服场景的 `ask_customer` 与 `handoff_to_human` 是只登记意图的终止工具，运行过程中按普通工具调用展示参数；发送消息与改派负责人统一在终态事务中完成。


### 设备能力（P2／P3）

P2 在服务端 Agent 需要调用设备能力的真实场景出现后启动；用户在电脑前让助理读写本机文件、执行命令的需求由 P4 承担。

- 在 P4 已有的设备注册与撤销上增加 Capability Manifest；复用现有稳定 `device_id` 和 Realtime 请求头认证，不增加独立票据。
- 在服务端单体内实现 Device Capability Gateway，并与 Realtime Gateway 保持业务编排和传输职责分离。
- 增加 `device_invocations`、HTTP claim/progress/result，复用 P4 的设备 `work_seq` 与 `device_work_advanced` 水位通知。
- 桌面端 Go 侧持有设备事件流，通过 `appservice.Service(API Proxy)` 领取和提交调用并交给 Go Executor，Executor 实现本机二次校验和设备侧幂等；前端只承担审批界面。
- 首批开放文件选择上传、授权根元数据、只读 Git 状态能力。
- 客户会话继续默认禁用设备工具。

验收边界：事件流丢失事件或重连不会丢调用；非幂等调用结果未知时不会自动重放；设备撤销后不能继续领取调用。

#### P3：设备能力扩展

- 按真实场景增加 `client_work_items`、长任务恢复和结果对账。
- 支持用户明确选择多设备，不自动广播副作用。
- 扩展类型化文件用途和大结果文件引用。
- 移动端支持前台审批、文件和相册选择，不承诺无人值守执行。
- 出现第三方本地工具生态需求后增加设备侧完整 MCP Adapter。


#### 服务端与设备的分工

前期架构为：

```text
消息与业务触发
  -> 服务端 Agent Runtime
      -> 服务端业务工具
      -> Device Capability Gateway
          -> 持久化设备调用并登记通知
          -> 提交后发布 Core NATS
          -> Realtime Gateway
          -> 成员 SSE 事件流推送设备工作水位
          -> 客户端经 HTTP 领取并由 Capability Executor 执行
```

客户端 Capability Executor 不是 Agent：

- 不运行模型和 Eino。
- 不维护 Conversation 上下文。
- 不选择下一项工具。
- 只执行已经通过服务端策略和本机校验的具体调用。
- 负责本机权限、授权目录、审批界面、参数约束、结果裁剪和设备侧幂等。

执行命令、读写代码和操作界面属于高频本机工具循环，由助理在主人电脑上本地运行 Agent，见「P4：助理与桌面端本地运行」。Capability Executor 承担服务端 Agent 对设备的低频类型化调用，随 P2 在真实场景出现后落地。

设备调用能力从 P2 开始落地；设备注册先随 P4 本地运行落地。

#### 形态

Device Capability Gateway 与 Realtime Gateway 是两个不同职责：

- Device Capability Gateway 是服务端业务编排模块，负责能力策略、设备选择、审批和持久调用。
- Realtime Gateway 是现有的实时传输模块，负责事件流认证、JSON 事件、Core NATS 订阅、发送队列和背压。

第一版两者都在 Cervi Server 内运行。Device Capability Gateway 不建立第二个实时事件端点，不直接管理事件流，也不自行订阅 NATS。它负责：

- 解析 Agent Tool 请求为类型化 Capability。
- 计算企业策略、会话类型、Agent 策略、用户授权和设备能力的交集。
- 选择设备或返回需要用户选择。
- 创建持久设备调用和审批。
- 事务提交后发布设备工作水位通知，管理超时并通过 HTTP 接收结果。
- 把调用状态投影到 Tool Invocation 和审计链。

Gateway 不替代 Action 和 Executor 的安全校验，也不允许 Agent 直接持有连接编号。

#### 设备注册与认证

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

`capability_manifest` 是最近一次经过校验的设备能力广告，不是允许集；实际允许集仍为设备广告、企业策略、会话类型、Agent Tool Policy、本机授权和当前 Run 的交集。`work_seq` 是该设备待领取工作的最新水位，覆盖 P4 本地 Run 和 P2 设备调用。

P4 先落地 `id`、`organization_id`、`user_id`、`name`、`platform`、`work_seq`、`last_seen_at`、`revoked_at` 和时间戳，并按本地运行需要增加 `install_id`、`runtime_version` 和 `tool_manifest`；`trust_level` 与 `capability_manifest` 随 P2 增加。

认证要求：

- 复用 Realtime Gateway 的请求头 Bearer 认证，不创建第二套凭据通道。
- 设备事件流沿用成员事件流的请求头认证，并在请求头声明稳定 `device_id` 与 Executor 能力；服务端绑定企业、用户、`device_id` 和客户端种类，声明 Executor 能力时同时校验设备未撤销。
- 首次设备绑定由当前登录用户确认；设备信任和本机授权保存在设备记录及客户端安全存储中，不能只依赖请求头声明的能力。
- 登出、换服、切换账号、设备撤销和用户停用必须使相关设备权限失效。
- 桌面端 Go 侧持有设备事件流，通过本地 `appservice.Service(API Proxy)` 领取工作、交给 Go 侧执行并提交结果；API Proxy 从 Go `clientsession` 注入 Bearer Token，前端不接触原生端凭据。
- 前端只承担审批界面：Go 通过 Wails 事件请求审批，前端通过 Wails 绑定回传决定；服务端和 Go 侧执行方都校验目标 `device_id`。

#### 持久设备调用

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
  -> 创建 device_invocation 并登记通知
  -> 事务提交后发布用户 Subject，并携带目标 device_id 路由提示
  -> Realtime Gateway 只向匹配该 device_id 且声明 Executor 能力的事件流推送 device_work_advanced
  -> 桌面端 Go 通过 HTTP claim 领取完整调用
  -> Go Executor 执行，需要审批时经 Wails 事件请求前端确认
  -> Executor 持久化设备侧幂等状态
  -> 桌面端 Go 通过 HTTP 提交 progress/result
  -> Gateway 更新 Tool Invocation
```

`device_work_advanced` 事件随 P4 本地运行落地，在同一套实时事件定义中新增并可被旧客户端忽略，只发送给声明相应设备能力的事件流。它与变更通知共用事件流发送队列并按设备合并为最新水位，只携带设备编号和最新 `work_seq`，不携带工具名、参数或审批内容；不增加客户端持久命令。首版复用用户 NATS Subject，由各 Realtime Gateway 按已认证 `device_id` 过滤，不提前增加设备 Subject。

设备重连后按现有 Realtime 认证重新建立事件流，再通过 HTTP 比较工作 Head、补拉或领取调用；声明 Executor 能力的设备另按固定间隔经 HTTP 比较工作 Head，覆盖提交后发布丢失的通知；设备 Head 不进入聊天 `GetSyncHeads`。不能依赖 Gateway 重放事件。终态设备调用按保留策略清理，长期审计仍由 Agent Tool Invocation 保存。

#### 客户端 Executor

桌面端首期 Executor 只在应用进程存活且用户在线时承诺执行。首批能力限制为：

- 文件选择并上传为 Cervi `file_id`。
- 授权根目录内的文件元数据读取。
- 只读 `git status`、`git diff` 等明确能力。

不在首批开放通用 Shell、删除、任意路径读取、通讯录全量读取或相册全量扫描。

长命令、跨进程恢复或客户端已经执行但尚未上报的场景出现后，再以该能力驱动 `client_work_items` 和设备侧幂等账本。客户端不嵌入 NATS。

现有文件上传只支持头像用途。设备工具需要返回文件引用前，先扩展类型化文件用途、授权和激活关系，不能绕过文件模块直接写存储。

#### 桌面端与移动端差异

桌面端：

- 适合前台文件、Git 和授权目录能力。
- 应用退出后不承诺继续执行。
- 长任务需要 SQLite 工作项和明确恢复语义。

移动端：

- 默认只承担文件/相册选择、OS 权限确认和前台审批。
- iOS 挂起、Android Doze 和厂商进程限制下，不假设常驻实时事件流。
- 系统推送只负责提示用户打开应用，不保证无人值守执行。
- WorkManager、BGTaskScheduler 和系统传输能力只在真实后台场景出现后接入。

#### MCP 决策

本节只讨论设备能力协议。企业远程 MCP 服务已由服务端在 Run 内直接连接和调用；设备侧完整 MCP Adapter 仅在第三方本地 MCP 工具生态出现后落地。

P2 不直接采用 MCP subset 作为设备主协议，优先使用 Cervi 类型化的 HTTP invocation、claim、progress、result 和 cancel 契约；实时提示只扩展现有实时事件。

原因：

- MCP 不表达 Cervi 的企业、会话、Agent Run、设备寻址、审批、幂等和 `uncertain`。
- 只有 `tools/list`、`tools/call`、progress 和 cancel 不是完整 MCP Profile。
- 完整 MCP 还需要初始化、协议版本和能力协商；取消也只是尽力请求，不能作为副作用未发生的证明。
- 自定义实时 Transport 加不完整 MCP Profile，会同时承担自有协议和 MCP 兼容成本。

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


#### 替代架构及取舍

##### 客户端直接运行完整 Agent

优点是本地隐私、低延迟和离线能力更强，工具调用成为本机函数调用。执行命令、读写代码和操作界面属于高频本机工具循环，上下文位于本机工作区和屏幕，采用为 P4 助理的方案。企业模型供应商凭据、输入认领、最终消息、Run 终态和群聊轮转仍由服务端承担，客户端不复制审计与恢复机制；移动端不运行本地 Agent。

##### 设备直接暴露为端到端 MCP Server

优点是生态兼容。缺点是 MCP 不能替代 Cervi 的设备注册、授权、审批、幂等和不确定结果状态，最终仍需要 Gateway。现阶段不采用，未来作为 Executor 内部适配器。

##### Agent Worker 直接调用设备实时连接

组件最少，但会把设备寻址、授权、审批、连接状态和断线恢复散入 Eino Runtime，并使 Agent 持有瞬时连接。否决。

##### 持久调用 + 实时事件唤醒

相比同步设备 RPC 多一次持久化和领取请求，但能自然处理断线、重连、多实例、审计和 `uncertain`，并直接复用现有 Realtime Gateway、提交后通知发布、Core NATS、JSON 事件流与背压。采用为首选方案。

##### 服务端 Agent 经 WebSocket 反向通道调用本机工具

每次工具调用少一次领取往返，适合终端输出、截图流和即时取消等高频双向数据。多实例连接路由、断线后结果未知、审批和审计仍需持久调用承担，单次模型调用耗时仍远大于传输往返。用户在电脑前使用的场景采用「客户端直接运行完整 Agent」；远程指挥用户不在场的电脑出现真实需求后再评估。


### 群内协作扩展

| 能力 | 增加什么 | 不改什么 |
| --- | --- | --- |
| 话题 / 结构化协作（目标、参与范围、暂停继续、结论） | `conversation_topics` 表；Lane 与 Run 的 `scope_kind` 增加 `topic` 取值 | Lane、Input、Run 的结构与执行流程，活动运行唯一索引 |
| 话题之间并行发言 | 群 Policy 的 `loadMessages` 按话题收敛上下文，与上一行的 scope 取值一并生效 | 轮转规则本身，上下文契约 |
| 主持 / 编排式讨论 | 替换轮转的选择规则 | 结果协议与输入模型 |
| 工具审批与外部副作用 | P1.5 的审批事实与调用幂等 | 群协作的触发与写回 |
| 本机执行（助理在主人电脑上处理群内请求） | 助理只进入主人所在的群，主人为该群指定工作区；其他成员触发的写入与命令由主人审批；建立运行时写入助理绑定的电脑并由设备领取，见 P4 | 轮转规则、点名接力、轮次语义与执行互斥 |
| 定时或外部事件触发 | 一个写入 `agent_inputs` 的新入口，`kind` 增加取值 | 执行链路 |

`scope_kind` 是本模型的扩展点：执行范围从隐含的列组合变成显式的、可增加取值的维度，Lane 归属与执行互斥的粒度都随它收细。

并行与上下文收敛是同一件事的两面：话题之间并行的前提是各自的执行读取按话题收敛的历史。上下文仍覆盖整个群时，两个话题的执行读写同一段历史，重新回到非确定的交叉结果。

### 后续阶段须遵守的约束

1. 流式 token、progress tick、框架 callback 和调试日志不得写成永久语义步骤。
2. 外部副作用必须区分确定成功、确定失败和结果未知；结果未知时禁止盲重试。
3. Checkpoint 可以丢弃，消息、Run、工具调用和审批事实不可丢弃。
4. Device Gateway 是统一编排入口，但 Action、Gateway 和 Executor 必须分层校验。
5. 客户会话默认没有设备能力；按 Agent、会话类型、能力、设备和当前 Run 逐层放开。
6. 大文件和大结果只传文件引用，不进入实时事件载荷或模型上下文。
7. 移动端默认是前台确认与选择器，不承担无人值守企业 Worker 职责。
8. 助理与 AI 员工共用 Agent 身份、Revision、Lane、Input、Run 和最终消息幂等，不创建第二套身份、表或空字段；AI 员工只在服务端执行，助理只在绑定电脑上执行。
9. Device Capability Gateway 复用 Realtime Gateway 的事件流、认证、JSON 事件、通知发布、NATS 和背压，不建设第二套实时基础设施。
10. 本地运行的 Run 由服务端建立并指定执行设备，设备领取整个 Run 在本机执行；输入认领、最终消息、Run 终态、水位推进和群聊轮转在服务端事务提交，企业模型供应商凭据只保存在服务端。

### 暂缓决策与触发条件

| 暂缓能力 | 触发条件 |
| --- | --- |
| 完整触发模式默认值与消息合并 | 进入完整 P1 策略配置 |
| CheckpointStore | 业务事实无法安全重建 Runner 中间状态 |
| SessionStore | 单 Run 内存在明确的框架工作集或 Middleware 回放需求 |
| Automemory | 产品定义了跨 Run 的长期记忆及用户可管理语义 |
| BackgroundTask Store | 同 Run 子代理或长工具需要框架级租约 |
| `agent_run_events` | 必须跨断线逐事件回放，且日志系统不能满足 |
| 设备侧完整 MCP Adapter | 出现第三方本地 MCP 工具生态需求 |
| 客户端可靠任务 | 首个设备任务必须跨进程恢复 |
| 独立 Device Capability Gateway 服务 | 设备业务编排需要独立扩缩容；连接扩缩容继续由 Realtime Gateway 负责 |
| 多设备自动选择 | 产品已经定义可解释且安全的选择规则 |
| 移动端后台执行 | 存在系统允许且用户明确需要的真实后台任务 |
| 操作系统级沙箱 | 本地 Run 的命令执行需要审批之外的隔离 |
| 本地 Run 使用企业远程 MCP 服务 | 助理需要调用企业远程 MCP 工具 |
| 其他端实时查看本地 Run | 用户需要在 Web 或移动端观看本机运行过程 |
| 远程指挥不在场的电脑 | 用户需要从其他端让 AI 员工操作无人值守的电脑，届时评估 WebSocket 反向通道 |
| 本地模型与离线运行 | 企业要求模型和上下文不离开设备，或无网络时需完成 Agent 循环 |


### 实施 PR 检查表

进入每个 Agent 实施 PR 前检查：

- 当前修改属于聊天事实、Agent 业务事实还是任务基础设施，是否发生混用。
- 是否显式校验 `organization_id`、会话、Agent、Revision 和工具策略。
- 是否只以服务端持久输入和 `input_seq` 判断新触发，避免把 `originated_at`、UUID 或历史补拉当作触发水位。
- 是否可能因 Task 至少一次执行产生重复模型输出或重复副作用。
- 是否使用 `agent:<agent_run_id>` 收敛最终 Message，并覆盖 Provider 返回后、最终事务前崩溃和重复 Task 的竞争。
- calculator 是否已在正式发布前删除。
- 后台执行是否使用 `AgentExecutionContext` 显式授权，而不是伪造用户或继承触发用户的全部权限。
- 是否把流式事件、日志或 Eino 内部状态误写成永久 Step。
- 是否能区分确定成功、确定失败、取消请求和结果未知。
- 是否把大内容改成文件引用并设置大小上限。
- 是否把客户消息等不可信输入暴露给设备能力。
- 是否依赖实时事件、内存连接、移动后台或进程常驻维持正确性。
- 是否绕过统一 Realtime Gateway、请求头认证和现有事件定义建设第二套设备实时协议。
- 本地运行是否把企业模型供应商凭据下发设备，或把工作区真实路径写入服务端。
- AI 员工是否只在服务端执行，助理是否只出现在主人所在的会话并只派发到绑定电脑。
- 本地 Run 的最终消息、终态、水位推进和群聊轮转是否仍在服务端事务提交。
- 是否提前创建没有真实场景的表、字段、运行时或协议。
- 是否能在不改变聊天身份和业务事实的前提下替换或升级 Eino。


## 6. 知识库与检索

### 内容来源

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| OCR 与扫描版 PDF | PDF 按纯文本解析，无可提取文字的原件按 `empty_content` 失败 | 出现以扫描件为主的知识库来源；需要先确定本地或外部 OCR |
| 浏览器渲染抓取 | `webfetch` 不执行 JavaScript，依赖前端渲染的页面按 `empty_content` 失败 | 出现必须导入的客户端渲染站点。引入时只替换 `webfetch.Client.Fetch` 的实现并增加对应配置，快照语义与处理分支不变 |
| 站点批量抓取与链接跟随 | 网页导入按单个页面地址创建单份文档 | 出现整站或整个帮助中心导入的需求；需要先确定范围边界、来源标识与完成状态 |
| 定时重抓 | 网页文档由成员显式「重新抓取」更新快照 | 出现页面更新频繁且人工重抓成本明显的场景；需要先确定调度粒度与失败通知 |
| 登录态或鉴权页面 | 抓取只发送固定 User-Agent，不携带凭据 | 出现企业内部需要鉴权的文档站点；需要先确定凭据保存与企业隔离方式 |
| 音频与视频转写 | 包装服务对 `.wav`、`.mp3`、`.m4a`、`.mp4` 直接返回 `unsupported_file`，`markitdown[all]` 自带的转写调用 Google 识别服务 | 出现音视频资料入库需求；实现时改为自托管 ASR |
| YouTube 来源 | 对应转换器只接受 URI，包装服务只接受上传的字节 | 与站点抓取一并评估；需要在包装服务中单开接口并明确出网边界 |
| Azure Document Intelligence 与 Content Understanding | 镜像装有依赖，不开放入口 | 与自托管定位冲突，暂不启动 |
| 在线文档图片 | 编辑器不注册图片节点，粘贴与拖入的图片不进入文档 | 出现图文混排的资料录入需求；需要先确定图片存储与召回中的表达 |
| XLSX 数值改写 | 单元格经 pandas 类型推断，编号前导零与金额小数尾零丢失，检索按改写后的字面值进行 | 认定改写不可接受时，把该格式的分派改回 Go 侧 excelize，只影响 `ProcessDocumentAction` 中一处按扩展名的分派 |

### 召回与检索

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 名称类短字段检索 | 文档名、问答问题等短字段使用 `ILIKE` | 列表数据量增大后为高频列加 `pg_trgm` GIN |
| 稀疏向量召回路 | 召回由 pgvector 稠密向量与 `tsvector` 词法两路融合 | 出现稀疏向量模型服务可用的部署形态；OpenAI 兼容接口当前普遍不提供 |
| 词法路近似排名 | GIN 命中后至多取 2000 条候选按 `ts_rank_cd` 排名，超过上限时为近似排名 | 单库分段规模使该上限明显影响召回质量 |
| 查询性能索引 | 迁移只保留主键和用于业务约束、幂等及并发正确性的唯一索引 | 上线前统一评估并集中补齐 |

### 向量存储

分段向量存放在同库 `public.knowledge_segments` 的 halfvec 列，命中按维度建立的部分索引。引入 Qdrant 的触发条件是单企业分段规模进入千万级，或确认需要 sparse 与 dense 原生混合检索；在此之前跨库双写会破坏删除文档时的事务一致性，并把企业隔离从 SQL 条件降级为 payload 过滤。

`internal/actions/knowledgebase/segment_store.go` 与 `knowledgeretrieval.Source` 共同构成切换向量存储的唯一改动面。

### 部署与安全

- markitdown 镜像随 Release 发布到 `ghcr.io/<repo>-markitdown`，标签与服务端镜像一致。需要在 `internal/server-deployment.md` 中说明该镜像的拉取、版本对齐和 `MARKITDOWN_URL` 配置。
- 网页抓取不拦截内网地址与私有网段，自托管的企业内网文档站点是该能力的实际来源之一。SSRF 与抓取频率控制列入上线前的安全与容量专项。
- 知识库的可见与编辑范围随角色权限统一建设，当前只校验已登录。

### 与其他能力的衔接

- 跨 Run 长期记忆在出现明确产品形态后设计，复用本地知识库的转换与向量管线及后台模型配置，见「暂缓决策与触发条件」。
- 助理的本地 Run 通过服务端检索 Revision 绑定的知识库，随 P4 交付。

### 全文检索

| 事项 | 当前行为 | 启动条件 |
|---|---|---|
| 名称类短字段的 `pg_trgm` GIN | 成员、联系人、团队、Agent、文档名、问答问题和会话名称均为 `ILIKE` 顺序匹配 | 单表规模使名称搜索耗时成为实际问题时；随上线前的查询性能索引评估一并处理 |
| 词元规则变化后的重新生成 | `searchtext` 没有规则版本号；规则变化后已写入的 `search_vector` 保持旧词元，知识库可通过重建索引刷新，聊天记录没有重新生成入口 | 词元规则发生影响已有数据命中的变化时 |
| `search_vector` 加入前的历史消息 | 没有词元，不在检索范围内 | 出现需要检索历史消息的存量部署时 |
| 消息与人员分组的「查看全部」 | 分组达到 6 条时入口保留显示并禁用；会话分组已由会话名称搜索分页启用 | 出现需要翻阅全部消息或人员命中的使用场景 |
| 目标规格上的复测 | 实测数据来自本地磁盘与预热缓存、合成语料 | 准备正式上线时，在目标托管规格上用真实语料复测耗时与统计目标 |
| 聊天记录检索的语句超时与 GIN 探测 | 查询不设置单独超时 | 数据规模增长后实测计划出现退化时 |

## 7. 平台基础与上线前专项

### 公开访客端点加固

公开访客端点的安全加固不作为聊天主流程功能的前置条件；准备正式上线时集中处理，覆盖以下边界：

- 限制单个渠道身份同时保持的未回复 Conversation 数量，超出时拒绝创建新线程并提示继续已有线程。
- 渠道停用即拒绝公共访问是紧急止血手段；限速和配额参数由渠道配置管理，不硬编码。


- 访客上传端点的频率限制与配额；附件内容的病毒扫描与敏感内容审核。

### 上线前统一专项

- 查询性能索引：迁移当前只保留主键和用于业务约束、幂等及并发正确性的唯一索引，上线前统一评估并补齐。托管方案新增的开通幂等、外部身份、成员邀请与权益表一并评估。
- 密钥加密存储与接口响应脱敏，范围包含托管部署的运营服务凭据与部署级对象存储凭据。
- 原生端 Bearer Token 从 SQLite 明文存储改为 iOS Keychain、Android Keystore 和桌面端安全凭据存储。

### 官方托管服务

Cervi 在自托管之外提供官方托管。用户在官网注册账号、创建企业并选择域名前缀，企业地址为 `https://<企业前缀>.cervi.runforyou.app`。两种部署方式共用同一套服务端与各端业务代码，由 `self_hosted` 和 `managed` 两个部署模式区分初始化、成员加入和登录方式。完整契约、数据模型和验收见 [官方托管服务建设方案](saas-hosting-plan.md)。

**首期范围**

官网注册与企业开通、官方账号登录、成员邀请、订阅有效期展示、客户端下载与连接；运营侧覆盖企业查询、基础资料、权益同步、停用恢复与删除。自动支付、自定义域名、多地域调度和细粒度运营权限属于后续。

**系统边界**

官方 SaaS 独立仓库、独立数据库，承担官方账号、订阅计费和运营后台，通过 cervi-server 的 `/operator/v1` 运营 API 开通和管理企业。cervi-server 只有一个 HTTP 监听地址，运营路由按路径前缀与企业业务共用该监听，只在 managed 模式下注册，访问控制由独立的运营凭据承担。契约上运营接口使用独立的 `OperatorBackend` 和独立生成目标，`Backend` 仍是企业业务调用的唯一契约源。

**关键决定**

- 首期一套部署承载多个企业，按域名和企业 ID 隔离，共用 PostgreSQL、NATS 与文件存储。泛域名解析与证书由入口统一维护，服务端使用 `TLS_MODE=external`。
- 官方身份采用 OIDC Authorization Code Flow 加 PKCE，外部身份绑定按 `organization_id + issuer + subject` 唯一。授权尝试带用途，`login` 校验已有绑定，`accept_invitation` 在同一次交换内完成接受邀请与企业 Token 签发。企业 Token 仅由 cervi-server 签发，客户端不持有官方身份令牌。
- 成员邀请绑定受邀邮箱，接受时校验官方身份的已验证邮箱一致。首期不引入邮件服务，管理员复制邀请链接自行转达。自托管保留本地密码账号创建，两种模式各自只有一条成员创建路径。
- 服务权益以企业为单位覆盖写入，版本单调，乱序与重试按版本判定。首期不按席位售卖，权益快照不含成员数与存储容量上限。
- 企业开通、权益同步和删除都走持久化操作记录与幂等调用，中断后可从原标识恢复。SaaS 恢复备份会使版本回退，需按方案的对账流程读取已应用版本再以更高版本下发。

**阶段划分**

企业与运营契约 → 官方身份接入 → 成员邀请闭环 → 官网开通闭环 → 服务管理 → 客户端与交付。阶段三之后托管企业具备多人使用能力，阶段四提供内部试用闭环，公开发布以最后两个阶段的验收完成为条件。

**对本文其他事项的影响**

| 影响点 | 说明 |
| --- | --- |
| 角色权限 | 托管方案的成员邀请由管理员发起，运营成员摘要返回角色类别。当前只校验已登录，邀请入口的权限判定随角色权限一并建设 |
| 公开访客端点 | 托管企业共用父域，访客 Cookie 按访问协议改用 `__Host-` 前缀形态；该改动在托管方案内完成，不等待本节的访客端点加固 |
| 客户端可靠任务 | 托管客户端的连接与授权不进入客户端任务队列；原生端凭据安全存储与本文同一条目共用 |
| 对象存储 | 两种部署模式都使用部署级存储配置并按企业编号隔离对象键，托管部署由平台运维该配置 |

SaaS 使用 Laravel 并承担官方身份服务，官网与账号入口使用企业后缀之外的域名；首期商业规则等其余定稿项在对应阶段实施前确定，见该方案的实现前定稿项。

### 角色权限

已有角色权限参与鉴权，并增加会话、报表与知识库的数据范围（全部、本团队、本人）；当前只校验已登录。

### 客户端可靠任务

#### 当前状态

客户端暂时没有需要跨进程恢复的异步任务，本目录只记录既定方案，不提供空壳运行时。出现首个真实场景后，再以该场景驱动数据模型和实现。

普通页面异步操作继续使用前端异步调用或 Go `context` 与 goroutine；只有断网、应用退出或系统终止后仍需恢复的工作才进入客户端可靠任务队列。

#### 能力边界

| 工作类型 | 执行位置 | 运行机制 |
| --- | --- | --- |
| 页面生命周期内的短任务 | Web、桌面端、移动端 | 前端异步调用或 Go goroutine |
| 需要本机恢复的工作 | 桌面端、移动端 | SQLite 持久化队列与进程内执行器 |
| AI Agent、工作流和企业后台任务 | 服务端 | PostgreSQL、NATS JetStream 与服务端调度器 |

客户端不嵌入或直连 NATS，不承担企业级常驻 Worker 职责。Web 端不实现客户端任务队列，需要可靠执行的工作直接提交给服务端。

#### 代码边界

后续实现放在 `internal/task/client`，不放入 `internal/task/server`，也不复用服务端 Runtime、仓储或调度模型。

```text
internal/
├── task/
│   ├── task.go          # 平台共享的最小 Action 语义
│   ├── client/
│   │   ├── runtime.go   # 单进程执行器与 Wake 入口
│   │   ├── registry.go  # 客户端 Action 注册
│   │   ├── repository.go  # SQLite 状态转换与崩溃恢复
│   │   ├── retry.go     # 客户端错误分类与退避
│   │   └── options.go   # 分组、去重和执行选项
│   └── server/          # 服务端 PostgreSQL、NATS 与 Cron 实现
└── storage/
    ├── desktop/         # 桌面端模型与 SQLite 迁移
    └── mobile/          # 移动端模型与 SQLite 迁移
```

`task` 根包只保留确实跨平台的 `Handler` 和永久错误等语义。队列、触发来源、Cron 定义和服务端投递选项属于 `task/server`。客户端定义自己的注册和入队选项，不与服务端结构保持形式上的一致。

客户端与服务端都采用 Action 模式，但分别注册可在本平台执行的 Action。允许名称或输入契约相同，不共享依赖平台存储的 Handler 实例。

#### 执行模型

客户端 Runtime 保持一个进程内 dispatcher，并对外提供统一唤醒入口：

```go
Wake(ctx context.Context, budget time.Duration)
```

应用启动、恢复前台、网络恢复、桌面端计时器及移动端系统后台回调都通过该入口执行到期工作。一次唤醒只在给定时间预算内领取任务，不创建第二个独立执行器。

第一版使用单 Worker。启动时把上次异常退出遗留的 `running` 工作恢复为 `queued`，通过单次 Action 超时处理卡住的执行，不实现数据库租约、心跳或多 Worker 抢占。

#### SQLite 模型

桌面端和移动端在各自迁移目录创建同构的 `client_work_items` 表。字段以首个真实场景为准，目标模型包含：

| 字段 | 职责 |
| --- | --- |
| `id`、`action_name`、`payload` | 标识 Action 及其 JSON 输入 |
| `status` | `queued`、`running`、`retrying`、`succeeded`、`failed`、`cancelled` |
| `priority`、`available_at` | 控制领取顺序和延迟重试 |
| `attempt`、`max_attempts` | 限制可恢复业务错误的重试次数 |
| `group_key` | 保证同一业务对象串行执行 |
| `coalesce_key` | 合并尚未执行的同类刷新工作 |
| `idempotency_key` | 防止同一业务命令重复提交 |
| `cancel_requested` | 支持协作式取消 |
| `last_error` | 保存最近一次可诊断错误 |
| 创建、开始、完成和更新时间 | 支持恢复、清理和必要的状态展示 |

`payload` 不保存大文件内容。需要恢复上传时，先把文件复制到应用数据目录，任务只记录受管理的本地路径。终态记录按保留策略清理；需要长期防重的业务命令由服务端业务唯一键或幂等记录保证。

SQLite 由客户端存储与 Runtime 共享同一数据库连接。引入后台 goroutine 时限制连接并发，避免同一客户端内出现多个 SQLite 写入者。

#### 顺序、幂等与重试

- 不保证所有工作全局 FIFO；默认按 `priority`、`available_at` 和 `created_at` 领取。
- 相同 `group_key` 严格串行，不同分组以后可按平台能力有限并发。
- 合并刷新、替换待同步数据和业务幂等是不同语义，不能共用一个键。
- Action 必须可重入，因为进程可能在服务端已成功、客户端尚未落库时退出。
- 无网络时暂停派发，网络恢复后重新唤醒；离线不消耗业务重试次数。
- 连接失败、超时和服务端临时错误使用指数退避。
- 认证失效时暂停需要认证的工作，等待重新登录，不持续重试。
- 明确的业务错误标记为永久失败。
- 用户取消只设置取消标记并通知执行上下文，不直接删除执行中的记录。
- 已提交到服务端的任务通过服务端接口取消，修改客户端记录不能替代服务端取消。

任务必须绑定企业服务器和用户范围。登出、更换服务器或切换账号时，取消或清理不再属于当前会话的工作，禁止使用新身份继续提交旧工作。

#### 服务端前置能力

客户端重试写请求前，相关 HTTP 接口必须支持持久化幂等键或等价的业务唯一命令标识。服务端任务自身的幂等不能替代 HTTP 接收入口的幂等。

原生端 Bearer Token 由 Go 会话管理器持有并暂存于客户端 SQLite。客户端 Action 不保存或接收 Token，只按企业服务器、组织和用户范围领取工作，执行时通过会话管理器取得当前有效凭据。

SQLite 过渡实现以明文保存 Token，只依赖应用数据目录和数据库文件权限。正式发布前改用 iOS Keychain、Android Keystore 和桌面端安全凭据存储。Token 不得写入任务载荷、日志、错误详情或诊断导出。

#### 平台唤醒

桌面端在应用进程存活时通过计时器、恢复前台和网络变化调用 `Wake`。应用退出后不承诺继续执行。

Android WorkManager 和 iOS BGTaskScheduler 只负责尽力唤醒应用并提供执行窗口，SQLite 才是工作状态的唯一来源。移动系统可能延迟或取消后台执行，因此本地任务不承担准时调度和无人值守的企业业务。

用户可见的大文件后台上传按平台使用系统传输能力，例如 iOS Background URLSession 或 Android 合规的前台服务，不依赖被冻结的 Go HTTP 请求。精确定时提醒使用系统通知；可靠定时业务交给服务端。

#### 实施顺序

1. 确定一个必须跨进程恢复的真实客户端 Action。
2. 为该 Action 调用的服务端写接口补齐 HTTP 幂等。
3. 在桌面端和移动端分别增加 SQLite 迁移，实现单 Worker、崩溃恢复、退避和取消。
4. 先验证前台运行、断网恢复、杀进程恢复、重复提交、登出和换服。
5. 正式发布前把 Go 会话管理器的 SQLite 凭据存储替换为平台安全存储。
6. 最后接入 WorkManager、BGTaskScheduler、网络恢复和平台传输能力。

在出现真实需求前，不引入客户端 Cron、数据库租约、outbox、多队列、多 Worker、嵌入式消息代理或第三方服务端任务框架。

