# Cervi 聊天与协作路线图

## 1. 文档目的

本文档确定 Cervi 从客户会话、企业内部聊天、第三方账号接入，逐步扩展到受管外部协作、工单和跨企业联邦通信的产品路线与核心模型。

本文档最初定义了前两个聊天 PR 的实施边界；两个 PR 已合并，第 13 章记录其交付基线和后续交付清单。后续开发如需改变本文中的核心概念、对象边界或阶段顺序，应先更新本文档并说明原因。

本文档还确定客户端同步和实时传输的长期边界，包括 WebSocket、Protobuf、HTTP、PostgreSQL Outbox、Core NATS 与 JetStream 的分工，避免聊天开发后再用多套协议补洞。

聊天重构与实时能力按 [PR 实施清单](chat-realtime-pr-plan.md) 交付。PR01 以 `707fdff` 为代码核对基线，仅对齐本文及 Agent 路线图；下文的统一消息顺序、同步、列表和置顶规则是待实施目标，第 13 章保留各次交付时的历史边界。当前仍使用群聊序号、其他会话来源三元顺序和前台轮询；不把目标规则视为已完成。

第 7 至第 10 章以及后续阶段中的状态机、协议、表字段和基础设施设计，作为对应阶段的实现基线。进入实现前，结合目标平台官方能力、当前代码和容量验证做增量修正。

路线阶段只定义验证和实现顺序，不提前创建远期表、依赖或接口。

## 2. 产品背景与约束

Cervi 是 AI 原生企业协作产品，以企业独立部署为主，但独立部署不等于只能在内网运行。企业服务器可以通过公网域名服务 Web、桌面端、移动端、外部访客和渠道回调，同时保持企业间的数据边界。

前期重点场景：

- 网站、微信公众号、Telegram Bot 私聊等渠道作为客服消息来源；Bot 群、频道和讨论组不进入一对一客户会话。
- 企业内部成员单聊和群聊。
- AI 智能体以与成员一致的方式加入单聊、群聊和客户会话。
- 企业成员将已有 Telegram 用户账号绑定到 Cervi，在 Cervi 中逐步接管联系人、单聊、群聊和消息处理。

中期重点场景：

- 未部署 Cervi 的供应商、承包商或客户，通过企业托管的邀请链接和外部协作门户参与单聊、群聊和工单。
- 客户会话和供应商协作中的问题可以转成工单持续处理。
- 在 Telegram 验证成熟后，按第三方平台能力继续接入其他个人消息账号。

长期重点场景：

- 两个或多个独立部署 Cervi 的企业建立可信连接。
- 各企业成员使用自己企业中的原生身份进行跨企业单聊和群聊。
- 各部署在保持本地身份、权限、审计和数据策略的前提下同步共享会话。

当前阶段不考虑历史接口和历史数据兼容。模型直接采用目标结构，但不提前创建尚未进入开发阶段的第三方账号、联邦和 AI 运行表。

## 3. 产品设计结论

### 3.1 统一消息容器，不堆叠产品概念

Cervi 底层使用统一 `Conversation` 承载消息时间线，但首期只向用户暴露熟悉的概念：

- 单聊
- 群聊
- 客户会话

公开群、话题模式和独立讨论空间属于后续增强能力。普通会话中的引用回复和话题串通过消息关系表达，不创建新的会话类型。

### 3.2 “渠道”只表示客服外部消息来源

Cervi 中的“渠道”仅表示网站、微信公众号、Telegram Bot 私聊等客服外部消息接入源，不表示企业内部聊天室，也不表示成员绑定的第三方用户账号。

以 Telegram 为例，同一个平台存在两条必须分开的接入路径；其他平台也按“企业客服入口”和“用户消息账号”分别归类：

| 接入方式 | 身份所有者 | 产品用途 | 归属模型 |
| --- | --- | --- | --- |
| Telegram Bot | 企业 | 接收客户私聊咨询、客服回复 | `channels`、渠道身份、客户会话 |
| 第三方用户账号（Telegram 首个实现） | Cervi 用户 | 在平台授权范围内处理个人联系人、单聊和群聊 | Provider Connection、用户消息账号、连接会话扩展 |

两条路径可以共享通用会话与消息展示能力，但不能共享账号、授权、幂等键或远端会话映射。

首期客服 `customer` 会话只表示企业与一个渠道联系人之间的一对一会话。Telegram Bot 首期只接私聊；Bot 所在群、频道和讨论组不是客户会话。以后如需接入，使用 `group` 会话及多个 `contact` 或 `external_sender` 主体，不复用 `customer_conversations`。

### 3.3 会话形态与来源正交

会话类型只表达沟通形态：

```text
direct    真人单聊
agent     独立 AI 聊天
group     群聊
customer  客户会话
```

内部、受管外部、第三方账号和联邦通信通过参与主体及扩展表表达，不增加 `telegram_group`、`cross_org_direct` 等类型，也不在 `conversations` 上增加不断扩张的 `source_type`。

扩展关系示例：

- 存在 `customer_conversations`：客户会话。
- 存在未来的 `connected_chats`：某个第三方用户账号下的远端会话视图。
- 存在未来的联邦会话扩展：跨 Cervi 部署共享的会话。
- 不存在来源扩展：Cervi 原生会话。

一个 `Conversation` 最多关联一种来源扩展：`customer_conversations`、未来的 `connected_chats` 和联邦扩展互斥。`conversations.type` 创建后不可修改，避免扩展关系与参与者规则失真。

Cervi 原生 `direct` 会话仅用于真人之间，采用“一对允许单聊的内部身份对应一个长期会话”的产品语义。当前已由 `direct_conversations` 保存按身份编号规范化的 `first_identity_id / second_identity_id`，并以企业内身份对唯一约束保证并发首发收敛；ChatSubject 与 Participant 继续承担发送主体和参与关系。首发通过 `SendFirstDirectTextMessageAction` 建立关系，唯一冲突后重试读取同一会话，不使用 advisory lock。

### 3.4 第三方账号会话按账号视图隔离

一个 Cervi 用户可以绑定多个第三方平台账号，包括同一平台的多个账号，并在同一个聊天列表中同时查看和回复这些账号的消息。每条会话必须清晰展示由哪个账号接收和发送。

安全默认模型是“一个绑定账号下的一个远端会话，对应一个 Cervi 会话”，不跨账号自动合并：

```text
(messaging_account_id, provider_chat_id)
    -> connected_chat
    -> conversation
```

即使两个 Telegram 账号都加入了名称相同或远端编号看似相同的群，也保留两个账号视图。原因包括：

- 私聊和基础群的标识及历史可见范围可能随账号不同。
- 同一群中两个账号的权限、禁言、匿名发送和删除状态可能不同。
- Telegram 的“为我删除”、草稿、未读、置顶和静音本来就是账号状态。
- 自动合并会造成用错账号回复或越权展示历史。

前端以后可以提供经过明确校验的聚合展示，但存储和发送路由不能依赖这种聚合。

### 3.5 公开性和消息组织方式使用正交属性

群聊未来可按需增加：

```text
visibility = invite_only | organization_discoverable
mode       = chat | topic
```

- `invite_only` 是默认方式。
- `organization_discoverable` 表示企业内可发现的公开群。
- `chat` 表示普通时间线。
- `topic` 表示按话题聚合内容。

### 3.6 实时通道与业务 API 分工

Cervi 的 Web、桌面端、移动端和网站挂件共用一个版本化 WebSocket 实时协议，使用 Protobuf 二进制帧承载同步水位、临时状态、AI 流和 WebRTC 信令。SSE、Long Polling、Mercure 和 WebTransport 不作为并行主协议；网站挂件如果在统一 Gateway 落地前需要工作，只能暂时使用普通 HTTP 轮询。

WebSocket 不是第二套业务 API。发送、编辑、撤回、回执、成员和设置变更、AI 运行控制等持久命令统一经过 `appservice.Service`，并复用 Action、事务、错误和幂等体系。服务端 Web 使用 `DirectBackend`，桌面端和移动端使用 API Proxy，网站挂件和渠道回调由 Gin 做外部 HTTP 适配；具体传输不能绕开统一应用服务。音视频媒体走 WebRTC，附件走 HTTP 和对象存储，应用退出后的唤醒走系统推送。

## 4. 身份边界

### 4.1 业务身份保持独立

不同身份模型解决不同业务问题，不创建一个可以同时承担登录、团队成员、CRM 联系人、远端账号和聊天发送者职责的全局 `actors` 表。

- `organization_identities`：企业内部的统一身份，目前只包含 `user` 和 `agent`。
- `users`：可登录的本地企业成员。
- `agents`：AI 智能体定义。
- `contacts`：CRM 外部联系人档案。
- 未来 `guest_users`：当前企业托管的外部协作者。
- 未来 `federated_users`：远端 Cervi 身份的本地投影。
- 未来 `external_senders`：第三方平台中实际发送消息的远端主体。

`organization_identities` 继续服务于企业通讯录、团队成员和工作状态，不加入联系人、访客、联邦用户或 Telegram 远端发送者。

### 4.2 ChatSubject 是聊天边界内的统一主体

聊天需要用一套稳定关系表达参与者、发送者、@ 提醒和表情反应，因此增加仅在聊天域内使用的 `chat_subjects` 注册表：

```text
chat_subjects
├── id
├── created_at
├── updated_at
├── organization_id
├── kind
└── source_id
```

首期支持：

```text
organization_identity   本地用户或 AI 智能体
contact                 CRM 外部联系人
```

未来按实际阶段增加：

```text
guest                   受管访客
federated_user          远端 Cervi 用户投影
external_sender         第三方平台发送者
```

同一企业内 `(kind, source_id)` 唯一。Action 在创建或读取 `chat_subjects` 时，必须按 `kind` 校验来源记录存在、属于同一企业且类型合法。

`ChatSubject` 只表示“这个主体可以出现在聊天关系中”，它本身：

- 不能登录。
- 不能加入企业团队。
- 不保存 CRM 字段。
- 不保存第三方发送路由。
- 不决定资源访问权。

这种边界既让参与者和消息只有一个稳定外键，又避免污染现有通讯录和联系人模型。

会话形态与主体类型的允许关系由 Action 显式维护：

| 会话场景 | 允许的聊天主体 | 额外规则 |
| --- | --- | --- |
| 一对一客户会话 | 恰好一个有效 `contact`；回复后可加入 `organization_identity` | 客服和 AI 未回复、未协作时不自动成为参与者 |
| Cervi 原生单聊或群聊 | `organization_identity`，以后可增加 `guest`、`federated_user` | `contact` 和 `external_sender` 不直接混入原生内部会话 |
| 第三方账号会话 | `external_sender` 与账号所有者的 `organization_identity` | 参与者不代表远端 ACL，访问仍校验账号所有权 |

主体类型与会话场景不匹配时拒绝写入。以后新增来源类型时先扩展该矩阵，不通过放宽所有会话的参与者规则实现。

### 4.3 系统消息不是伪造主体

`system` 是逻辑消息来源，不创建 `chat_subject`，也不加入参与者表。系统消息的 `sender_participant_id` 为空，由消息类型和事件载荷表达来源。

### 4.4 同一自然人的多个身份不自动合并

同一个自然人可能同时是企业成员、联系人、受管访客、联邦用户或多个 Telegram 账号中的远端主体。系统不按手机号、邮箱、用户名或显示名称自动合并。

需要聚合展示时使用经过验证的独立关联记录，并保留每条历史消息原始发送主体和账号边界。

### 4.5 网站匿名访客使用长期渠道 Cookie

网站独立接待页和嵌入挂件使用服务端生成的渠道级不透明 token 恢复匿名身份；企业成员仍使用 Bearer Token。

- Cookie 名称为 `cervi_visitor_<channel_id>`，每个网站渠道独立；值为 16 个随机字节编码成的 32 位小写十六进制字符串，不能由业务编号推导。
- Cookie 有效期为 365 天，不滑动续期；设置 `Path=/`、`HttpOnly`，不设置 `Domain`。同一个公开初始化接口无法稳定区分独立页和 iframe，因此所有 HTTPS 请求统一使用 `SameSite=None; Secure`，所有 HTTP 请求使用 `SameSite=Lax` 且不设置 `Secure`。反向代理后的协议判断使用可信代理提供的外部协议，不只检查 `Request.TLS`。
- 状态初始化请求只在 Header、Cookie 优先解析得到的 token 缺失或非法时生成 token，写回带 365 天有效期的 Cookie 并在响应中返回。已有合法 token 不重写长期 Cookie，不刷新过期时间；Header 与 Cookie 不一致时 Header 优先，但不为了收敛值滑动 Cookie。此时不创建联系人、渠道身份或会话；第一条有效文本消息才在同一事务创建业务记录。
- 页面只在内存中保存响应 token；需要时通过 `X-Cervi-Visitor-Token` 回传，服务端按 Header、Cookie 的顺序解析。token 不写入 `localStorage`。
- 匿名身份规范化为 `contact_channel_identities.external_id = web-session:<token>`；查找同时校验企业和渠道。客户端消息编号独立承担消息幂等。
- Cookie 过期、被清除或不可用时建立新的匿名身份；渠道停用后拒绝公共访问。

> [!WARNING]
> Header 回传只维持当前页面内的嵌入访问；浏览器完全阻止第三方 Cookie 时，不承诺跨刷新恢复。阶段 1A 验证独立页和 HTTPS 挂件的首次访问与刷新恢复。

公开访客端点的安全加固按当前阶段约束后置，不作为聊天主流程功能的前置条件；后续集中处理时覆盖以下边界：

- 初始化、发送和历史接口按来源 IP 和渠道限速；发送接口同时按访客身份限制消息长度和发送频率。
- 限制单个渠道身份同时保持的未回复 Conversation 数量，超出时拒绝创建新线程并提示继续已有线程。
- 渠道停用即拒绝公共访问是紧急止血手段；限速和配额参数由渠道配置管理，不硬编码。
- 阶段 1A 已上线的公开端点保持现状，直到安全加固阶段统一补齐上述限制。

挂件接入 Realtime Gateway 后，公共换票端点校验该访客身份并签发第 10.12 节定义的短期一次性连接票据；Cookie 不直接用于 WebSocket 认证。

实时连接时机按业务身份是否存在决定：已有渠道身份的访客初始化后可换票并连接；只有 Cookie/token、尚无渠道身份时不换票、不连接。第一条有效消息提交并建立身份后再换票连接。另一空白页面通过恢复前台和低频、无副作用的身份检查发现同渠道 Cookie 对应的新身份，先读取目录基线再追赶；检查不得创建联系人、渠道身份或会话。挂件预览不连接真实服务。

## 5. 核心业务对象

```text
业务身份 ── ChatSubject ── ConversationParticipant ── Conversation
                                      │                    │
                                      └── Message.sender ──┤
                                                           ├── CustomerConversation
                                                           ├── ConnectedChat（未来）
                                                           └── FederationConversation（未来）

CustomerConversation ── ServiceSession ── 客服排队、分配和指标

Ticket ── TicketConversationLink ── Conversation / Message 范围
```

### 5.1 Conversation

`Conversation` 是稳定的消息容器，负责会话形态、标题、群聊配置、参与者集合、消息时间线、归档状态和当前企业中的数据归属。

它不负责客服排队、工单流程、外部渠道配置、第三方账号授权、用户登录或 AI 模型调用状态。

### 5.2 ConversationParticipant

参与者关系表示一个 `ChatSubject` 显式加入某个会话，并保存会话角色、加入和离开时间。身份来源只通过 `subject_id` 访问，不再保存 `identity_type + identity_id` 多态组合。

参与者采用“一主体在一个会话中一行到底”的模型。退出时只设置 `left_at`；重新加入时复用原行、清空 `left_at`，保留首次 `joined_at`，并把 `role` 更新为当前角色。成员期和角色变更历史进入审计记录，不在参与者表中复制多行。发送新消息要求 `left_at IS NULL`，历史消息始终指向原参与者行。

参与者和主体来源记录不因退出、停用或远端失联而被物理删除，避免历史消息失去发送者上下文。

参与者不保存个人视图状态。已读、置顶、归档和静音具有不同的所有者：

- Cervi 原生用户视图：已使用 `conversation_user_states` 保存阅读、静音和手动未读；个人置顶 rank 与用户水位行上的顺序版本由 PR47 增加。
- 第三方账号视图：未来使用 `conversation_account_states`。
- AI 处理进度：单聊与网站客服已使用 `conversation_agent_states`；群聊尚无 Agent Trigger／Run。

三者不能共用一个 `last_read_message_id`。

参与者关系表达显式成员和消息发送主体，不是所有会话的唯一列表授权来源：

- 原生单聊和群聊按有效参与者授权。
- 客户会话先按企业边界读取，`ServiceSession` 落地后按排队、负责人和协作者授权。
- 未发言、未分配的客服不因能查看客户收件箱而写入参与者表。
- 客服或 AI 实际回复或明确加入协作时，才建立参与者关系。
- 第三方账号会话还必须校验绑定账号所有权和连接状态。

### 5.3 Message

消息属于一个会话，由该会话中的参与者发送。首期只实现文本消息，但保留引用、话题、编辑和删除关系。

消息的本地顺序、来源时间和同步版本各自独立：

- `message_seq` 是单会话内最终顺序，由锁定的 `conversations.last_message_seq` 在消息事务中分配；真人、渠道、系统消息和 AI 最终回复共用分配器。
- `originated_at` 表示业务来源时间，`source_order` 保留平台提供的来源顺序；两者用于展示和来源诊断，不决定 Cervi 时间线、分页或阅读位置。网站在取得业务锁后生成本地时间，Telegram 保留远端时间。
- `created_at` 是当前服务器入库时间，用于审计和同步诊断。
- `sync_seq` 是会话可恢复变化版本，消息、资料和 Run 终态都可推进，不等于消息序号或未读数量。

晚到的外部消息排在本地接收位置，显示原来源时间，不插入已经读过的历史位置。`before`、`after`、引用上下文、普通已读和提及比较统一使用 `message_seq`，游标绑定会话与原始序号，响应按正序返回。HTTP 序号使用十进制字符串，TypeScript 使用 bigint；UUID 不承担消息排序。

会话内序号由唯一约束保证。事务回滚允许未提交编号重用，已提交分配器不得因摘要重算或删除消息而回退。消息追加、Conversation／ServiceSession 摘要、Agent Trigger 和任务投递在同一事务提交；幂等命中返回已有事实，不再次分配或触发。摘要可以按现存消息重算，预览不得暴露已删除内容；普通列表独立使用 `lastActivityAt`，纯资料更新、已读和静音不改变活动顺序。

PR06 在一个 PR 中切换迁移、全部写入、读取、绑定和各端比较，删除群专属顺序及来源三元排序。使用空库或可丢弃测试库验收；已有开发库停服后按仓库流程重建，不提供旧消息序号回填、默认零值或双顺序兼容。

文件、语音、卡片、反应、@ 提醒和系统事件使用独立关系或类型化载荷扩展。`messages` 仍是时间线内容事实来源；消息序号不能发现旧消息编辑、删除和资料变化，这些变化由第 10.8 节的同步索引恢复，不把消息表变成事件总线。

### 5.4 CustomerConversation

`CustomerConversation` 是客户会话扩展，关联通用会话与 `contact_channel_identities`。一条客户 Conversation 只绑定一个渠道身份；同一渠道身份可以根据来源线程能力拥有多条 Conversation，并通过该关系为列表提供联系人和来源渠道的可靠查询路径。

`customer_conversations` 同样保留 `created_at` 和 `updated_at`；当前不依赖 `updated_at` 排序或驱动业务，但不因暂时未使用而省略标准审计字段。

客户会话严格表示一对一客服关系，只允许一个有效联系人参与者。群、频道和讨论组不创建 `customer_conversations`。

网站 Messenger 能明确创建和选择 Cervi Conversation，可以把同一渠道身份下的不同客户话题保存为独立线程。Telegram Bot 私聊、微信公众号等没有稳定线程选择能力的来源，由 Adapter 固定把该渠道身份映射到一条 Conversation；来源以后提供稳定线程编号时再按真实能力扩展。不同渠道身份之间仍不自动合并消息时间线，跨渠道历史只在联系人或工单层聚合。

客户会话扩展不保存外发队列、Worker 租约或渠道 FloodWait。需要调用外部客服平台时，投递状态进入独立 `customer_message_deliveries` 和渠道发送 Gate；网站访客直接读取 Cervi 时间线，不创建投递记录。

### 5.5 ServiceSession

`ServiceSession` 表示一条客户 Conversation 上的一次客服处理过程，与客户可见线程分离。持久状态只保留 `open` 和 `closed`；开放周期是否排队以及由谁负责，分别由 `assignee_identity_id` 是否为空及其指向表达。团队、转接记录、响应指标和满意度按实际需求另行建模，不把它们扩成同一状态枚举。

一个客户 Conversation 可以先后产生多个服务批次，同一 Conversation 同时最多一个未结束批次。批次不切断 Conversation 消息历史，也不作为客户侧聊天列表和历史接口的主键。真人单聊、AI 聊天、群聊和第三方账号会话不创建服务批次。

网站 Messenger 允许同一 `contact_channel_identity` 同时拥有多条未结束客户线程，每条 Conversation 仍同时最多一个未结束服务批次。访客选择哪个 Conversation，就继续哪个客户线程。

### 5.6 Ticket

工单是问题处理与流程对象，不等同于会话。会话和工单使用多对多关系：一个长期会话可以产生多个工单，一个工单也可以关联多个客户、第三方或内部会话。

建议关系：

```text
ticket_conversation_links
├── ticket_id
├── conversation_id
├── source_message_id
├── range_start_message_id
├── range_end_message_id
├── created_by_user_id
└── created_at
```

跨企业共享会话不表示共享工单。每个企业默认维护自己的工单状态、负责人、SLA 和内部评论。

## 6. 现有通讯录与联系人调整

### 6.1 内部人员

现有内部通讯录结构方向保持不变：

- `organization_identities.type = user | agent` 统一企业内部展示身份。
- `organization_identities.role_id` 是真人和 AI 员工唯一的企业角色来源；`users` 与 `agents` 子类型只保留各自账号状态和专属配置。
- `users.identity_id` 和 `agents.identity_id` 分别关联具体身份。
- `team_members.identity_id` 继续只允许有效用户或智能体加入团队。

聊天不直接引用 `users.id` 或 `agents.id`，而是为对应 `organization_identities.id` 创建 `kind = organization_identity` 的 `chat_subject`。因此用户和智能体通过完全相同的参与者、消息、提醒和反应路径进入聊天。

如后续需要调整现有 `organization_identities`，必须新增向前迁移，不修改已经存在的建表迁移；届时应把 `work_status_updated_at` 统一改为 `timestamptz`，避免与项目其他时间字段不一致。

### 6.2 外部联系人

现有联系人 CRUD 的业务对象与聊天设计一致。客户聊天数据底座 PR 只调整自动创建联系人所必需的字段：

- `contacts.created_by_user_id` 可空。
- `contacts.source_channel_id` 保持非空。
- 现有手工创建 Action 继续记录创建用户和明确来源渠道，但管理端“添加外部联系人”入口暂时隐藏，避免创建没有真实渠道身份、却被误认为能够直接聊天的联系人。
- 渠道自动创建联系人：创建用户为空，来源渠道为当前渠道。

联系人合并、导入、系统同步和无来源手工联系人出现真实需求时，再共同设计来源审计，并决定是否把 `source_channel_id` 改为可空。当前不为假设中的未来入口提前放宽约束。

`source_channel_id` 只记录联系人当前明确的创建来源，不表示联系人只能出现在该渠道，也不承担消息发送路由。真实渠道账号关系始终由 `contact_channel_identities` 表达。

联系人列表和详情继续按非空来源渠道查询，不在两个网站聊天 PR 中修改现有 DTO 和表单的来源契约。手工联系人表单、Action 和既有记录的编辑、删除、恢复能力暂时保留，只注释新增菜单入口。客户会话列表中的实际渠道始终从 `contact_channel_identities.channel_id` 读取，不能用联系人的创建来源渠道替代。

Telegram 用户账号同步得到的联系人默认不自动创建 CRM `contacts`。只有用户明确执行“保存为外部联系人”时才建立 CRM 联系人；Bot、网站等客服入站联系人始终走渠道路径，不由 TDLib 写入，避免把成员私人通讯录污染为企业客户数据。

## 7. AI 智能体作为一等聊天主体

本章只固定 Agent 与聊天事实的关系；配置、运行、工具、审批、设备和 Eino 接入由 `agent-roadmap.md` 定义，实时能力复用第 10 章。

### 7.1 身份与消息路径

AI 智能体继续使用现有 `organization_identities.type = agent` 和 `agents` 子类型。其聊天路径与用户一致：

```text
agent
  -> organization_identity
  -> chat_subject
  -> conversation_participant
  -> message.sender_participant_id
```

Agent 已可加入单聊、群聊和网站客户会话，并在单聊与网站客服中作为发送者；群内 @Agent 和响应策略仍待独立设计。引用、暂停和只读限制随对应能力交付，消息沿统一审计链追溯。

不创建 `bot` 参与者类型。Cervi AI 是 `agent`；Telegram 等平台中的机器人账号如果作为远端发送者出现，则属于未来的 `external_sender`，两者语义不同。

### 7.2 运行状态与聊天状态分离

AI 调用不能依赖用户已读状态，也不能把模型请求、工具步骤和令牌消耗塞入消息表。`agent_revisions`、`conversation_agent_policies`、`conversation_agent_states`、`conversation_agent_triggers`、`agent_runs`、`agent_run_steps` 和 `agent_tool_invocations` 的表结构、Revision 与快照语义、步骤与工具事实由 [agent-roadmap.md](agent-roadmap.md) 唯一定义，本文档不复制字段清单。本章只固定聊天域必须遵守的不变量：

- 独立 `message_mentions` 关系记录 @ 事实；符合策略且首次持久化的消息在同一事务写入 `conversation_agent_triggers`。
- 同一“会话 + 智能体”的 `trigger_seq` 由服务端锁定状态后单调分配，`desired_*` 与 `processed_*` 使用该序号；对应 Message 编号只作审计指针。`originated_at` 只用于来源时间展示和诊断；`message_seq` 决定本地时间线、分页和阅读位置，Trigger 水位独立。迟到的历史补拉默认不创建 Trigger，需要时通过独立总结或人工回放命令处理。
- 每个“会话 + 智能体”同时最多存在一个排队中或运行中的 Run。新消息在已有 Run 执行期间只推进 `desired_*`；Run 结束时原子推进 `processed_*`，仍有差距则在同一事务创建下一 Run 并通过 `TxEnqueuer.EnqueueIn` 唤醒，不能因活动任务幂等丢失后续处理。
- `agents.status` 表示智能体全局停用，`conversation_participants.left_at` 表示退出会话。P1b 网站 AI 客服只以 ServiceSession 的 `open/closed + assignee_identity_id` 表达当前客服状态，不增加 Agent 专属暂停状态；完整 P1 如引入通用会话响应策略，再定义其状态。`agent_identity_id` 统一指向 `organization_identities.id`。
- 自动响应记录触发消息、精确输入快照、配置版本与快照、语义步骤、工具调用、输出消息、费用、失败、取消和人工接管，保证可审计和可恢复。

首轮 Agent Direct 和网站 AI 客服都允许同一 Run 在 Eino 安全点吸收连续 Trigger，并只持久化一条最终文本 Message，不依赖流式实时能力。开发期 calculator 是可配置延时、且不创建 Tool Invocation 的临时纯函数测试 Tool，用于验证 Tool 完成后下一 Turn 读取最新输入，正式发布前删除；任何业务、设备或副作用 Tool 仍须先落完整审计。最终 Message 使用 `agent:<agent_run_id>` 业务幂等键；输出消息、Run 终态与 `processed_*` 在同一事务提交。`task_runs` 只负责至少一次唤醒与租约，不承担 Agent Run 或工具调用账本。

## 8. 第三方用户消息账号接入预留

### 8.1 接入分类与产品边界

第三方用户消息账号表示 Cervi 用户本人授权、能够以该用户身份读取和发送消息的外部账号。Telegram + TDLib 是第一个实现，但数据模型不以 Telegram 的编号、会话文件或能力范围作为通用假设。

外部平台按授权主体和产品用途分流：

| 类型 | 典型场景 | Cervi 模型 | 规则 |
| --- | --- | --- | --- |
| 完整客户端协议 | Telegram TDLib、Matrix Client-Server API、XMPP 用户账号 | Provider Connection + 用户消息账号 | 可以按平台能力逐步替代官方客户端 |
| Workspace 委托授权 | Slack 用户 OAuth、Teams 委托 Graph | Provider Connection + 用户消息账号 | 只提供官方授权实际允许的范围，不承诺完整客户端能力 |
| 企业客服 API | 网站、微信公众号、Telegram Bot、WhatsApp Business 等 | `channels` + 客户会话 | 企业拥有的客服入口，不是员工个人账号 |
| 非官方用户自动化 | 个人微信 Hook、Discord Selfbot、盗用 User Token 等 | 默认拒绝 | 不因技术上可模拟登录就纳入用户账号模型 |

同一平台可以同时拥有 Channel Adapter 和 User Messaging Adapter，但两者属于不同领域。例如 Telegram Bot 继续进入 `channels`，Telegram TDLib 用户会话才进入本章模型。`ChannelType` 不表示个人消息账号，也不与 User Messaging Adapter 共用存储枚举。

### 8.2 Provider、Adapter 与外部命名空间

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

远端账号、会话和发送者编号的唯一作用域不是整个 Provider，而是平台中的稳定外部命名空间。因此阶段 3 增加薄 `messaging_provider_connections`：

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

### 8.3 用户账号、所有权与共享

阶段 3 的用户账号目标结构：

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
- 阶段 3 一个账号只有一个 Owner。一个 Cervi 用户可以拥有多个不同外部账号。
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

Binding 只表示额外查看或代发授权，Owner 仍以 `owner_user_id` 为唯一真相。阶段 3 第一版不创建该表，团队成员或客服角色也不自动获得账号访问权。

### 8.4 会话、发送者、消息和账号状态

进入阶段 3 时按实际能力创建以下扩展表，子表不重复保存 `provider`，统一通过 `messaging_account_id -> connection_id` 解析：

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

### 8.5 外部编号、游标和来源顺序

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
- 客服和站内时间线的 `messages.source_order` 已在阶段 1 为 Telegram Bot 私聊增加，使用非空 `bigint`，没有来源顺序时统一为 `0`；它与第三方账号映射字段不共用约束。

账号级 `projection_cursor` 表示当前账号投影进度，会话级 `connected_chats.projection_cursor` 表示单个远端会话历史补拉进度，`provider_read_cursor` 表示平台已读位置。三者语义不同，不能共用时间戳或消息编号字段。

### 8.6 Adapter 能力契约

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

### 8.7 凭证、会话与运行时隔离

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

### 8.8 会话映射不变量与平台例外

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

### 8.9 发送、同步与生命周期规则

- 一个 Cervi 用户可以绑定多个不同平台或同平台账号，聊天列表必须清晰标识接收和发送账号。
- 第三方用户账号不能写入 `channels` 或 `customer_conversations`，不能使用 `chmsg:` 幂等键；除用户明确执行“保存为外部联系人”外，也不写入 `contacts` 或 `contact_channel_identities`。
- 本地发起的远端操作使用稳定 `client_operation_id`，发送状态和远端消息编号保存在 `connected_message_records`，不依赖 `task_runs` 作为业务投递账本。
- 外发调用出现超时、连接中断或进程崩溃时先标记结果不确定，通过平台本地状态、历史或权威查询对账；只有确认远端未发送后才能重试。
- Connector 按账号串行推进投影游标，支持断线重连、限流、幂等写入和历史补拉；平台瞬时 Update 不默认全部写入 PostgreSQL。
- 账号解绑不删除已沉淀消息；保留、脱敏或删除由企业策略和用户授权决定。
- 编辑、删除、反应、置顶、草稿和已读按 Adapter 能力逐项接入，不承诺所有平台都能双向同步。

## 9. 受管外部协作与联邦预留

### 9.1 受管外部协作

未部署 Cervi 的供应商或合作伙伴由当前企业托管访客账号，只能访问被邀请的会话、工单和文件。受管访客不能浏览完整通讯录、加入未邀请群、查看内部工单字段或管理企业资源。

默认使用定向、一次性、短期有效邀请。入口优先使用响应式 Web/PWA 和安全链接。小程序只是客户端壳，不是新的消息渠道。

访客来源记录创建 `kind = guest` 的 `chat_subject` 后即可使用通用参与者和消息模型，但登录、邀请和访问范围仍由访客域负责。

### 9.2 联邦通信

联邦通信遵循：

- 不假设所有参与者存在于同一数据库。
- 不使用数据库自增编号作为跨部署标识。
- 服务器请求需要认证、签名、幂等和重放保护。
- 远端成员只保存必要投影，不创建成本地企业成员。
- 会话、消息、成员和文件同步都显式带协议版本和全局标识。
- 首期不实现分布式事件图或多主冲突解决。

未来联邦用户投影创建 `kind = federated_user` 的 `chat_subject`。受管访客升级为联邦身份时不重写历史发送者，只新增经过双方确认的身份关联供界面聚合，新消息使用新的联邦主体。

联邦阶段使用独立的协议入站和出站事件表，以 `(peer_deployment_id, event_id)` 等协议稳定编号永久防重，并保存签名、协议版本、投递确认和重放窗口。联邦协议事件不能复用 `task_runs`、客服投递表或客户端同步日志。

## 10. 可靠任务、外部投递、客户端同步与实时传输

### 10.1 四类可靠性对象

聊天中的可靠性由四类不同对象承担，不能合并成通用 `chat_inbox`、`chat_outbox` 或领域事件总线：

| 对象 | 职责 | 数据性质 | 落地阶段 |
| --- | --- | --- | --- |
| `task_runs + task_outbox` | 可靠执行一个已注册的异步 Action | 命令运行与临时发布状态 | 项目已有，聊天复用 |
| `customer_message_deliveries` 等来源投递表 | 记录一条消息在外部平台上的投递、顺序、回执和不确定结果 | 长期业务记录 | 对应外发能力落地时 |
| `conversation_sync_events + user_sync_states + user_conversation_wakeups` | 告诉客户端哪些会话变化，并补拉会话内编辑、删除、反应和成员变化 | 有保留期的客户端同步 Changelog 与压缩索引 | 阶段 2 |
| `realtime_outbox` | 把事务内产生的最新同步水位可靠交给 Core NATS，缩短在线客户端感知延迟 | 发布成功后删除、允许重复的临时传输记录 | 阶段 2 |

`messages` 是聊天内容和时间线的业务事实来源；它不是外部平台投递状态，也不能单独表达旧记录发生的增量变化。

`realtime_outbox` 不是聊天业务 Outbox，不保存消息正文、客户端 ACK 或长期事件。它只关闭 PostgreSQL 提交与 Core NATS 发布之间的崩溃窗口；Gateway、NATS 或 WebSocket 丢失通知后，客户端仍以同步水位和 HTTP 补拉恢复。

现有 `internal/actions/inbox` 表示工作台统一收件箱查询，与分布式系统的 Inbox Pattern 无关。技术接入表不得复用这一业务名称和包职责。

### 10.2 复用现有任务 Outbox

现有服务端任务运行时已经提供：

```text
业务入队
  -> task_runs + task_outbox
  -> PostgreSQL Outbox 发布到 NATS JetStream
  -> Worker 按数据库租约认领 task_run
  -> Action 成功、失败或退避重试
```

它是“请可靠执行某个 Action”的事务型命令 Outbox，不是“某个领域事实已经发生”的广播事件日志：

- `task_outbox` 发布成功后删除，不用于长期重放。
- JetStream 使用工作队列语义，一个任务由一个 Worker 处理，不提供多消费者事件扇出。
- `task_runs.idempotency_key` 只避免相同 Action 的活动任务并发重复；任务进入成功或失败终态后，相同键可以再次创建。
- `task_runs` 的租约可以避免同一个 Run 同时被多个 Worker 执行，但不能提供 Exactly Once。
- Handler 已完成外部副作用、但尚未提交任务终态时崩溃，任务仍可能再次执行。

因此不创建 `chat_outbox`、通用 `domain_events` 或 NATS 消费 Inbox。阶段 2 的专用 `realtime_outbox` 只发布客户端同步水位，不能复用 `task_outbox`，也不能被业务 Handler 当成投递账本。聊天 Handler 必须可重入，永久业务幂等由 `messages`、来源映射或投递记录保证，不能依赖 `task_runs` 的活动幂等键。

### 10.3 事务内 Enqueue

现有公开 `Runtime.Enqueue` 会开启独立事务，不能与聊天 Action 正在执行的业务事务原子提交。第一次出现“业务记录和异步副作用必须同时成功或同时失败”的场景前，应在 `internal/task/server` 增加事务内接口：

```go
type TxEnqueuer interface {
    EnqueueIn(
        ctx context.Context,
        tx bun.IDB,
        actionName string,
        payload any,
        options EnqueueOptions,
    ) (string, error)
}
```

实现规则：

- `Runtime` 继续作为门面，负责 Action 注册校验、Payload 编码和 Options 规范化。
- 底层只使用传入的 `bun.IDB` 写入 `task_runs + task_outbox`，不再开启嵌套事务。
- Action 只依赖 `TxEnqueuer`，不直接写任务表，也不接触 NATS。
- 任务 Payload 只携带企业编号和稳定业务编号，Handler 读取业务记录时重新校验 `organization_id`。
- 普通 `Enqueue` 继续服务于没有外层业务事务的任务。

首个聊天 PR 没有异步副作用，不需要同时实现该接口；阶段 1 的第一个可靠外发 Action 必须先具备它。

### 10.4 入站消息不使用统一 Inbox 表

不同来源拥有不同的受理、去重、顺序和保留语义，只统一适配器流程，不统一存储表：

| 来源 | 受理和防重方式 |
| --- | --- |
| 网站挂件 | 同步调用入站 Action，使用 `messages.idempotency_key` 防重，不创建原始 Inbox 表 |
| 公众号或 Bot Webhook | 默认验签后同步归一化并写入消息；只有处理明显变慢或必须先返回成功时，才增加来源专属 `channel_inbound_events` |
| Telegram TDLib | 使用 TDLib 本地状态、账号同步游标和 `connected_message_records`，不把 typing、在线状态等每个 Update 写入 PostgreSQL |
| NATS 内部任务 | 使用 `task_runs`，不再套一层消费 Inbox |
| Cervi 联邦 | 阶段 6 建立带签名、协议版本和永久事件编号的协议 Inbox |

可选 `channel_inbound_events` 只保存平台受理所需的原始事件编号、载荷、处理状态和错误，归一化后仍调用通用聊天 Action，不能让 Worker 绕开领域 Action 直接拼装联系人和消息。

### 10.5 客服外部投递记录

网站访客直接从 Cervi 消息时间线读取客服回复，消息行本身就是投递事实，不创建 Delivery。微信公众号、Telegram Bot 等需要调用外部平台的客服回复，在对应能力落地时增加 `customer_message_deliveries`：

```text
customer_message_deliveries
├── id
├── organization_id
├── conversation_id
├── message_id
├── channel_id
├── contact_channel_identity_id
├── bot_id                         # Telegram 投递使用的机器人编号
├── position
├── status
├── attempt
├── provider_message_id
├── lease_worker
├── lease_expires_at
├── available_at
├── uncertain_until
├── last_error
├── sent_at
├── created_at
└── updated_at
```

阶段 1 的一对一客户会话中，一条本地消息只向一个客户渠道身份投递。核心唯一性和索引：

```text
UNIQUE (organization_id, message_id)
UNIQUE (channel_id, contact_channel_identity_id, position)
UNIQUE (channel_id, bot_id, contact_channel_identity_id, provider_message_id)
    WHERE provider_message_id IS NOT NULL
UNIQUE (channel_id, contact_channel_identity_id)
    WHERE status IN ('sending', 'uncertain')

INDEX (available_at, channel_id, contact_channel_identity_id, position)
    WHERE status IN ('pending', 'retry_wait')
INDEX (lease_expires_at)
    WHERE status = 'sending'
INDEX (uncertain_until)
    WHERE status = 'uncertain'
```

`position` 是同一渠道身份外发管道中的严格顺序。在锁定对应 `contact_channel_identities` 行后，按 `(channel_id, contact_channel_identity_id)` 分配 `MAX(position) + 1`，不能使用创建时间或 UUID 推导平台发送顺序。网站等允许一个渠道身份拥有多条 Conversation 的来源仍共享同一身份级外发管道，避免客户在外部一对一聊天中收到交错乱序消息；网站访客直接读取 Cervi 时间线，不创建 Delivery。

渠道级 FloodWait 或全局限流不写入客户会话，使用独立状态：

```text
customer_channel_send_gates
├── channel_id
├── organization_id
├── flood_wait_until
└── updated_at
```

`customer_conversations` 继续只表达客户会话与渠道身份的业务关系，不加入 dirty、Worker lease 或 FloodWait 等投递运行字段。TDLib 和联邦也不复用 `customer_message_deliveries`。

### 10.6 外部投递状态机

成员端消息气泡只展示三种产品状态：发送中（时钟）、已发送（对勾）和需要关注（提醒）。本地提交、排队、平台调用和自动重试共用发送中图标；失败、渠道停用和发送结果未确认归入需要关注，具体原因及处理方式在提示和操作入口中表达。数据库投递状态不直接作为用户可见状态，发送结果未确认也不等同于发送失败。

外部平台调用结果分为确定成功、确定未受理和结果未知。状态机至少包括：

```text
pending
  -> sending
      -> sent
      -> retry_wait
      -> failed
      -> uncertain

uncertain
  -> sent                 Echo 或权威查询确认成功
  -> pending              权威查询确认没有发送
  -> needs_review         等待期结束仍无法确认

needs_review
  -> pending              人工确认风险后重试，并分配新的队尾 position
  -> sent                 人工标记已送达
  -> failed               人工标记失败
```

规则：

- 只有 `pending` 和到期的 `retry_wait` 可以自动调用平台。
- `pending/retry_wait/sending/uncertain` 属于队头阻塞状态；`sent/failed/needs_review/cancelled` 属于自动管道终态，不再阻塞同会话后续投递。
- 2xx 且取得平台消息编号属于确定成功，立即用最小事务写入 `sent + provider_message_id`。
- 429 或平台明确表示未受理时进入 `retry_wait`；永久业务拒绝进入 `failed`。
- 超时、连接重置、含义不明的 5xx、进程在平台调用前后崩溃，以及 `sending` 租约过期，都进入 `uncertain`。
- `sending`、`uncertain` 和 `needs_review` 不能自动重新发送，避免远端成功但本地未记录时产生重复消息。
- `uncertain` 在配置的等待窗口内阻塞同会话后续外发，为 Echo 或权威查询留出时间；到期后进入 `needs_review` 并放行后续消息。
- 人工重试必须提示“对方可能已经收到，重试可能产生重复”，并作为新的队尾操作执行。
- 入站适配器识别由本渠道发出的 Echo 后，只用于补全投递状态和平台消息编号，不能再次作为客户入站消息创建 `messages`。

编辑、撤回等远端操作也必须进入同一路由顺序，但在真正实现相关渠道能力时再增加操作类型或独立操作表，首期不预建空表。

### 10.7 持久有序扫描与丢失唤醒

外发正确性不能依赖进程内 FIFO、JetStream 消息顺序或 `task_runs` 的会话级活动幂等键。服务端有多个 Worker，未来也可能存在多个实例；进程内队列无法跨实例协调，崩溃后也无法恢复。

阶段 1 使用 PostgreSQL Delivery 表作为持久有序队列：

1. 生产事务锁定 `contact_channel_identities`，写入本地消息和 `pending` Delivery，按该渠道身份分配 `position`，并通过 `EnqueueIn` 可选创建以 `delivery_id` 为作用域的快速唤醒任务。
2. 周期扫描器是正确性来源，持续查找已到期的 `pending/retry_wait`、租约过期的 `sending` 和到期的 `uncertain`。
3. 多实例扫描时，对候选 `contact_channel_identities` 行使用 `FOR UPDATE SKIP LOCKED`；选定渠道身份后只处理该身份最小 `position` 的非终态队头，不能对 Delivery 队头使用 `SKIP LOCKED` 后跳到下一条。
4. 同一事务把队头更新为 `sending` 并写入短租约，提交后才在事务外调用平台。
5. 平台返回后用新的短事务提交确定成功、重试、永久失败或不确定结果。
6. Worker 崩溃后，扫描器把过期 `sending` 升级为 `uncertain`，禁止直接重发。
7. 毒消息最终进入 `failed` 或 `needs_review`，不永久阻塞后续消息。

可选快速任务使用 `cdeliv-item:<delivery_id>` 等每条投递独立的活动幂等键，只负责降低延迟；即使任务丢失或空执行，扫描器仍能恢复。禁止使用 Conversation 或渠道身份级 Drain 任务键合并整条管道：旧 Run 尚未提交终态时，新投递可能命中旧活动任务而不创建新 Run，随后产生永久丢失唤醒。

渠道限流由 `customer_channel_send_gates` 阻止同一渠道继续认领。首版可以通过 `UNIQUE (channel_id) WHERE status = 'sending'` 限制同一渠道同时只有一个平台调用，验证平台限流语义后再安全增加并发；同一渠道身份的 FIFO 始终由 `position` 和队头规则保证。

### 10.8 客户端离线增量同步

新消息通过 `message_seq` 的 `after` 游标补拉，更早历史通过同一顺序的 `before` 游标读取；但旧消息编辑、删除、反应、参与者变化和会话设置更新发生在已有行或其他表中，单纯补拉新消息会遗漏这些变化。

阶段 2 承诺实时消息和离线补拉时使用“用户 Mailbox 指出哪些会话变化，会话 Changelog 描述具体变化”的两层游标。只使用每会话序号时，拥有大量会话、多个设备或多个第三方账号的用户必须扫描全部会话才能发现变化，不能作为目标设计。

每会话 Changelog：

```text
conversations.sync_seq

conversation_sync_events
├── id
├── organization_id
├── conversation_id
├── seq
├── kind
├── entity_id
├── payload
└── created_at
```

核心约束与索引：

```text
UNIQUE (organization_id, conversation_id, seq)   -- 兼作会话内增量扫描索引
```

规则：

- `conversations.sync_seq` 保存会话当前同步水位，`conversation_sync_events.seq` 在每个会话内严格单调递增。业务变更在锁定会话、推进水位和写入 Sync Event 的同一事务中完成。
- 事件类型覆盖消息新增、编辑、删除，参与者变化，会话设置变化，以及以后增加的反应和回执变化。
- Payload 只保存客户端恢复所需的类型化最小信息；业务实体当前状态仍从对应业务表读取。
- Changelog 按保留策略清理。客户端落后超过保留窗口时，重新获取会话、参与者和消息快照，再从新的同步序号继续。
- 该表不用于搜索、通知和 AI 的多消费者广播。搜索从业务表回填，AI 继续使用独立消息处理游标和 Run。

`conversation_sync_events` 是客户端同步协议，不是通用领域事件总线，也不能承担 Cervi 联邦传输。

每用户压缩 Mailbox：

```text
user_sync_states
├── organization_id
├── user_id
├── mailbox_seq
├── oldest_retained_mailbox_seq
└── updated_at

user_conversation_wakeups
├── organization_id
├── user_id
├── conversation_id
├── mailbox_seq
├── conversation_seq
├── kind
└── updated_at
```

核心约束与索引：

```text
UNIQUE (organization_id, user_id)                         -- user_sync_states
UNIQUE (organization_id, user_id, conversation_id)        -- 当前压缩行
UNIQUE (organization_id, user_id, mailbox_seq)            -- 序号不重复，兼作增量扫描索引
```

规则：

- `mailbox_seq` 按用户严格单调递增。`user_conversation_wakeups` 对每个用户、每个会话只保留当前最新一行，是“哪些会话变了”的压缩索引，不复制会话 Changelog。
- 一次事务影响同一用户的多个会话时，完成业务锁后，按稳定顺序锁定 `user_sync_states`，一次推进所需序号区间，再给每个 Wakeup 分配不同序号。禁止无锁读取后在应用内执行 `+1`。
- 业务行、会话 Sync Event、变更前与变更后受众的 Wakeup，以及 `realtime_outbox` 在同一事务提交。Gateway 不自行推断受众。
- 原生单聊和小群通知有效的本地用户参与者；第三方账号会话只通知账号 `owner_user_id`；AI 智能体不消费用户 Mailbox，继续使用独立的 `conversation_agent_states` 和 Run。
- 用户被移出会话、会话删除时，仍向变更前受众写 `removed/deleted` Tombstone。同步接口只返回其曾经可见的会话编号和删除种类，不能向已无权限用户返回正文或当前成员。
- 第三方账号解绑使用账号级 `account_unbound` Tombstone，客户端按账号清理本地会话投影，不能为成千上万个会话逐行制造解绑事件。
- 普通 Wakeup 在关系有效期间可以长期保留；已删除关系和 Tombstone 按离线窗口保留。`after < oldest_retained_mailbox_seq` 时返回 `need_snapshot`，客户端重新拉取有权访问的会话列表和必要快照。
- 服务端不保存每设备消费游标。首版各端业务投影和游标只保存在内存；刷新、退出或缓存清空后重新建立基线，禁止单独持久化游标而跳过已丢失的数据。真正引入离线库时再把投影与游标原子持久化。

各受众增量请求统一携带 `after`、可续用的 `head`、`limit` 和 `epoch`。首响应固定本轮 head，续页只扫描 `(after, head]`，返回 `nextAfter`、`hasMore` 和当前公开投影或移除标记。`nextAfter` 取已扫描位置，不能取最后一条可见实体的位置；即使整页实体被授权过滤也必须前进。压缩 Wakeup 被更新到本轮 head 之外时留到下一轮，不能因此宣称已永远追平。实体当前版本可能比事件更新，不能用事件旧版本覆盖它。

同步协调器属于登录外壳，离开消息页仍工作。每条流分别保存 `target / applied`，合并目标后串行追赶；索引应用进度与实体读取版本分开，窗口重读失败保留 dirty。协调器只保存版本和失效状态，业务 DTO 由 Query 缓存持有；视图控制器保存 ID 顺序、窗口边界、锚点和交互状态。账号、企业、服务器或访客身份变化使旧 generation 全部失效，旧响应不能恢复敏感内容。

冷启动、缓存丢失、游标过旧或超前、epoch 不符时重建受影响流：先记录 H0，按不可变 conversationId 扫描完整可见索引，再增量追赶 H0 后变化。完整索引不受展示列表条数和活动排序限制；每页投影与水位来自同一数据库快照，跨 HTTP 页不持长事务，也不声称历史快照一致。未缓存会话可只记录目标版本，打开时再读；重建失败不能确认基线完成。清理事件／tombstone 与推进保留边界同事务；数据库重建或恢复备份必须更换 epoch，普通服务重启不更换。

受众统一以 `(namespace, organizationId, audienceKind, audienceId)` 标识，首版只有：

| audienceKind | audienceId | 同步范围 |
| --- | --- | --- |
| `user` | userId | 内部会话、本人阅读／提醒／置顶状态及身份变化 |
| `customer_inbox` | organizationId | 企业客服共享列表和客户会话变化，不逐客服写个人 Wakeup |
| `visitor_directory` | 渠道身份记录 ID | 该身份全部有权线程及公开会话版本，覆盖另一页面新建的线程 |

ID 来自服务端授权结果，不能用凭据或客户端任意编号订阅。客服共享 head 和用户个人 head 独立保存，不能相互比较。访客先追目录，再补当前窗口；不能只检查已经打开的会话。会话 Changelog 按 conversationId 定位，不创建第四类持久受众。团队队列、大群 Shared Fanout 和第三方账号流待对应能力出现后扩展。

小群先使用每用户 Fanout-on-write，客服使用共享受众。PR41 分别测量 users 守卫、用户水位行、共享客服水位行争用及 HTTP 放大；只按实际证据再优化锁、普通性能索引或 Shared Fanout，不预设产品人数上限或容量承诺。

### 10.9 实时传输与持久命令边界

Cervi 使用一个 WebSocket 连接承载实时下行事件和临时上行控制，但不把它设计成第二套业务调用总线：

| 通道 | 职责 |
| --- | --- |
| HTTP / Wails 绑定与 API Proxy | 登录、持久命令、业务查询、Mailbox/会话同步、快照和附件上传下载；调用统一进入 `appservice.Service` |
| WebSocket | 同步水位通知、输入状态、在线状态、当前焦点、AI 流式输出、任务进度和 WebRTC 信令 |
| WebRTC | 音视频媒体；P2P 优先，TURN/SFU 按网络和群聊需求补充 |
| APNs / FCM / 厂商推送 / 可选 Web Push | 应用退出或后台时的系统级唤醒，收到后通过业务查询同步权威水位 |

发送消息、编辑、撤回、回执、成员变更、会话设置、发起或取消 AI 运行等持久命令统一经过 `appservice.Service`，进入同一 Action、事务、错误映射和幂等体系。桌面端和移动端需要离线可靠发送时，由客户端任务能力重试同一个幂等调用，不把任务队列绑定到长连接。

WebSocket 上行只接受认证、Hello、Ping/Pong、输入状态、在线状态、焦点会话和 WebRTC 信令等临时控制。此类事件不进入消息表、会话 Changelog、Task Outbox 或 JetStream，允许限流、合并和丢弃。

持久变更的数据流固定为：

```text
服务端 Web -> appservice.Service(DirectBackend)
桌面或移动端 -> appservice.Service(API Proxy) -> Gin -> appservice.Service(DirectBackend)
网站挂件或渠道回调 -> Gin -> appservice.Service(DirectBackend)
appservice.Service -> Action
  -> 同一 PostgreSQL 事务写业务记录、会话 Sync Event、受众 Mailbox/Inbox 水位和 realtime_outbox
  -> 调用成功响应
  -> realtime_outbox 发布 Core NATS
  -> Realtime Gateway 通知本节点连接
  -> 客户端按 Mailbox 和会话序号通过业务查询增量同步
```

WebSocket 可以携带 `message_id`、会话序号等路由提示，但首版不复制完整 `Message`、`Conversation` 或 `Participant` 业务 DTO。客户端收到水位后批量同步业务投影，避免实时协议与 `appservice` 契约长期漂移。

### 10.10 Realtime Gateway、Outbox 与 NATS

Realtime Gateway 第一阶段作为 Cervi Server 内的独立模块运行，共用认证和应用生命周期；连接规模或独立扩缩容需求出现后再拆成单独 Go 服务。客户端永远不直接连接 NATS。

`realtime_outbox` 与产生业务变化的事务一起写入，一条业务变化原则上写一行。目标结构至少包括：

```text
realtime_outbox
├── id
├── organization_id
├── event_id
├── audience_kind
├── audience
├── conversation_id
├── conversation_seq
├── attempts
├── available_at
├── lease_token
├── lease_expires_at
├── last_error
├── created_at
└── updated_at
```

`audience` 只保存发布所需的规范受众标识、epoch 和水位，或类型化撤销控制；小群保存目标用户，客服和访客使用各自共享目标。`conversation_id` 和 `conversation_seq` 按通知种类允许为空。`event_id` 全局唯一，用于日志关联和发布端去重，不取代客户端同步序号。

发布器使用短租约认领记录，向 Core NATS 发布后执行 Flush，确认当前连接已经把批次交给 NATS Server 再删除；失败时释放租约并退避重试。一次记录包含多个用户时，发布中途失败可以从头重发整个记录，不能维护逐用户永久 ACK；重复通知由 Gateway 和客户端按 Mailbox/会话序号幂等处理。普通性能索引按实际查询证据另行评估，首轮迁移只保留业务唯一约束。

`realtime_outbox` 只关闭数据库提交后进程在发布前崩溃的窗口。Core NATS 仍是至多一次实时传输；Gateway 下线、订阅瞬断或 WebSocket 丢失通知时，正确性仍来自 PostgreSQL 同步记录。禁止为了实时扇出创建每用户 JetStream Consumer，也不能把现有工作队列语义的 `task_outbox` 改造成广播事件流。

NATS Subject 使用唯一编解码器，与 Outbox、Hello heads、水位帧及客户端追赶 key 的受众标识一致：

```text
<namespace>.realtime.<organizationId>.<audienceKind>.<audienceId>
```

`audienceKind` 仅为 `user / customer_inbox / visitor_directory`，ID 使用无点的内部规范值；禁止原始凭据或用户输入直接拼接。epoch 独立表示流恢复代次。撤销和下线控制与水位共用 Outbox，以通知种类区分，按需携带 tokenSessionId／conversationId，不能被当成普通水位合并丢失。

规则：

- Gateway 只为本节点已连接用户订阅用户 Subject；只有本节点存在有权连接时才订阅对应客服 Inbox／访客目录 Subject；大群受众到对应阶段再扩展。
- 在线扇出不能使用 Queue Group，否则同一用户连接分布在多个 Gateway 时只有一个节点收到通知。
- 同一用户的多个标签页和设备在节点内扇出；连接注册、能力和发送队列首版只保存在进程内，不引入 Redis、NATS KV 或粘滞会话。
- 不订阅企业级 `<namespace>.realtime.<organizationId>.>` 通配 Subject，避免把无关用户和租户流量发送给每个 Gateway。
- 滚动升级先停止接收新连接，再发送带随机重连延迟的 `server_going_away`，等待发送队列和 NATS 订阅 Drain 后关闭，避免客户端同时重连形成惊群。
- Ping/Pong 只负责保活，不每隔十几秒查询 PostgreSQL。客户端在窗口重新聚焦和低频周期校验时通过 HTTP 获取权威 Mailbox/Inbox Head，修复网关或权限异常。

### 10.11 Protobuf 实时协议

WebSocket 实时协议从第一版使用 Protobuf 二进制编码，Schema 是该传输层的唯一来源。新文件使用稳定的 Protobuf Edition 2024，不使用 `syntax = "proto3"`，也不同时维护 JSON、ProtoJSON 实时编码或 SSE 主通道。

契约边界：

- `appservice` Go 结构体继续是联系人、用户、会话、消息、收件箱和设置等业务 DTO 的唯一来源，并由 Wails 生成 TypeScript bindings。
- `proto/cervi/realtime/v1` 只定义连接认证、Hello、同步水位、临时状态、AI 流、WebRTC 信令、错误和优雅下线帧，不重新定义完整业务 DTO。
- Go 生成代码放入 `internal/realtime/protocol/v1`；TypeScript 生成代码放入 `frontend/src/api/realtime/generated`。两处都禁止手工修改，页面只能通过 `frontend/src/api/realtime` 使用实时能力。
- 使用 Buf 管理 Schema、Lint、代码生成和 Breaking Change 检查；Go 使用 `google.golang.org/protobuf` 与 `protoc-gen-go`，TypeScript 使用 `@bufbuild/protobuf` 与 `@bufbuild/protoc-gen-es`。CLI、插件和运行时全部固定精确版本。
- `wails3 generate bindings` 与 `buf generate` 是两条并列的契约生成任务；生成版本必须在 CI 中校验，不能让开发机工具静默覆盖为不同格式。

顶层分别定义 `ClientFrame` 和 `ServerFrame`，使用 `oneof payload` 形成 Go 与 TypeScript 都可收窄的事件联合。PR27 最小协议仅覆盖以下语义，名称可按生成规范统一：

```text
ClientFrame
├── Authenticate
├── ClientHello
└── Ping / Pong

ServerFrame
├── Authenticated
├── ServerHello（连接信息与受众 heads）
├── Ping / Pong
├── MailboxAdvanced
├── InboxAdvanced
├── VisitorDirectoryAdvanced
├── ConversationAdvanced（绑定已授权访客线程）
├── AccessRevoked / SessionRevoked
├── ServerGoingAway
└── RealtimeError
```

AI 帧与授权焦点控制由 PR45 扩展同一协议；typing、presence、任务进度和 WebRTC 随真实功能加入，不在最小协议预建空帧。访客会话提示使用目录受众传输，不能据此任意订阅 conversationId。

Schema 演进规则：

- Package 和 WebSocket Subprotocol 使用 `cervi.realtime.v1`；只有破坏性演进才增加 V2，不能在同一 V1 中改变已有字段含义。
- 已发布字段编号永不修改或复用；删除字段和枚举值后保留编号与名称。
- 新增字段和 `oneof` Case 必须允许旧客户端忽略。未知服务端事件不能导致旧客户端断开，客户端能力通过 Hello 显式协商。
- 枚举保留 `UNSPECIFIED = 0`，收到未知数值时按未知能力降级，不能误映射为有效业务状态。
- 不使用 `Any`、`Struct`、`type + bytes` 或 ProtoJSON 绕开类型约束。
- Protobuf 解码只保证线格式和生成类型；Gateway 仍需校验帧方向、`oneof` 是否有效、编号格式、长度、序号、连接状态、组织边界和资源权限。
- Buf Breaking Change 检查以 Git 主线为基准；Schema 和两端生成代码必须在同一个提交更新。

AI 流使用 `stream_id + sequence`，包含开始、增量、完成和失败帧。模型 Token 按几十毫秒合并后发送，不把每个 Token 写入消息表、Changelog 或 Outbox；完成、失败或取消时持久化最终业务状态。断线客户端通过 HTTP 获取已经持久化的消息或可选运行快照，不能要求 Gateway 重放全部 Token。

### 10.12 连接认证、恢复与背压

浏览器 WebSocket 不能自由设置认证 Header，因此连接认证统一采用 HTTP 换票：企业成员客户端使用 Bearer Token 调用成员换票端点；网站挂件则由公共换票端点先校验第 4.5 节的长期访客 Cookie/Header 及其客户会话访问范围。两者都只返回短期、一次性连接票据，再建立 WSS，并在五秒内用首个 Protobuf `Authenticate` 帧提交票据；票据不放入 URL 查询参数，避免进入代理和访问日志。

多 Gateway 时，票据摘要通过 PostgreSQL 原子消费，并显式保存 Principal 类型。成员票据绑定原登录 `tokens.id`（tokenSessionId）、企业、用户、稳定 `device_id`、客户端种类和允许的 Origin；访客票据绑定企业、网站渠道、`contact_channel_identity_id`、挂件客户端种类和允许的 Origin，不伪造用户或设备。两种票据都不能越过其绑定范围复用；签发和消费都复核原身份与授权。原生端由 Go Proxy 换票，只向 TS 返回短票据；Socket Origin 按 iframe 自身来源校验，宿主页白名单沿用网站嵌入规则。

认证后 `ClientHello` 携带协议主版本、Web/桌面/移动/挂件客户端种类、应用版本和能力集合。成员客户端同时携带 `mailbox_after` 和当前有权使用的共享 Inbox 游标；网站挂件携带票据绑定的访客目录游标及已缓存的授权客户 Conversation 同步游标，并只接收严格白名单内的会话水位和临时事件。服务端不允许任何客户端任意订阅会话编号；连接按已认证身份接收通知，焦点会话只用于提高临时状态和通知密度，不能改变授权。

Gateway 安装订阅并 Flush 后再读取 Hello heads，避免先读水位后订阅的空窗。成员连接到期不晚于原登录会话；登出、停用、群失权和渠道停用经事务 Outbox 撤销，服务端低频授权复核修复控制丢失。撤销时清除未发送的受限帧与焦点，不承诺撤回已进入网络的字节。NATS 恢复但 Socket 未断时也主动要求校验 Head。

客户端统一实现：

- 带随机抖动的指数退避重连，不进行固定间隔重试。
- 重新连接前换取新票据；票据不能跨设备、用户、访客身份、渠道或企业复用。
- 成员重连后先比较 Mailbox/Inbox Head；网站挂件先比较当前渠道身份的目录 Head，再检查所需客户 Conversation 的同步 Head；两者都通过对应业务查询补拉，WebSocket 和 NATS 不提供历史重放。
- 多设备分别维护内存投影和游标，同一用户的连接可以同时接收通知；缓存丢失按第 10.8 节重建。
- 页面或应用进入后台时不假设长连接持续存活；恢复前台后总是校验 Head。

Gateway 为每条连接维护单写协程和有界优先级发送队列：

```text
P0 认证结果、权限／会话撤销、错误、Ping/Pong、优雅下线
P1 Mailbox/Inbox/访客目录与会话水位
P2 AI 增量和任务进度
P3 typing、presence 等临时事件
```

同一受众、epoch 及通知种类的水位只保留最大值；会话提示另按 conversationId 区分，撤销控制单独保留，AI 增量按 Stream 合并，P3 可以丢弃；队列溢出时不能静默丢失 P0/P1，而应以 `slow_consumer` 关闭连接，让客户端重连并按水位同步。调度必须限制 P2 连续占用的字节数，避免长 AI 输出饿死控制帧和新消息通知。浏览器客户端同时观察 `bufferedAmount`。

首版单帧上限设为可配置的 64–256KB 范围，不通过 WebSocket 发送附件、历史列表或快照；默认不开启 `permessage-deflate`，验证 CPU 和每连接内存后再决定。服务端和部署文档必须配置反向代理 Upgrade、空闲超时、最大连接数和优雅关闭，并记录连接数、队列深度、慢消费者、认证失败、NATS 发布失败、Outbox 积压、同步追赶条数和各帧类型流量。

### 10.13 列表、个人置顶与通知行为

列表详情通过独立 Query 授权，不以当前列表是否包含它判定存在性。PR08–14 建立分页、窗口和选择行为，PR35 接入实时，PR47–49 交付个人置顶。置顶是本人跨端同步的全局手动顺序，新消息不改变顺序；新增置顶追加末尾，取消回普通活动序，再次置顶重新追加。失权清除置顶且重入不恢复，解散后仍可读则保留。各端滚动位置、焦点和选择不上传。

PR08 的 lastActivityAt 取消息追加事务中数据库当前时间与该会话旧值的较大值；新文本和系统消息更新活动，客服状态、纯资料更新、token、已读和静音不更新；群改名等操作若追加系统消息，则由该消息更新活动。预览显示来源时间；列表行和总数在同一只读快照中读取，前端按服务端顺序展示。

查询身份包含企业、用户、scope、customerView、assigneeIdentityId，启用后再加入分区和规范化搜索词；数据 key 共享，页面实例的选择与滚动位置各自保存。普通区按 `lastActivityAt DESC NULLS LAST, conversationId DESC` 排序；时间使用数据库精度，游标携带原值。置顶区按本人顺序升序，不采用活动时间。

| 输入／位置 | 内容与顺序如何处理 | 选择与滚动如何处理 |
| --- | --- | --- |
| 顶部且没有点击／拖动等交互 | 应用新项和最新活动序 | 保持顶部，不自动换当前详情 |
| 深处收到变化 | 更新已展示内容，暂存新增和活动序移动；资格移除立即生效 | 保存原可见邻居，不跟随活跃行上浮 |
| 点击“刷新” | 重读窗口并合并待更新结果 | 保留原浏览范围；原锚点已远离则用原后继／前驱定位 |
| 点击“查看最新” | 重建顶部窗口、清除已应用提示 | 明确回顶，仍保留有权阅读的详情 |
| 切换筛选 | 读取新查询；旧结果不可操作 | 恢复该查询历史，否则保持未选中 |
| 会话不再匹配筛选 | 从列表移除 | 有阅读权限则保留详情；失权立即清理敏感内容 |
| 断网／请求失败 | 保留已展示数据和独立错误状态 | 重试同一窗口，不换主体 DOM、不跳第一行 |

“保锚点”记录稳定可见 ID、相对滚动视口顶部的像素偏移、原后继／前驱及原窗口边界。读取前捕获，DOM 更新后测量一次并补偿；锚点因排序远移时优先留在原邻域。原生 scroll anchoring、控制器或虚拟列表只能有一个补偿所有者。用户正在滚动时延后非必要重排；失权清理不等待用户停止操作。

列表刷新和补页由同一控制器串行调度：刷新遇到在途补页，先等待其收尾，再捕获已经扩展后的窗口边界；刷新中触发补页则排队，重复刷新合并。切换查询使整个队列的 queryGeneration 失效，但不取消已经执行的 Wails 业务调用。刷新重读已加载双向边界之间的连续范围，并按 ID 重读已加载项的当前资格；不能只以首页或可见视口替换全部已加载结果。已上浮的可读行进入待重排集合，真正失权才按失权规则清理；请求失败保留已确认的窗口和重试边界。

#### 个人置顶写入与读取契约

置顶、取消和移动均提交 expectedPinOrderVersion，成功返回新版本。移动输入为 `{ conversationId, position, neighborId?, expectedPinOrderVersion }`：position 是 `before`／`after` 时 neighborId 必填，表示插入该邻居在**全局个人顺序**中的紧前／紧后；`start`／`end` 不带邻居，表示整个置顶区的首／尾。目标和邻居必须是本人当前有权读取的置顶会话；版本或邻居失效时整次操作失败，客户端重新读取后让用户重试，不自动用旧意图覆盖。

“隐藏项”包括被当前筛选排除及尚未加载的置顶项。移动只改变目标相对位置，其余项之间相对顺序保持；如全局 `A,X,B`、当前仅看见 A/B，将 B 移到 A 前，结果必须是 `B,A,X`。前端只提交位置命令，不能把可见 ID 数组当成全量个人顺序保存。rank 重编号只是存储实现变化，不改变逻辑顺序。

任意改变置顶集合／顺序的事务（含失权清理及随操作发生的重编号）推进用户水位与 pinOrderVersion，并经本人流带出最新顺序版本。客户端发现版本变化时让整个置顶分区及其游标失效，重新取得同一版本的权威窗口；不得把单条新 pinRank 与缓存旧 rank 混排。多页重读途中版本再次变化，舍弃该轮候选并从新版本恢复锚点；已展示内容可保留为待更新视图，但不能提交使用旧版本的排序。新消息只推进内容版本，不推进置顶顺序版本。

#### 阅读与通知边界

自动已读只按当前激活页面实际可见且连续读到的消息推进；用户明确执行“标为已读”时可推进到目标水位，提及确认保留独立语义。收到帧、建立连接、同步完成或打开详情都不能直接标为已读，移动端接入实时不顺带开启尚未交付的已读交互。

本地通知区分 live、catchup 和 bootstrap：在线确认的新 Message 逐条经过静音、@、工作状态、全局／设备开关和当前可见上下文策略；压缩唤醒覆盖多条时读取实际新增范围，不只使用最后预览。冷启动和重连历史只同步未读，不逐条补弹；他端已读只刷新状态。按 messageId 去重，并协调同浏览器标签页处理者；不承诺跨设备恰好一次或应用退出后推送。总数通过权威 Query 读取，不做 +1/-1 推算，客户未读不加入当前桌面总提醒。

### 10.14 写入口、守卫与锁序

以下表初始按 `707fdff` 追踪，PR03 基于 `7c64c07` 更新真人单聊、独立 AI 聊天、共享主体和 Agent 执行入口，PR04 基于 `6dc3bdc` 并同步 `8f96135` 更新客服入站、周期管理、Agent 与 Telegram 外发交叉路径；本轮锁序、幂等、停用／归档和任务租约用例已于 2026-09-08 通过服务端全量测试，构建及界面回归记录见 PR 实施清单。当前列记录事务路径，目标列是 PR02–04、PR17–23、PR30–31、PR47 要实现的约束；同步受众、水位及 Outbox 尚未接入。`U` 表示本人或有效内部真人受众，`C` 表示企业客服 Inbox，`V` 表示受影响网站渠道身份的访客目录；V 只允许公开投影。

共用目标顺序：入口守卫／稳定定位 → 按 conversationId 排序锁定业务会话集合 → 每会话的 CustomerConversation／ServiceSession（客服才需要）→ 个人状态 → AgentState → Run → 任务执行记录 → 客服 Inbox 行（organizationId）→ 访客目录行（渠道身份 ID）→ 用户水位行（userId）。仅获取本次需要的锁；多 Agent 按 agentIdentityId、Run 按 runId 排序。拿到受众锁后不得回头获取业务锁。个人置顶顺序版本与本人用户水位共用一行锁，不新增独立顺序锁。

参与关系的顺序按会话路径明确：内部会话在 Conversation 后锁已有 Participant，再处理个人状态和 Agent 状态；客服会话在 Conversation → CustomerConversation → ServiceSession 后处理本次需要的个人状态／AgentState／Run／Task，最后确保回复所需 ChatSubject／Participant 并追加消息。接管不发消息时不建 Participant，AI 被抑制时不提交新的发送关系。所有已有会话的参与者写入都必须先持有同一 Conversation 锁；不得有先锁客服 Participant 再反向等待周期或 Run 的路径。首次创建主体与会话按稳定唯一键定位后进入对应路径。

PR03 的停用生效边界已确认：目标停用只拒绝之后资格校验的新发送，已通过事务内资格校验的发送可提交；不新增目标账号锁。Agent 停用或 AI 会话归档不取消已经提交的排队、运行中和尚待消费的输入，结果仍写回原 Conversation。真人归档会话只在显式首发时恢复；AI 草稿重试不恢复归档，也不新增归档／恢复操作。幂等重放不能绕过当前发送资格。

共享企业身份主体按身份 ID 升序创建，唯一冲突后使用独立查询读取已提交主体，不通过无变化 UPDATE 取回记录。真人／AI 首发、群创建和增员使用同一创建顺序，群内显示顺序保持原规则。已有会话先锁 Conversation，再进行参与关系和业务写入；新建会话的主体准备发生在创建新行之前。

转交目标身份属于前置业务守卫：LockActiveUser 后先锁定并校验指定目标 OrganizationIdentity，再锁 Conversation／CustomerConversation／ServiceSession，锁后重验周期及转交资格，最后处理 Agent 状态。用户／Agent 停用与资料变更同样先完成身份对象守卫，再进入相关会话集合；不得从会话锁反向获取转交目标身份。渠道凭据、渠道身份及联系人恢复等前置集合按表中入口确定，拿到受众锁后不得再扩展集合。

当前用户写事务先调用 `LockActiveUser`（users 的 `NO KEY UPDATE`）。渠道回调使用渠道凭据、身份守卫，Agent 使用任务上下文与 attempt／worker／租约守卫，不伪造当前用户。任务上下文可以前置解析，但数据库 `LockExecution` 放在会话、State、Run 之后，锁后重新确认租约。Query 信任应用服务已解析身份，校验会话资格但不为读取额外加写锁。

| 入口与实际函数 | 守卫 | 当前锁定对象与顺序 | 目标调整与事实写入 | 受众 |
| --- | --- | --- | --- | --- |
| 真人单聊首发：[SendFirstDirectTextMessageAction.Execute](internal/actions/conversation/direct_conversation.go) | LockActiveUser；同企业活跃真人目标 | 读取规范身份对；新建时按身份 ID 确保主体 → 新 Conversation／唯一关系／Participant；统一锁 Conversation → Participant → 锁后资格 → Message／摘要 → 个人已读 | 唯一冲突后整笔事务重试；显式首发可恢复归档，普通发送和幂等重放需通过当前授权；PR05–06 集中消息追加及序号 | 有效真人 U |
| 真人单聊后续：[SendDirectTextMessageAction.Execute → sendDirectTextMessage](internal/actions/conversation/direct_conversation.go) | LockActiveUser；规范身份对与活跃目标资格 | Conversation → Participant → 锁后资格 → Message／摘要 → 个人已读 | 已通过事务内资格校验的发送允许完成；停用之后重新校验的新发送拒绝，不在会话锁后获取目标账号锁 | 有效真人 U |
| AI 聊天首发／后续：[SendFirstAgentTextMessageAction / SendAgentTextMessageAction](internal/actions/conversation/agent_conversation.go) | LockActiveUser；固定用户与 Agent 归属、活跃 Agent 及有效参与关系 | 草稿按 conversationId 收敛；新建按身份 ID 确保主体并建会话／扩展／Participant；Conversation → Participant → 资格／幂等 → Message／摘要 → 个人已读 → State／Trigger／Run／Task | 同草稿和消息编号重试确认已有结果，不同草稿保持独立；新建会话不取消旧 Run；PR05–06 集中追加与序号 | 所属真人 U |
| 群创建／发送：[CreateGroupConversationAction、SendGroupTextMessageAction.Execute](internal/actions/conversation/group_conversation.go) | LockActiveUser；活跃成员／可发资格 | 创建时头像激活 → 按身份 ID 确保创建者与全部成员主体 → 新 Conversation／Participant；发送经 chatstate.LockGroup 锁 Conversation／Participant 并重查资格 | 保留成员展示顺序；系统与文本追加仍归 PR05–06；创建空群不伪造消息基线，增员时按系统消息设基线，不触发群 Agent | 当前真人 U |
| 群资料／增员：[UpdateGroupConversationAction、AddGroupConversationMembersAction.Execute](internal/actions/conversation/group_management.go) | LockActiveUser；chatstate.LockGroup；群主 | Conversation → 参与者／头像文件或新主体 → 群系统消息／个人阅读基线 | 保留头像激活与关联事务；重入复用 Participant、重设阅读及提及基线，保留静音 | 变更前后真人 U |
| 群移除／转让／退出／解散：[RemoveGroupConversationMemberAction、TransferGroupConversationOwnerAction、LeaveGroupConversationAction.Execute](internal/actions/conversation/group_management.go) | LockActiveUser；群主或本人资格 | chatstate.LockGroup（内调 chatstate.LockMember，依次锁 Conversation／本人 Participant）→ loadActiveGroupParticipant（UPDATE OF cp）→ 参与者修改 → createGroupSystemEvent | PR18 为旧受众写移除；PR47 失权清 pin 并推进本人顺序版本；解散可读则保留，不因列表移除伪造失权 | 变更前后真人 U，移出者仅移除标记 |
| 真人客服回复：[SendCustomerTextMessageAction.executeTransaction](internal/actions/conversation/send_customer_text_message.go) | LockActiveUser；企业及 website／Telegram 外发能力；当前周期负责人规则 | Telegram 先 Prepare 锁 Channel／渠道身份；随后 Conversation → CustomerConversation → ServiceSession → Participant／Message／Delivery → 摘要 | PR04 共用 chatstate 会话／周期锁；保留隐式领取、引用、首次响应、首次回复 Participant、消息与投递同事务 | C、V |
| 客服领取／转交／关闭／重开：[ClaimServiceSessionAction 等 Execute](internal/actions/conversation/manage_service_session.go) | LockActiveUser；周期状态；转交目标 LockActiveCustomerServiceIdentity | 目标身份（转交）→ Conversation → CustomerConversation → ServiceSession → AgentState → Run；提交后取消内存 context | PR04 已将转交目标身份移到 Conversation 前，再锁会话、扩展与周期并复核资格；PR19–20/23 原子提交周期和取消／补触发事实，不能以进程取消代替数据库门禁 | C、V（公开状态） |
| 网站入站：[ReceiveWebsiteCustomerTextMessageAction.executeTransaction](internal/actions/conversation/receive_website_customer_text_message.go) → [ReceiveInboundCustomerTextMessage](internal/actions/conversation/receive_customer_text_message.go) | 公开层解析 Cookie/Header；渠道启用、渠道身份及线程归属；无当前用户 | EnsureChannelIdentity 锁渠道身份 → 联系人恢复／主体 → 创建或锁 Conversation → CustomerConversation → ServiceSession → Participant → Message／摘要 → ScheduleCustomerAuto | PR04 已稳定定位渠道身份后锁 Conversation；联系人 Participant 在周期后，AI 回复 Participant 在 State／Run／Task 门禁后，均受同一会话锁串行保护；幂等消息不重复 Trigger，首发前初始化不建业务记录 | C、该身份 V |
| Telegram 入站：[ReceiveTelegramWebhookAction.Execute](internal/actions/channel/receive_telegram_webhook.go) → ReceiveInboundCustomerTextMessage | Preflight 后事务内重验当前 Secret、启用状态；无当前用户 | Telegram 设置（UPDATE OF tcs）→ 渠道身份 → 联系人／主体 → 锁单线程 Conversation → CustomerConversation → ServiceSession → Message／摘要 | PR04 已补会话前置锁并交叉检查凭据、真人外发与投递任务路径；保留来源幂等及来源时间，PR06 按本地 message_seq 展示，不创建 Telegram Agent Trigger | C |
| Agent 调度：[Scheduler.Schedule / ScheduleCustomerAuto](internal/actions/agentrun/schedule.go)、[customer_schedule.go](internal/actions/agentrun/customer_schedule.go) | 继承调用方事务守卫；客服负责人、渠道及 Agent 资格 | AI 聊天继承已锁 Conversation／Participant，个人已读在 State 前；State 分配 Trigger，再写 Run／Task；客服先锁 Conversation → CustomerConversation → ServiceSession | Trigger、Run 与 task_outbox 原子提交；客服会话前置锁由 PR04 实现 | AI 聊天 U；客服 C、V（公开状态） |
| AI 开始／认领／成功／失败：[ExecuteAction.begin / complete / fail / FinalizeFailure](internal/actions/agentrun/execute.go)、[databaseInputFeed.Claim → lockAgentRun](internal/actions/agentrun/input_feed.go) | 可靠任务上下文；锁后 LockExecution 校验尝试与租约；无当前用户 | agentChatRunPolicy 先 Conversation／Agent Participant；客服策略先 Conversation／CustomerConversation／ServiceSession；随后 State → Run → Task，锁后判断终态 | 成功 text 或失败 agent_error、Run 终态及消费水位同事务；停用／归档不取消 AI 聊天已提交输入；客服会话前置锁由 PR04 实现，同步版本归 PR23 | AI 聊天 U；客服 C、V（公开投影） |
| 客服 AI 写回／取消：[customerRunPolicy.prepareLocked / persistMessage](internal/actions/agentrun/customer_execute.go)、[CancelForServiceSession](internal/actions/agentrun/cancellation.go) | 锁后核对 serviceSessionId、负责人、website、Agent 资格、Revision、消费边界；取消继承成员事务 | Conversation → CustomerConversation → ServiceSession → AgentState → Run；执行写回另锁 Task，取消不锁 Task；persistMessage 确保 Participant、写 Message 和双摘要 | PR04 已先锁 Conversation，再按客服扩展／周期 → State → Run → Task 锁后复核资格，最后确保回复 Participant；抑制结果不建关系。接管先提交则旧结果被抑制，反向只保留已提交一次回复；跨进程取消只是加速 | C、V；内部过程不进入 V |
| 个人已读／提及／静音／手动未读：[MarkConversationReadAction](internal/actions/conversation/mark_conversation_read.go)、[MarkConversationMentionReviewedAction](internal/actions/conversation/conversation_mentions.go)、[UpdateConversationNotificationSettingsAction](internal/actions/conversation/update_notification_settings.go)、[UpdateConversationUnreadMarkAction](internal/actions/conversation/update_unread_mark.go) | LockActiveUser；当前会话可读资格 | 内部普通已读、提及和手动未读先锁 Conversation；客服已读及静音仍需补锁后资格检查；更新个人状态／提及确认记录 | PR02/17/19 统一 Conversation → 个人状态 → 本人水位；阅读不创建 Participant 或改变负责人，不推进活动序，不广播其他用户 | 本人 U，客户共享 C 不因个人已读推进 |
| 个人置顶／取消／移动（PR47 新增） | LockActiveUser；目标／邻居当前可读且满足置顶条件；expectedPinOrderVersion | 当前没有写入口或顺序字段 | 锁所涉 Conversation（ID 排序）及个人状态 → 本人 user_sync_states；校验版本后写 rank／顺序版本／用户水位与 Outbox，隐藏项不被全量覆盖 | 本人 U |
| 真人资料／头像：[UpdateProfileAction](internal/actions/user/update_profile.go)、[UpdateUserAction](internal/actions/user/update_user.go)；[Agent 资料](internal/actions/agent/update_agent.go)；[联系人资料](internal/actions/contact/update_contact.go)；[渠道资料](internal/actions/channel/update_message_channel.go) | LockActiveUser；对象企业与文件用途 | 各自业务对象及头像文件锁／UPDATE；尚无会话集合与同步写入 | PR22 按实际 Query JOIN 列出字段依赖；稳定定位对象及相关会话集合，完成业务锁后最后锁受众；只失效当前资料，不改消息来源快照或活动序 | 受影响 U／C；V 仅公开资料 |
| 系统头像完成：[refreshTelegramContactAvatar → applyTelegramContactAvatar](internal/actions/channel/receive_telegram_webhook.go) | 消息事务后独立入口；企业、渠道身份、文件用途及有效状态；无当前用户 | 事务外下载／导入 → 新事务渠道身份 → 按 ID 锁新旧文件 → 新文件激活／头像引用 → 旧文件回收 | PR22 在最终落库事务锁相关会话并写变化；createdByUserId 是文件归属信息，不能当成当前用户守卫 | C；不向其他渠道访客泄露资料 |
| 用户／Agent 停用及工作状态：[user/update_status.go](internal/actions/user/update_status.go)、[agent/update_status.go](internal/actions/agent/update_status.go)、[user/update_work_status.go](internal/actions/user/update_work_status.go)、[agent/update_work_status.go](internal/actions/agent/update_work_status.go) | LockActiveUser；目标企业及各自业务守卫 | 用户／Agent、OrganizationIdentity、ResetRoutingTarget 的渠道行 | PR21–22/30 纳入资格及资料变化；目标用户、转交目标与渠道锁必须在受众前处理，不能持有水位锁再调用 ResetRoutingTarget | 本人会话撤销及受影响 U／C／V 的授权投影 |
| 渠道停用／凭据／公开设置：[UpdateMessageChannelStatusAction](internal/actions/channel/update_message_channel_status.go)、[UpdateTelegramChannelStatusAction](internal/actions/channel/update_telegram_channel_status.go)、[SaveTelegramConnectionAction](internal/actions/channel/save_telegram_connection.go)、[网站访问设置](internal/actions/channel/update_website_channel_access.go) | 成员事务 LockActiveUser；Telegram 另有渠道／Bot advisory 串行锁 | Telegram 外层渠道锁 → 按稳定 Bot ID 锁 → 事务用户守卫 → 渠道及设置；网站更新渠道／设置 | PR04/31 与回调 tcs → 渠道身份路径一起检查，不把外层串行锁移到受众后；渠道失效写撤销控制，访客停止公开读取与旧流 | C、受影响 V；控制不合并成普通水位 |
| 退出登录／偏好：[LogoutAction.Execute](internal/actions/auth/logout.go)、[UpdatePreferencesAction.Execute](internal/actions/user/update_preferences.go) | Logout 按企业与令牌 hash 定位，不接收当前 Identity；偏好使用 LockActiveUser | Logout 删除对应 token；偏好更新本人 User | PR30 为撤销建立同事务 tokenSessionId 控制；PR21 通知偏好走本人流，不伪造会话消息或已读 | 本人 U；撤销只影响对应会话凭据 |

锁序交叉检查不能只检查显式 `FOR UPDATE`：INSERT 的唯一冲突等待、UPDATE、头像文件激活、身份停用和渠道路由重置也会取锁。PR03 检查规范身份对与 ChatSubject 的创建顺序；PR04 共用 chatstate 的企业身份主体创建能力，并检查渠道／Bot 外层锁、凭据设置、渠道身份／联系人恢复、转交目标身份和文件前置关系；PR22 检查资料对象到多会话集合的顺序，禁止新增反向获取这些前置对象的路径。文件锁按稳定 ID 排序，网络下载和模型执行在事务外完成。上述现状中尚未满足目标的路径由对应 PR 修正，本次不宣称已经消除死锁。

三条验收追踪：群移除沿 `RemoveGroupConversationMemberAction.Execute → chatstate.LockGroup → chatstate.LockMember → loadActiveGroupParticipant → leaveGroupParticipant → createGroupSystemEvent`；客服接管沿 `ClaimServiceSessionAction.Execute → lockOpenServiceSession → CancelForServiceSession`，提交后再调用 `finishServiceSessionAgentCancellation`；AI 写回沿 `ExecuteAction.complete → lockAgentRun → policy.prepareLocked → policy.persistMessage`，客服策略再进入 `ensureCustomerAgentParticipant / updateCustomerAgentSummaries`。PR03 已用数据库屏障覆盖真人单聊和独立 AI 聊天的发送与执行交错；PR04 已统一客服接管和写回的完整锁序，验证记录见 PR 实施清单。

## 11. 数据隔离与长期规则

所有查询和写入必须显式带当前 `organization_id`，不能只凭资源 UUID 查询。由于项目迁移不创建外键和 `CHECK`，Action 必须在事务内显式校验企业边界、来源类型和关联合法性。

已合入 main 的迁移不得修改、重排、重命名或删除，结构变化新增实际时间生成的增量迁移并提供 Down；同一未合并 PR 的结构调整直接合并为目标最终结构，开发库重建不写入产品过渡迁移。每个迁移必须在同一文件中为其新增或修改的表、每一列、显式索引和具名约束写全简洁中文数据库注释；Up 改变既有字段语义时更新注释，Down 恢复结构时也恢复原注释。`kind`、`type`、`status`、`role` 等枚举型字段必须在列注释中列出当前全部可用值，并与领域值定义保持一致；数据库不为这些枚举增加 `CHECK`，新增枚举值时使用新的向前迁移更新注释。

长期保持：

- 原生单聊和群聊按有效参与者关系授权。
- 客户会话访问与参与者关系分离。
- 第三方账号会话同时校验当前用户、绑定账号和会话映射。
- 联系人只能通过已验证的渠道会话访问外部渠道。
- 受管访客只能访问邀请范围内资源。
- 联邦用户只能访问双方仍然共享的资源。
- 工单内部评论和字段不进入外部消息时间线。
- 群聊存在外部参与者时持续标识。
- 退出或停用成员不物理删除历史发送者关系。
- 已被聊天主体引用的来源记录采用软删、停用或保留投影，不物理删除后留下悬空 `source_id`。
- 邀请、成员变化、第三方账号发送、AI 工具调用和跨企业操作进入审计链。
- `task_runs` 只提供至少一次异步 Action 运行语义，所有 Handler 可重入；消息和外部投递的永久幂等保存在业务表。
- 外部发送的确定成功、确定失败和结果未知必须显式区分，结果未知时禁止自动重发。
- 持久业务命令统一经过 `appservice.Service`；WebSocket 是实时事件与临时控制通道，不能形成第二套 Action、错误、幂等或 ACK 语义。
- WebSocket、Core NATS 和 `realtime_outbox` 都不是业务事实来源。客户端恢复必须使用用户 Mailbox、共享 Inbox、会话同步序号或对应来源协议游标。
- `appservice` 是业务 DTO 的唯一来源，Protobuf Schema 是实时帧的唯一来源；两者不能重复定义完整消息和会话模型。
- 客户端不直接连接 NATS，不为用户或设备创建 JetStream Consumer，也不把实时扇出并入任务工作队列。

## 12. 路线阶段

### 阶段 0：客户会话数据底座（已交付）

建立 `chat_subjects`、统一 Conversation、参与者、双时间消息、客户会话扩展和 `ServiceSession` 所需的 PostgreSQL 表、索引、领域值与 Bun 模型，同时把渠道自动联系人所需的创建用户字段改为可空。

阶段 0 已由独立的数据底座 PR 交付（见第 13.2 节），不包含入站 Action、公开 API 或 Messenger 真实数据接入。管理端手动添加外部联系人的菜单入口在该 PR 暂时隐藏，既有联系人 CRUD 实现保持不变。

阶段 0 不包括客服回复、实时推送、已读状态、Telegram 和 AI 运行表，也不创建阶段 2 的用户 Mailbox、`realtime_outbox`、Protobuf 实时协议、外部投递或客户端同步表。

### 阶段 1：外部客户单聊

阶段 1 按可独立验收的子阶段交付：先在阶段 0 数据底座上完成网站文本闭环，再以 Telegram Bot 验证外部投递，随后扩展其他客服渠道、文件和客服处理能力。`ServiceSession` 随数据底座提前建立，但客户可见列表和历史始终以 Conversation 为线程；领取、接管、同事转交、关闭和重新打开已经交付，指标和满意度仍在后续阶段交付。

#### 阶段 1A：网站客户文本闭环（双方文本轮询闭环已完成）

- 网站访客通过第 4.5 节的渠道 Cookie 恢复身份，第一条有效文本消息创建 `stage = visitor` 联系人、渠道身份和长期客户会话。
- 新草稿第一条有效文本创建 Conversation 和首个 `ServiceSession`；点击已有列表项发送时复用该 Conversation，并复用或续开它的 ServiceSession。访客会话列表以 Conversation 为列表项，ServiceSession 只提供最新处理状态摘要。
- 网站消息 PR 已交付访客发送、Conversation 列表和完整历史读取（见第 13.3 节）；企业成员列表、历史和回复也已由后续独立 PR 交付。访客与成员当前打开的会话均使用 `after` 轮询新增消息，并在页面恢复可见后立即补拉。
- 同一渠道身份可以同时拥有多条正在处理的 Conversation，每条 Conversation 同时最多一个 `status = open` 的 ServiceSession；`conversations.status = active` 不表示客服批次未结束。
- 网站公开请求由现有 `/api` Gin Service 适配到未注册 Wails 绑定的访客应用服务和 Action；访客直接读取 Cervi 消息时间线，不创建 Delivery。
- 本子阶段只实现文本，不包含文件、外部平台投递和统一实时基础设施。

#### 阶段 1B：Telegram Bot 客服私聊（文本入站已交付，外发待实施）

当前工作区已实现人工客服私聊文本外发、投递状态和人工处理；AI、引用和媒体仍不在本次范围。`TxEnqueuer.EnqueueIn` 已由现有任务运行时提供。

- Telegram Bot 是第一种真正调用外部平台收发消息的客服适配器，首版只接私聊。
- 已由 `ReceiveTelegramWebhookAction.Execute` 在锁定当前凭据设置并重新认证后调用共用 `ReceiveInboundCustomerTextMessage`；私聊文本、联系人渠道身份及长期会话已落地。Telegram 人工外发和 Agent 回复尚未支持，各 Adapter 保留验签、来源时间、幂等和平台规则。
- `TxEnqueuer.EnqueueIn` 已用于 Agent Trigger／Run 的原子任务投递，后续外发可复用事务入队边界。
- 增加 `customer_message_deliveries`、`customer_channel_send_gates`、有序扫描器和 `uncertain/needs_review` 状态机。
- 数据库扫描是外发恢复的正确性来源；以 `delivery_id` 为作用域的任务只作为降低延迟的快路径。
- 入站 Echo 只更新投递状态，不重复创建客户消息。
- Telegram Webhook 默认同步受理；只有必须先应答后处理时才增加来源专属 `channel_inbound_events`。
- Bot 群聊、频道和讨论组不进入客户会话；以后如需接入，使用 `group` 会话能力。

#### 阶段 1C：其他客服渠道

在 Telegram Bot 的 Adapter、Delivery、限流和对账边界经过验证后，再接入微信公众号等其他一对一客服渠道。每个平台复用客户会话事实和经验证的外发可靠性边界，但保留来源专属验签、受理、防重、顺序和能力降级。

#### 阶段 1D：文件与客服处理扩展

- 文件消息复用项目既有临时上传流程，保存 Message 关联时在同一事务激活文件；不把文件内容塞入文本正文。
- 已建立的 `ServiceSession` 使用开放/关闭两态和负责人表达排队、领取、接管、同事转交、关闭与重新打开；显式重开后由操作人负责，不增加挂起状态，后续逐步增加指标和满意度。
- `ServiceSession` 仍与长期 `CustomerConversation` 分离，文件读取仍按记录中的存储类型处理。

阶段 1A 使用 HTTP 轮询。若需提前提供实时回复，整体前移 Realtime Gateway、会话同步序号、`realtime_outbox` 和 Protobuf 最小协议，并按第 10.12 节签发访客连接票据；不增加独立的 SSE 或 JSON WebSocket 协议。

### 阶段 2：企业内部聊天与 AI 参与

阶段 2 依次交付成员单聊、基础群聊、内部 AI 员工验证、网站 AI 客服以及统一实时与离线同步。

#### 阶段 2A：成员文本单聊（全端已完成）

- Web、桌面端和移动端已实现成员会话列表、历史、文本发送和 `before/after` 轮询；移动端使用独立详情路由和前台可见性判断。
- 同一企业内同一对允许单聊的内部身份只对应一个长期 `direct` 会话；两个并发创建请求必须收敛到同一会话。
- 单聊严格按有效参与者授权，不包含已读、实时、文件和 Agent 自动响应。

#### 阶段 2B：基础群聊与协作事实

阶段 2B 按以下独立 PR 依次交付，不把创建、成员变化、消息关系和已读事实塞入同一轮改动：

1. G1 基础群聊文本闭环（已交付）：创建群聊和固定初始成员，接入统一收件箱、成员只读资料、历史、文本发送以及 Web、桌面端轮询；首轮只允许有效真人用户，不接入 Agent。移动端已支持真人建群、已有群聊详情和文本收发，群资料编辑与成员查看已接续交付，触屏引用、提及输入继续由独立 PR 交付。
2. G2 群资料与成员管理（已交付）：修改名称、描述和头像，增员、移除、退出、群主转让、参与者行复用，以及成员变化的系统事件和审计边界。
3. G3 引用与 @ 提醒（本次交付）：开放同会话消息引用，增加类型化提醒关系和参与者校验。
4. G4 已读持久事实（本次交付）：使用独立用户会话状态保存已读水位，不把个人视图状态写入参与者关系。首轮覆盖 Web 与桌面端的成员单聊和群聊；移动端和客户会话不进入本阶段。

通知和临时状态可以先通过刷新降级，阶段 2E 再接入统一实时和离线同步。未读实时扇出、用户 Mailbox 和统一实时协议不进入 G4。群聊已支持邀请和管理活跃 Agent（第 13.23 节），群主仍为真人；结构化 @ 仅接受真人，尚无群聊 Agent Trigger、Run 或回复。`@Agent` 与响应策略独立设计交付，不让普通群消息隐含触发 Agent。

#### 阶段 2C：内部 Agent 最小聊天事实与 AI 员工验证

- Agent 作为普通聊天主体加入内部单聊，发给活跃 Agent 的新文本自动触发，并生成一条最终文本 Message；群聊 @Agent 在基础群聊和提醒事实完成后接入。
- 首个 Runtime 精确锁定 Eino v0.10 Alpha，并用纯函数计算器验证 Tool 闭环；运行中到达的新消息必须在下一次 Tool 或模型规划前通过持久 Trigger 补入。
- 客户端通过业务查询和轮询读取最终消息，本子阶段不依赖统一实时、AI 流式帧或离线 Mailbox。
- 本子阶段验收 Agent Participant、Trigger 水位、Run 幂等、最终 Message 和访问边界；Runtime 由 `agent-roadmap.md` 定义。

#### 阶段 2D：网站 AI 客服

- 内部 AI 员工验证通过后立即交付网站 AI 客服。
- Agent 作为客户会话参与者处理网站客户新消息，并把最终回复写入同一条 Cervi 消息时间线；访客通过 `after` 轮询读取。
- ServiceSession 的负责人是唯一客服状态：网站首次入站只在当前负责人是合格 Agent 时创建 `customer_auto` Trigger；转交给 Agent 时，最后消息来自客户则补触发，来自企业身份则等待下一条客户消息。
- queued Run 只冻结起点，运行中到达的新消息在 Eino 下一个 Tool 或模型安全点由同一 Run 的下一 Turn Claim；最终仍只写入一条 Agent Message。
- 人工接管、转交和关闭继续使用现有通用命令，不增加 AI 专属暂停、恢复或接管状态；写入最终回复前重新校验当前 ServiceSession、负责人、Agent 资格、Run Revision 和消费边界，阻止迟到回复。
- 本子阶段只验收网站客户会话边界；Agent Runtime 由 `agent-roadmap.md` 定义，第三方平台仍使用各自的 Delivery。

#### 阶段 2E：统一实时、通知与离线同步

本子阶段实现第 10 章定义的统一实时和同步设计：

- 增加 `conversation_sync_events`、`conversations.sync_seq`、`user_sync_states`、`user_conversation_wakeups` 和共享收件箱水位。
- 新消息、编辑、删除、参与者、会话设置、反应和回执变化与会话 Sync Event、变更前后受众 Wakeup、`realtime_outbox` 在同一事务提交。
- 使用 Protobuf Edition 2024，先交付认证、Hello、心跳、水位、撤销、错误和下线，再按独立 PR 扩展 AI 流等临时能力。
- Realtime Gateway 先内嵌 Server，通过专用 Outbox 向 Core NATS 定向发布；客户端不连接 NATS，JetStream 继续只承担可靠任务。
- 客户端重连或低频校验时先读取 Mailbox/Inbox Head，再按会话序号补拉；超过保留窗口时重新获取快照。
- 小群使用每用户 Fanout，客服收件箱与访客目录使用共享水位；PR41 测量守卫、受众锁和恢复成本，再按证据优化，不提前设人数上限。

会话 Changelog 不作为搜索、通知、AI 或联邦的事件总线；用户 Mailbox 也只是会话变化索引，不保存消息正文。

### 阶段 3：第三方用户账号接入

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

Telegram 首个实现额外验证 TDLib 会话托管、FloodWait、远端历史对账和明文投影边界。阶段 3 不建设通用 OAuth 产品、连接管理页面、账号共享绑定表或数据库能力矩阵；这些能力应在第二个平台或真实共享需求出现后再提炼。

### 阶段 4：受管外部协作

实现合作方企业、受管访客、定向邀请、外部协作门户、受管单聊群聊、文件访问和审计。

### 阶段 5：工单

实现工单状态、优先级、负责人、团队、会话消息关联、内部评论、对外回复、SLA 和自动化。

### 阶段 6：Cervi 企业联邦

实现企业信任连接、联邦身份投影、跨企业单聊群聊、成员与消息事件同步、断线补拉和访客身份升级。同步协议使用独立的联邦 Inbox/Outbox，以对等部署和协议事件编号永久防重；不复用 `task_runs`、客服 Delivery、客户端 Sync Event 或客户端实时 Protobuf Schema。联邦编码届时按服务端协议独立确定。

### 阶段 7：结构化大型协作

根据真实需求选择公开群、公告群、话题模式、独立子讨论空间和显式共享的跨企业工单。大型群 Shared Fanout、会话级订阅引用计数和容量治理按真实规模证据落地。

## 13. 已交付的客户聊天 PR

### 13.1 交付基线

客户聊天当前由六个连续 PR 推进：

1. 客户聊天数据底座 PR：建立客户聊天数据底座并暂时隐藏管理端手动添加外部联系人的入口。
2. 网站访客消息 PR：实现网站访客身份、首条消息事务、真实 Conversation 列表和完整消息历史。
3. 成员客户会话列表 PR：实现未结束客户会话工作队列和消息页中栏骨架。
4. 成员客户会话历史 PR：实现成员按 Conversation 读取和渲染完整文本历史。
5. 客户会话工作区 PR：建立 Conversation 工作区和独立联系人上下文栏骨架。
6. 成员文本回复 PR：实现成员回复、首次回复隐式领取和当前时间线即时更新。

前两个 PR 对应的详细设计文档已随交付删除；成员侧后续 PR 的临时边界记录在 `inbox-frontend-followup-plan.md`，完成对应交付后删除。本文档仍是长期设计基线，后续实现与本文档不一致时，先更新本文档再改代码，不能让实现自行选择另一套语义。

交付前明确修正了早期“一个渠道身份永久一条 Conversation、访客列表使用 ServiceSession 编号”的方案：

- Conversation 是客户实际打开和继续的聊天线程。
- ServiceSession 只是一条客户线程上的客服处理周期。
- 同一渠道身份可以拥有多条 Conversation，并可同时继续这些线程。
- 每条 Conversation 同时最多一个未结束 ServiceSession。
- 访客列表、历史路径和页面状态从第一版开始使用 `conversationId`。

访客列表、历史和发送从第一版开始都使用 Conversation 公开主键，ServiceSession 只表达各线程内部的客服处理周期。

### 13.2 PR1：客户聊天数据底座（已合并）

PR1 创建了六张聊天表：

```text
chat_subjects
conversations
customer_conversations
conversation_participants
service_sessions
messages
```

该 PR 同时：

- 在当前目标联系人建表迁移中把 `contacts.created_by_user_id` 定义为可空。
- 增加聊天领域值和 Bun 模型。
- 未在 `customer_conversations` 上建立渠道身份永久唯一约束。
- 保留 Conversation 级未结束 ServiceSession 唯一约束。
- 通过 ServiceSession 的渠道身份字段保存客服队列和来源边界；PR1 暂时建立的身份级未结束唯一约束已由 PR2 的向前迁移删除。
- 给 Conversation 增加与时间一致的 `last_message_id`。
- 暂时注释管理端“添加外部联系人”菜单入口，保留既有表单、Action 和联系人管理能力。

PR1 的六条聊天建表迁移在文件内写全相关表、列、显式索引和具名约束的中文数据库注释；联系人创建用户的可空语义直接体现在当前目标建表迁移中。

六张新表统一按主键、`created_at`、`updated_at`、其他业务字段的顺序建表，并全部保留两个审计时间；`customer_conversations` 使用 `conversation_id` 作为主键，`messages` 也保留独立于 `edited_at` 的 `updated_at`。PR1 未为统一格式修改任何历史迁移，既有表的字段顺序由后续独立 PR 处理。

PR1 未实现入站 Action、公开接口或 Messenger 真实写入。数据库中不会因为表已经存在而自动产生聊天记录。

### 13.3 PR2：网站访客真实消息闭环（已合并）

PR2 在第一条有效网站文本事务中创建或取得联系人、渠道身份和联系人 ChatSubject，并选择或创建目标 Conversation、参与者、ServiceSession 和 Message。

同一访客的事务锁点是 `contact_channel_identities` 行，不是共享渠道配置行。两个页面使用不同客户端消息编号并发发送空草稿时分别创建 Conversation；不同访客不会互相串行。

网站消息继续使用：

```text
chmsg:<channel_id>:<client_message_id>
```

幂等记录核对联系人主体和客户会话两条完整关系。请求指定的 Conversation 是客户线程意图的一部分；空编号重试返回首次实际创建的 Conversation。

公开路由注册在现有 Wails `/api` Gin Service：

```text
GET  /api/public/website-channels/{channelID}/messenger
POST /api/public/website-channels/{channelID}/messages
GET  /api/public/website-channels/{channelID}/conversations/{conversationID}/messages
```

打开页面、初始化 Token、点击“开始聊天”和输入草稿不创建业务记录。每次点击“开始聊天”都创建本地草稿，首条合法文本成功后才变为新的真实 Conversation。

访客点击任意 Conversation 都读取其完整历史并继续该线程。打开最新 ServiceSession 已关闭的 Conversation 再次发送时，在同一 Conversation 上创建新的 ServiceSession；其他 Conversation 是否正在处理不影响当前线程。

### 13.4 PR3：成员客户会话工作队列（已合并）

PR3 把 `LoadInbox` 从空占位改为真实客户会话查询，按当前企业读取最新 ServiceSession 未结束的 Conversation，并返回联系人、渠道、最近消息预览、时间和客服状态。查询固定返回最近 50 条，暂不按负责人、团队或关闭状态筛选。

前端按消息页原型交付范围纵栏、本地搜索、客户队列子筛选外观、客户会话列表、响应式详情主区和会话头部。队列子筛选仍只改变选中样式，不修改列表数据；主区在 PR3 保留消息占位。

### 13.5 PR4：成员客户会话历史（已合并）

PR4 增加成员会话消息查询：

```text
GET /api/conversations/{conversationID}/messages?before={cursor}&after={cursor}
```

成员查询按当前企业授权 `customer` Conversation，不要求最新 ServiceSession 仍未结束，避免以后读取已关闭会话时改变历史权限。`before` 与 `after` 互斥，游标绑定 Conversation；当前消息按 `(originated_at, source_order, id)` 稳定分页并统一正序返回。

成员消息 DTO 保留消息类型、双时间和真实 ChatSubject 发送者，不复用访客挂件的 `visitor/agent` 二元作者视角。前端在会话主区加载最近一页并使用 `before` 读取更早文本，按消息编号去重并保持阅读位置；`after` 已作为后续 HTTP 增量补拉契约，但本 PR 不自动调用。

### 13.6 PR5：客户会话工作区骨架（已合并）

PR5 以 Helmdesk 收件箱为样式和交互基线，把选中区拆为客户 Conversation 工作区和独立联系人上下文栏。会话头展示联系人、Conversation 标题和最新 ServiceSession 状态；右栏建立资料、AI 助手和业务三个页签，并在宽屏支持拖动调整宽度与收起，在较窄视口使用 Sheet。

时间线仍只读取当前 Conversation，不按联系人拼接其他 Conversation。每条消息显示来源时间、真实发送主体头像和方向气泡；每个 ServiceSession 的 opening message 前显示批次序号、开始时间和状态，单个批次同样显示边界。该 PR 的回复区与处理操作只保留禁用骨架，后续 PR 已接通回复、领取、接管、同事转交、关闭和重新打开，并移除单独的「交给 AI」。

### 13.7 PR6：成员文本回复（本次交付）

PR6 增加 `POST /api/conversations/{conversationID}/messages` 成员客户会话文本消息命令。消息使用 `mmsg:<organization_identity_id>:<client_message_id>` 永久幂等键；Action 在事务中锁定最新开放 ServiceSession，禁止直接回复其他主体负责的批次，并在公共队列首次回复时由当前成员隐式领取。首次实际成员回复建立或恢复 ChatSubject 和 ConversationParticipant，写入 Message、首次响应时间以及 Conversation 和 ServiceSession 摘要。

网站渠道回复直接写入 Cervi 时间线，不创建 Delivery、异步任务或实时事件。前端启用工作区回复区，成功后立即合入成员时间线并刷新收件箱摘要；访客和成员自动补拉仍留给下一 PR。

### 13.8 阶段 2A：企业成员文本单聊（本次交付）

阶段 2A 交付了 Web、桌面端和移动端文本单聊；移动端当前交付边界见本节末尾：

```text
POST /api/direct-conversations
POST /api/direct-conversations/{conversationID}/messages
GET  /api/conversations/{conversationID}/messages?before={cursor}&after={cursor}
```

阶段 2A 交付时发起单聊只接受当前企业的活跃用户身份；阶段 2C 扩展为同时接受活跃 Agent 身份，仍不接受自己、联系人、跨企业或停用身份。当前实现已由 `direct_conversations` 的企业内规范身份对唯一约束收敛首发，替代早期 advisory lock 与 Participant 集合匹配方案；主体按身份 ID 顺序取得，唯一冲突后重试读取会话。已有归档会话只在显式发起时恢复为 `active`。当前真人 direct 与独立 AI 聊天已分开，PR03 的首发与已有会话锁序实现见第 10.14 节，验证尚待执行。

Direct 发送只允许现有双方有效 Participant，不自动加入、恢复 Participant 或恢复归档 Conversation；消息继续使用 `mmsg:<organization_identity_id>:<client_message_id>` 幂等键，且不关联 ServiceSession。成员历史查询按 Conversation 类型严格分叉：Customer 保持企业与客户扩展授权，Direct 要求当前身份是未离开的 Participant；阶段 2A 交付时其他类型不开放，阶段 2B-G1 已按相同 Participant 规则开放 Group。

统一 Inbox 使用 `type + customer?/direct?` 信封。Customer 与 Direct 分别最多读取 50 条，合并后不再截断；Direct 使用最后消息左连接，空会话返回可空预览与时间，合并按 `last_message_at DESC NULLS LAST, id DESC` 排序，空会话沉底。消息发送者的 `sourceId` 明确定义为 `chat_subjects.source_id`：Customer 中联系人靠左、所有企业身份靠右；Direct 中 `sourceId == CurrentUser.identityId` 靠右，对方靠左。

成员收件箱和当前 Customer/Direct 时间线每 3 秒轮询；Web 与桌面端只在消息路由所属工作区标签活动、页面可见且窗口聚焦时运行，移动端只在应用前台可见时运行，恢复前台后立即补拉。空 Direct 在没有 `after` 时轮询无游标最近页，得到第一条消息后安装游标；空增量响应保留旧游标，`before`、`after` 和本地发送结果按消息编号合并，切换会话后忽略旧结果。

阶段 2A 交付时，Web 与桌面端收件箱以 `inbox-sidebar-prototype.html` 为交互基线，范围固定为「全部 / 客户 / 内部」，Direct 属于「内部」且不增加形态子筛选；成员单聊选择器已启用，当时群聊仍显示“即将推出”。Direct 主区不展示客户渠道角标、ServiceSession、转接操作或客户上下文栏。移动端现以「消息／通讯录／我的」为一级导航，消息按「全部／客户／内部」请求真实列表，客户范围支持处理视图与负责人筛选。Customer 保留只读摘要；Direct 与 Group 支持已有会话详情、历史、文本发送和前台轮询。移动端从通讯录的企业成员列表进入只读资料页，再通过“发消息”查找活跃单聊；没有活跃单聊时进入草稿，首次发送才创建或恢复会话。企业成员列表支持姓名或邮箱搜索和下滚自动加载，追加失败在列表底部重试，资料展示姓名、邮箱、所属团队及工作状态；自己的资料和停用成员不提供发消息入口。首次发送成功后替换草稿路由，聊天页返回成员资料，再返回原搜索、已加载成员和列表位置。AI 员工、团队和外部联系人分类仍保留占位。详情通过统一布局隐藏一级导航，返回后恢复来源查询和列表位置。

### 13.9 阶段 2C：Agent Direct、Eino 与计算器 Tool（本次交付）

本 PR 在既有 Direct 上允许选择活跃 AI 员工。用户文本首次持久化时，在同一事务推进 `conversation_agent_states`、创建 `conversation_agent_triggers`、单个活动 `agent_runs` 和隔离 Agent Worker 任务；消息幂等重放不重复触发。Run 成功或失败都推进明确的 `processed_seq`，Task 重试耗尽通过通用终态回调收敛业务 Run，避免活动索引永久卡住会话。

Runtime 通过内部适配层精确锁定 Eino v0.10 Alpha，并优先使用 eino-ext 的 OpenAI 兼容模型组件。开发期 Tool 只有无副作用四则运算 calculator，并支持可控延时测试并发；TurnLoop 轮询持久 Trigger，通过 `Push + AnySafePoint` 并等待 preempt ack，保证 Tool 完成后、下一次模型规划前读取最新会话上下文。calculator 正式发布前删除，该重建方式也不作为未来设备或副作用 Tool 的恢复模型。

Web、桌面端与移动端复用统一收件箱、Direct 时间线和前台轮询，展示 Agent 类型及排队、运行、失败状态；移动端详情继续遵循前台运行与底部安全区约束。本 PR 不实现流式输出、Tool Invocation、审批、设备权限、本地 Agent Runtime、群聊 @Agent 或网站 AI 客服。

### 13.10 阶段 2D：网站 AI 客服（本次交付）

网站访客 Messenger 已在当前打开的 Conversation 中使用 `after` 每 3 秒补拉新增文本；页面不可见或嵌入挂件收起时停止轮询，恢复后立即补拉。访客发送结果与增量结果按 `(originated_at, id)` 有序去重合入，发送不自行推进服务端游标。

当前聊天用 `open/closed + assignee_identity_id` 作为唯一客服状态。网站消息首次持久化且当前负责人是合格 Agent 时，在同一事务创建 `customer_auto` Trigger，并在没有活动 Run 时创建 Run 和可靠 Task；幂等重放、真人负责人和公共队列不触发，Telegram 等第三方渠道也不触发。

Customer Auto 复用 Eino TurnLoop 和安全点 Push，但使用独立的客户 Feed、上下文角色映射与完成门禁。queued Run 只冻结 `trigger_start_seq`；每次 Claim 按当前连续 Trigger 扩展 `trigger_end_seq`，因此 Tool 或模型执行期间到达的客户补充会在下一 Turn 进入同一 Run。最终完成边界之后到达的消息创建下一 Run。本 PR 用结构化日志记录每次 Claim 的前后边界，并通过 Eino 通用 Tool Middleware 记录实际 Tool 名称、调用编号、结果和耗时；临时 calculator 只补充同一调用编号的 `operation` 和 `delay_ms`。本阶段不增加 Step、Tool Invocation 或调用事件持久化，完整可观测事实留给后续独立 PR。

Agent 回复以普通企业身份绑定当前 ServiceSession，使用 `agent:<run_id>` 幂等键，并在同一事务更新首次响应、Conversation 和 ServiceSession 三元组摘要、Run 终态和 `processed_seq`。写回前按 `ServiceSession -> State -> Run` 重新校验当前周期、负责人、website 能力、Agent 客服资格、Run 冻结 Revision 和消费边界；接管、关闭或换负责人后的迟到结果被抑制。

真人与 AI 继续共用领取、接管、转交、关闭和重新打开命令。转交给 Agent 时最后一条来自客户会立即补 Trigger，最后一条来自企业身份则等待客户下一条消息；不增加 AI 专属暂停、恢复、接管、收件箱状态或前端页面。

### 13.11 阶段 2B-G1：企业成员基础群聊（已交付）

本 PR 在既有 Conversation、ChatSubject 和 Participant 数据底座上增加企业成员基础群聊文本闭环，不新增迁移。创建接口接收群聊名称和初始成员，当前身份自动成为 `owner`，其他成员为 `member`；首轮只允许同企业 active user，拒绝 Agent、联系人、停用身份、跨企业身份和重复成员。一个群聊最多包含 100 名初始参与者，同一成员集合可以创建多条群聊，不复用 Direct 的身份对唯一关系。

统一 Inbox 扩展为 `type + customer?/direct?/group?` 信封，Group 进入现有「全部 / 内部」范围，并与 Customer、Direct 各自最多读取 50 条后统一排序；尚无消息的群聊同样可见。成员历史和群聊资料要求当前身份是未退出的 `organization_identity` Participant；群聊资料首轮只读，返回当前有效成员和 owner。文本消息使用独立群聊命令，沿用成员消息幂等键、稳定时间线游标和 Conversation 摘要更新，不关联 ServiceSession，也不创建 Agent Trigger 或 Run。

Web 和桌面端支持创建、列表、只读查看成员、历史、文本发送和前台轮询。移动端已接入群聊列表、真人建群、已有群聊详情和纯文本收发，群资料编辑与成员查看已接续交付；群资料修改、成员变更、系统消息、引用和 @ 已由 G2、G3 接续交付，已读、文件、Agent 成员和统一实时仍留给后续 G4 与阶段 2E。

### 13.12 阶段 2B-G2：群资料与成员管理（已交付）

G2 支持群主修改名称、描述和头像，批量增员、移除成员和转让群主，普通成员可以退出；群主退出前必须转让，最后一位群主可以解散群聊并保留只读历史。成员重新加入时复用原 Participant 行，群名称和成员变化以类型化系统消息记录操作人与目标快照。

Web 和桌面端资料栏提供群资料和成员两个页签，按群主与普通成员权限展示管理操作。退出成员失去群聊访问权；解散后的最后一位成员仍可查看资料、成员与历史，但不能继续发送或修改。

### 13.13 阶段 2B-G3：群聊引用与 @ 提醒（本次交付）

G3 复用 `messages.reply_to_message_id` 保存一层消息引用，并增加独立 `message_mentions` 关系，以 ChatSubject 而不是正文中的显示名称记录提醒事实。引用目标必须是同一群聊中的未删除文本消息，提醒目标必须是当前有效 Participant；消息幂等同时核对正文、引用目标和提醒主体集合。

Web 和桌面端在鼠标移入消息气泡时显示「回复」：他人消息显示在气泡右上角，自己的消息显示在左上角。文本消息右键菜单提供「回复」和「复制文本」；回复摘要显示在输入区和新消息气泡中。输入 `@` 后从当前群成员中选择才会形成提醒关系，纯手写同名文本不产生提醒事实。本阶段不创建 Agent Trigger、未读、通知或实时事件。

### 13.14 当前共同边界与后续交付

G4 使用 `conversation_user_states` 保存真人用户在 Cervi 原生 Direct 和 Group 中的最后已读消息，水位按群聊消息序号或单聊消息三元顺序单调推进。收件箱返回每条内部会话的未读消息数、群聊结构化提醒未读数，以及不受列表条数和当前筛选影响的内部未读总数；已删除消息和当前用户自己发送的消息不计未读，系统消息计入普通未读。

当前阶段不处理历史数据。群成员首次加入或重新加入时，在同一事务把已读水位初始化到本轮增员系统消息，避免离开期间消息形成未读。Web 与桌面端进入会话时加载最新窗口，当前页面激活且可见尾端后推进已读；留在会话并上翻后，新消息只在逐条进入可见区域时连续推进水位，也可以通过新消息按钮或列表右键直接标到最新。消息由轮询还是未来 WebSocket 输入不改变这套已读状态机。本阶段不接入移动端，不增加已读成员展示、未读分割线或通知投递。

当前聊天仍未实现以下能力，全部属于后续阶段：

- 实时、文件、外部平台投递、团队队列、指标和满意度。
- 第三方用户消息账号、受管访客、联邦和完整 AI 客服策略与审计表。
- `customer_message_deliveries`、渠道发送 Gate、`conversation_sync_events`、用户 Mailbox、`realtime_outbox` 或实时 Protobuf Schema。

后续按独立 PR 继续完成：

1. 以独立 PR 接入群聊 `@Agent`。
2. 基于移动端已有群聊创建和文本收发，继续以独立 PR 交付成员添加、移除、独立群主转让及触屏引用、提及输入。
3. 根据聊天主流程需要继续交付未读、统一实时、文件和外部平台投递。
4. 根据真实产品需要增加网站渠道“只允许一个入站会话”的可选策略；默认多会话保持 Conversation 公开主键。
5. 团队队列、指标和满意度按实际需求独立建模。
6. 公开访客端点的限速与其他安全加固按当前阶段约束继续后置，不阻塞聊天功能开发。

### 13.15 会话提醒事实（本次交付）

本 PR 在不接入界面和系统通知的前提下建立内部聊天注意力状态：`conversation_user_states` 增加当前用户的会话静音事实，并允许尚无已读消息的空会话先保存个人设置；Direct 和 Group 共用更新提醒设置命令。会话行的 `unreadCount` 始终表示客观未读，静音不修改已读水位或隐藏客观未读。

Group 消息使用 `messages.mention_all` 保存结构化 `@所有人`，不向 `message_mentions` 展开群成员。显式 `@成员` 继续引用 ChatSubject。群聊的 `mentionedUnreadCount` 同时统计显式提醒和 `@所有人`，同一消息只计一次；消息幂等核对正文、引用、显式提醒集合和 `mentionAll`。

Inbox 顶层同时返回客观未读总数和提醒未读总数。未静音内部会话的全部客观未读进入提醒总数；静音 Group 只有 `@当前用户` 或 `@所有人` 进入提醒总数；静音 Direct 不贡献提醒。所有计数均按当前设置读取，不在消息事务内向成员扇出未读或提醒记录。

Web、桌面端已接通静音菜单和样式、内部范围提醒圆点及桌面应用提醒计数，见第 13.17 节；`@所有人` 输入与展示见第 13.18 节。静音策略、全局消息通知开关和工作状态共同控制本地系统通知的投递，当前轮询阶段按 `inbox-notification-followup-plan.md` 交付，实时接入时按第 10.13 节区分 live／catchup／bootstrap 并保持既有通知策略。


### 13.16 群聊引用跳转与提及导航（本次交付）

群聊引用可以定位原消息；待查看提及通过独立的 `last_reviewed_mention_message_id` 保存，与普通已读、静音和通知开关分开。提及查询合并显式提醒与 `@所有人`，排除本人和已删除消息，并返回本轮固定的消息 ID 序列。当前目标在激活页面中实际可见后，正常浏览与主动导航都会确认实际可见的提及，屏幕外目标继续保留。单条查看记录支持乱序确认，再合并到不越过待查看目标的连续水位；重复确认幂等，已删除目标可以跳过。首次增员和重新入群同时把两个水位设到本轮增员事件，保留静音设置。

消息页共用双向边界契约，定位查询返回目标及前后各 25 条消息。Web 与桌面端时间线区分最新窗口和锚点窗口：窗口内引用与提及定位保留当前数据窗口，完整可见时只高亮；屏幕外定位暂停跟随。窗口外上下文仍有后续消息时进入锚点，前后分页保持连续，期间不拼入最新轮询结果、不自动贴底、不推进普通已读。补页到尾端恢复连续增量读取，保留当前位置；回到最新成功后恢复跟随；在锚点发送时先读取最新页，失败保留草稿和位置。已删除的引用返回删除标记，不返回正文。

导航保留本轮分母和回看位置；引用可以暂停未确认项，通过“继续查看”恢复。跨设备确认越过本轮上界后结束旧轮次。交互采用整行短暂高亮，减少动态效果设置下显示静态高亮；定位代次隔离过期响应。实现和验证细节见 `todo.md`。

### 13.17 内部会话静音、未读标记与提醒计数（本次交付）

Web 与桌面端的内部单聊、群聊列表右键菜单提供静音和取消静音，空会话与已读会话同样可以操作。静音图标显示在会话时间下方、消息摘要右侧，客观未读徽章使用灰色；群聊提及标记继续来自 `mentionedUnreadCount`。

`conversation_user_states.marked_unread` 保存个人独立未读标记。标为未读不回退普通已读或提及查看水位，没有真实未读时显示圆点，有真实未读时继续显示数量。停留在当前会话时，自动阅读后续消息只推进真实水位，不清除主动设置的标记；再次进入会话、重新点击当前会话或主动标为已读时清除。主动标为已读通过 `clearUnreadMark` 在推进水位的同一事务清除标记，空会话只清除标记。

内部范围圆点和桌面应用未读计数统一读取 Inbox 的 `attentionUnreadCount`，会话行及 Inbox 的 `unreadCount` 继续表示客观未读。未静音会话的提醒数取客观未读与个人标记（1）中的较大值，不重复计数；静音单聊不贡献提醒，静音群聊仍只贡献提及未读，手动标记不制造提及。会话列表和无条数限制的总数共用单聊身份对、群聊成员及未读查询，内部提醒总数归零时隐藏范围圆点。

保存后失效统一 Inbox 查询，重新读取当前策略下的提醒总数，保持列表排序、选择和滚动位置。新增字段通过独立增量迁移建立，不回填旧数据。本次不接入 `@所有人` 输入、本地系统通知投递、移动端和统一实时；提醒数据仍按现有前台轮询与刷新机制更新。

### 13.18 群聊 `@所有人` 输入与展示（本次交付）

Web 与桌面端群聊输入 `@` 后，候选首项提供「所有人」，支持键盘和鼠标选择。选择后与 `@成员` 一样在正文光标处插入标记，不增加独立标记行。每份草稿最多一个结构化所有人标记；编辑或删除选中标记后取消提醒，其他同名手写文本不会接管提醒。手写或粘贴相同文字不形成结构化提醒。

发送接口直接使用已有 `GroupTextMessageInput` 传递正文与 `mentionAll`；发送中、失败和已保存消息复用成员提及的正文加粗、下划线样式，保留发送时的标记文字及位置。展示支持已配置语言的标记拼写，但提醒事实只来自结构化字段，不从正文推断。失败恢复保留提醒状态和草稿内位置；内容相同的重试沿用客户端消息编号，改变 `mentionAll` 后使用新编号。切换会话沿用线程卸载机制清理草稿，标记不跨会话保留。

提醒计数、静音群提醒和提及查看继续复用既有服务端事实；同一消息显式 `@成员` 与 `@所有人` 只计一次，不展开成员提醒记录。本次不新增业务接口或迁移，不接入系统通知、移动端群聊、`@Agent` 和统一实时。


### 13.19 内部单聊引用回复与原消息定位（本次交付）

Web 与桌面端的内部单聊支持引用同会话中自己或对方的未删除文本消息，沿用群聊的回复入口、输入区摘要、取消回复、气泡摘要与原消息定位。定位到历史窗口时不推进普通已读、不拼入最新轮询结果；回到最新后恢复跟随，锚点中发送前先读取最新窗口。引用、提及、分页和返回最新共用视口控制；“回到最新消息”只按实际离底距离及后续消息显示，返回成功后清理旧导航并跟随最终布局。提及轮次已在尾端时显示“结束查看”。

单聊发送契约增加 `replyToMessageId`，复用 `messages.reply_to_message_id`。单聊和群聊共用引用目标校验与成员消息引用幂等核对；同一客户端消息编号的正文或引用目标变化会产生冲突。发送失败保留正文和引用，相同内容重试沿用编号，修改引用后使用新编号。已保存引用的目标被删除后只返回删除标记，不返回正文和发送者；重放已保存消息不会因引用目标随后被删除而重复写入。

Agent 单聊的模型输入为带引用消息附加一层结构化 `replyTo`，包含原消息编号、发送者身份、名称与正文，即使目标已超出最近 100 条历史也保留引用上下文。引用不创建额外的模型对话角色、不递归展开引用链；已删除目标仅提供编号和删除标记。普通消息继续使用原正文，Agent Trigger、运行调度和最终回复机制保持原有语义。

本次不增加数据库迁移，不扩展客户会话引用、移动端交互、消息编辑与撤回、话题串、群聊 `@Agent`、通知或统一实时能力。

### 13.20 客服会话个人已读与未读提示（本次交付）

Web 与桌面端客户会话列表显示当前用户的文本未读数量，包括客户、其他真人和 AI 员工的未删除文本，不统计本人发送。个人水位复用 `conversation_user_states`，按用户和长期 Conversation 保存；首次查看前的文本均计入未读。客户列表各处理视图与「全部」范围中的同一会话使用相同个人状态。

客服阅读沿用当前企业内的客户历史访问规则，不要求当前用户成为参与者或负责人，不因阅读创建参与者、领取或转交会话。页面激活且消息在最新连续窗口中实际可见时，沿用现有阅读逻辑推进水位；列表右键支持主动「标为已读」。历史锚点不自动推进普通已读。不同客服互不代读，同一用户在 Web 和桌面端通过既有查询刷新同步结果。

领取、转交、关闭、显式重开与客户新消息开启下一处理周期均不重置个人阅读状态。标记重复提交幂等，旧目标不回退水位；读到的目标后来被删除仍保留水位，已删除文本不计未读。网站入站和人工回复在取得当前客服周期锁后生成本地消息时间，避免较早开始却较晚提交的请求落到已读及增量读取水位之前；外部来源时间仍保留来源语义，外部平台投递与统一同步顺序继续按各自阶段交付。

本次不增加数据库迁移，不增加客户手动未读标记、静音、客户范围总提醒、系统通知、访客已读回执或移动端阅读交互。Inbox 顶层客观未读与提醒总数仍只汇总内部会话，客户会话行的个人未读不进入桌面应用提醒计数。

### 13.21 移动端已有群聊详情与文本收发（本次交付）

移动端从「全部／内部」消息列表打开已有群聊，详情通过群资料接口独立读取名称、头像、有效成员人数和解散状态，不依赖收件箱最近列表判断会话是否存在。返回时恢复原筛选和列表滚动位置，详情沿用移动端安全区和隐藏一级导航的布局。

群聊复用统一消息窗口、文本发送与幂等失败重试，展示发送者、系统消息、已有引用及提及正文。已有引用可定位原消息，离开消息尾端时提供“回到最新”入口，发送前恢复最新窗口；本次不提供新增引用回复、结构化提及输入或提及导航，也不提交普通已读或提及查看确认。群资料和最新消息仅在前台轮询，恢复前台立即同步；历史分页和新消息到达保持当前阅读位置，读取失败保留已有内容并提供重试。

已解散且仍有查看权的群保留只读历史并关闭发送区，失去访问权时提示并返回原消息列表。发送失败后刷新群资料，及时同步成员访问资格和解散状态。本次不调整后端契约或数据库，不交付创建群聊、群资料编辑、成员管理、未读徽章、静音或系统通知。

### 13.22 网站客服与访客引用回复（本次交付）

Web 与桌面端客服可以引用当前网站客户会话中未删除的文本消息，使用已有回复入口、输入区摘要、取消回复、引用气泡与原消息定位。引用属于长期 Conversation，允许引用较早客服处理周期的消息；发送仍按当前开放周期及负责人校验。历史定位继续使用共同的锚点窗口和阅读规则，不因定位原文推进已读。

客户回复契约增加 `replyToMessageId`，复用 `messages.reply_to_message_id` 和成员消息幂等核对。同一客户端消息编号的正文或引用目标变化产生冲突；发送失败保留正文和引用，相同内容重试沿用编号。引用摘要补齐客户渠道身份的名称和头像；原消息删除后，读取和幂等重放仅返回删除标记。

网站 Messenger 展示客服回复携带的一层原文摘要，以「你 / 客服」表达发送方，不公开企业内部身份。访客与客服采用相同的引用交互：收到的文本消息悬停显示回复按钮，双方消息均支持右键引用；输入区展示原文摘要并可取消，发送失败保留正文和引用，切换会话分别保留草稿。点击摘要定位原文，窗口外原文通过已有 before 游标补齐历史，保持阅读位置并提供回到最新消息的入口。访客发送校验当前渠道身份、目标会话和原文有效性，幂等重试包含引用编号。网站 AI 客服读取历史时保留引用消息的一层结构化正文、发送者类型和来源编号，即使原文已超出最近 100 条上下文；引用不增加模型对话角色，已删除目标只保留编号与删除标记。

本次不新增迁移，不扩展 Telegram 外发、移动端引用交互、文件、编辑、撤回或实时同步。

### 13.23 群聊邀请 AI 员工（本次交付）

Web 与桌面端创建群聊和添加成员支持同企业的活跃 Agent，候选和成员列表展示 AI 员工标识及头像。成员资料返回身份类型，Agent 作为普通成员加入，可移除并重新添加；停用及跨企业 Agent 不可加入。

群主仍由真人担任，不能转让给 Agent；最后一位真人可以解散仍包含 Agent 的群聊。群内结构化 @ 候选及服务端提醒目标仅接受真人，既有真人 @成员和 @所有人功能保持原有语义。

本次只交付成员邀请和管理，不创建群聊 Agent Trigger 或 Run，不提供群内 Agent 回复、抢占、上下文组装或运行状态展示；群内 @Agent 和 @所有人触发 Agent 的规则留待后续设计。不增加迁移，不扩展移动端建群和成员管理。

### 13.24 移动端创建内部群聊

移动端消息页右上角提供加号菜单，菜单内可“发起群聊”。独立创建页左上角返回，右上角“完成”；群名称为空、仅含空白或未选择初始成员时禁用“完成”，提交期间同样禁用。填写群名称、搜索并多选有效真人成员；创建者自动加入，额外选择 1–99 人。搜索切换保留已选成员，已选区可直接移除成员。创建成功后替换表单路由进入已有群聊详情，立即复用文本收发；返回恢复来源列表筛选与位置。提交前返回不写入群聊，失败保留表单并展示错误，离开页面后忽略在途结果。

本次复用既有企业身份候选、群聊创建接口和数据读取规则，群资料补充当前用户的静音状态，不新增迁移。建群表单仍只选择真人，群头像、简介、资料编辑和包含 AI 的成员查看由后续群详情功能接续交付，成员添加与移除仍待后续实现。移动端仍不提供引用／提及输入、已读、通知和实时同步。

### 独立 AI 聊天

AI 聊天使用 `conversations.type = agent`，通过 `agent_conversations` 固定所属成员身份与目标 Agent 身份。同一企业内同一成员与同一 Agent 可以拥有多个 Conversation，身份组合没有唯一索引。双方同时写入统一参与者表，消息读取和发送按企业及有效参与者授权。真人 `direct` 保留 `direct_conversations` 的规范化身份对唯一约束，查找和发送入口不再接受 Agent。

Web 与桌面端消息页的加号提供「发起单聊 / 创建 AI 聊天 / 创建群聊」。AI 选择器仅列出活跃 Agent，选中后每次进入新的本地草稿；不查找历史会话，不创建数据库记录。草稿预生成稳定 `conversationId`，首次发送携带该编号、目标 Agent、`clientMessageId` 和正文，在一个事务中创建 Conversation、扩展记录、参与者、消息、个人阅读状态、Trigger、Run 与可靠任务。事务失败全部回滚；响应丢失后沿用会话和消息编号重试，确认已有结果。编号已存在时核对企业、所属用户、目标 Agent 和会话类型，消息重试核对正文和引用。

每个 AI 会话分别进入「全部 / 内部」列表，标题取首条消息归并空白后的前 40 个字符，列表和会话头同时展示 Agent 名称。组件身份、消息缓存、未读和运行状态按 Conversation 隔离；草稿转正式会话时保持组件身份，离开草稿后的迟到结果仅刷新列表。移动端支持已有 AI 会话的列表、历史和文本收发。草稿保留范围沿用当前页面，不建立服务端草稿状态。

新建 AI 会话不关闭旧会话或中止旧 Run。任何已有 AI 会话都可以继续聊天，模型仅读取当前 Conversation 的历史和引用；Agent 配置与知识库能力由正常执行配置提供。不创建 ServiceSession、上下文重置标记或历史兼容分支。

### 移动端群详情与群管理

移动端群详情顶部以每行 5 个头像展示群成员，头像下方显示姓名，最多两行。群主最多展示 8 位成员，末尾保留「添加」「移除」图标；非群主最多展示 9 位成员，末尾保留「添加」图标。本轮图标仅展示，不接入操作。下方「查看群成员(N)」和右箭头进入独立成员页，支持姓名搜索、完整名单与群主标识，不提供成员管理操作。群资料与个人免打扰放在成员区下方，不额外显示「群资料」标题。头像、名称、描述各自显示为独立资料行并保留右侧箭头；群主点击后进入带标题和标准返回按钮的独立编辑页，编辑页仅保留全宽「完成」按钮，左上角返回放弃未提交的编辑并恢复详情位置，提交期间锁定返回。普通成员点击资料项时提示仅群主可修改；已解散群提示不可修改。图片选择后立即上传为临时文件，保存时关联。群主、创建时间为只读字段，免打扰只修改当前用户设置，使用独立保存状态，点击立即切换、失败恢复原值，不禁用其他资料项。

免打扰下方，普通成员显示红色「退出群聊」按钮并在确认后退出；群主显示禁用的红色「解散群聊」按钮，暂不接入解散操作。已解散群保留资料、成员和历史。

群聊、详情、成员列表与分项编辑采用嵌套路由，隐藏的页面保持挂载和布局尺寸，同时禁用交互与消息轮询，返回后恢复草稿和阅读位置。群资料在应用前台同步；主动退群期间暂停群资料轮询与自动访问失效提示，退出后按群子页记录的历史距离回到来源消息列表并刷新；直接打开的群子页通过替换路由返回所属页面。接口、业务权限和数据库沿用现有契约，不扩展移动端已读、提及、通知投递或实时协议。
