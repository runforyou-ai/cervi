# Cervi 聊天与协作路线图

## 1. 文档目的

本文档确定 Cervi 从客户会话、企业内部聊天，逐步扩展到受管外部协作、工单和跨企业联邦通信的产品路线与核心模型。

本文档同时定义第一个开发 PR 的实施边界。后续开发如需改变本文中的核心概念、对象边界或阶段顺序，应先更新本文档并说明原因。

## 2. 产品背景与约束

Cervi 以私有化部署为主。即使由 Cervi 官方运营 SaaS，也为每个企业分配独立域名，并保持清晰的企业数据边界。

前期重点场景：

- 外部客户通过网站等渠道与企业客服单聊。
- 企业内部成员单聊。
- 企业内部成员群聊。

中期重点场景：

- 未部署 Cervi 的供应商、承包商或客户，通过企业托管的邀请链接和外部协作门户参与单聊、群聊和工单。
- 客户会话和供应商协作中的问题可以转成工单持续处理。

长期重点场景：

- 两个或多个独立部署 Cervi 的企业建立可信连接。
- 各企业成员使用自己企业中的原生身份进行跨企业单聊和群聊。
- 各部署在保持本地身份、权限、审计和数据策略的前提下同步共享会话。

当前阶段不为历史接口和历史数据保留兼容逻辑。模型应直接采用目标结构，但不提前实现尚未验证的复杂功能。

## 3. 产品设计结论

### 3.1 借鉴统一会话模型，不照搬 Rocket.Chat 的产品概念

Rocket.Chat 将频道、团队、讨论、私信、话题等能力统一在 Room 模型之上。Cervi 应借鉴这种底层统一方式，但不在首期向用户同时暴露频道、私有频道、讨论组、团队和话题等大量概念。

国内企业用户更熟悉“单聊、群聊、客户会话”。因此 Cervi 的首要用户概念保持为：

- 单聊
- 群聊
- 客户会话

公开群、话题模式和独立讨论空间属于后续增强能力，不作为首期基础分类。

### 3.2 “渠道”只表示外部消息接入源

Cervi 中的“渠道”仅表示网站、微信、邮件等外部消息接入源，不用来表示企业内部聊天室。

这一约定避免“内部频道”和“外部渠道”在中文界面、接口与代码中产生歧义。

### 3.3 跨企业不是新的聊天类型

会话类型只表达沟通形态：

```text
direct    单聊
group     群聊
customer  客户会话
```

内部、受管外部和联邦通信由参与者身份及所属企业推导，不增加 `cross_org_direct`、`cross_org_group` 等会话类型。

例如：

- 两名本地用户参与 `direct`：企业内部单聊。
- 一名本地用户和一名受管访客参与 `direct`：受管外部单聊。
- 多个本地用户和受管访客参与 `group`：受管外部群聊。
- 来自多个 Cervi 部署的原生用户参与 `group`：联邦群聊。

### 3.4 公开性和消息组织方式使用正交属性表达

群聊未来可按需增加：

```text
visibility = invite_only | organization_discoverable
mode       = chat | topic
```

- `invite_only` 是符合国内使用习惯的默认方式。
- `organization_discoverable` 表示企业内可搜索、可申请或主动加入的公开群。
- `chat` 表示普通时间线。
- `topic` 表示以话题聚合内容。

普通会话中的引用回复和话题串通过消息关系实现，不需要创建独立会话。

Rocket.Chat 式带名称、成员和独立生命周期的 Discussion 暂不实现。只有在项目协作、事件处理等实际需求无法被群聊和话题串满足时，再通过父子会话关系增加。

## 4. 核心业务对象

```text
身份主体 Identity
    │
    └── 参与会话
            │
Conversation ── ConversationParticipant
    │
    ├── Message
    │     ├── 引用消息
    │     └── 话题根消息
    │
    └── CustomerConversation
            │
            └── ServiceSession
                    │
                    └── 客服排队、分配、转接和响应指标

Ticket ── TicketConversationLink ── Conversation / Message 范围
```

### 4.1 Conversation

`Conversation` 是稳定的消息容器，负责：

- 会话类型。
- 标题及群聊配置。
- 参与者集合。
- 消息时间线。
- 归档状态。
- 本地数据归属。
- 未来联邦通信所需的全局标识和来源信息。

`Conversation` 不负责：

- 客服排队和接待状态。
- 工单优先级、SLA 和解决状态。
- 外部渠道连接配置。
- 用户登录认证。

### 4.2 CustomerConversation

`CustomerConversation` 是 `Conversation` 的客户会话扩展，显式关联产生该会话的渠道身份。

它负责：

- 关联通用会话与 `contact_channel_identities`。
- 保证同一渠道身份只对应一个长期客户会话。
- 为客户会话列表提供联系人和来源渠道的可靠查询路径。

它不负责客服接待状态。同一个联系人拥有多个渠道身份时，每个渠道身份保留独立客户会话；跨渠道历史在联系人或工单层聚合，不自动合并消息时间线。

客户会话扩展使用真实关系字段，不解析字符串键获取渠道身份，也不把渠道专属字段直接加入通用 `conversations`。

### 4.3 ConversationParticipant

参与者关系表示某个身份主体参与会话，并保存：

- 身份类型及身份编号。
- 会话角色。
- 加入和离开时间。
- 最后已读位置。
- 后续可扩展的提醒设置。

参与者退出后不物理删除关系，避免历史消息失去发送者上下文。

参与者关系表达消息发送主体和显式会话成员，不是所有会话类型的唯一访问控制来源：

- 内部单聊和群聊按有效参与者关系授权。
- “我的内部会话”查询必须显式限定 `type IN (direct, group)`，不能把参与者关系作为所有会话类型的统一列表入口。
- 客户会话在当前阶段按企业登录身份读取，后续按 `ServiceSession` 的排队、负责人和协作者授权。
- 未发言、未分配的客服不因能够查看客户收件箱而写入参与者表。
- 客服实际回复或明确加入协作时，才建立对应的 `user` 参与者关系。

### 4.4 Message

消息属于一个会话，由会话参与者发送。首期只实现文本消息，但模型预留：

- 消息类型。
- 引用消息。
- 话题根消息。
- 来源或客户端幂等编号。
- 编辑和删除时间。
- 联邦来源信息。

文件、语音、消息卡片、表情反应和系统事件后续通过独立关系或消息内容扩展实现，不在首个 PR 中一次性加入。

### 4.5 ServiceSession

`ServiceSession` 表示一次客服处理过程，与长期消息容器分离。

它负责：

- 等待、处理中、挂起和结束状态。
- 当前负责人和负责团队。
- 排队、首次响应、转接和结束时间。
- 满意度和客服指标。

一个客户会话可以先后产生多个服务批次。内部单聊和内部群聊不创建服务批次。

### 4.6 Ticket

工单是问题处理与流程对象，不等同于会话。

工单负责：

- 主题、状态、优先级和分类。
- 负责人、负责团队和协作者。
- SLA、截止时间和流转记录。
- 内部评论与对外回复。
- 自动化规则。

会话和工单使用多对多关系。一个长期会话可以产生多个工单，一个工单也可以关联多个渠道或内部会话。

建议关联结构：

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

`source_message_id` 记录从哪条消息创建工单，消息范围用于表达长期会话中与工单相关的上下文。

跨企业共享会话不表示共享工单。每个企业可以为同一个共享会话创建自己的内部工单，工单状态、负责人、SLA 和内部评论默认不向其他企业公开。未来的协作工单必须通过显式共享策略实现。

## 5. 身份模型

### 5.1 身份类型

聊天参与者需要支持以下身份：

```text
user             本地企业成员
guest            当前企业托管的外部协作者
federated_user   其他 Cervi 部署中的原生企业成员
contact          网站、微信等渠道中的外部联系人
bot              AI 或自动化机器人
system           系统身份
```

业务层统一使用 `ParticipantIdentityType` 表达可加入会话的身份类型。该概念与现有仅面向企业内部团队成员的 `MemberIdentityType` 分开，避免扩大团队成员语义。

`system` 是逻辑消息来源，不加入会话参与者表；系统消息的发送参与者为空，并由消息类型表达来源。

### 5.2 本地用户、访客、联邦用户和联系人的区别

- 本地用户拥有当前企业的完整登录身份，并按企业权限访问工作台。
- 受管访客由当前企业托管，只能访问被邀请的外部协作资源。
- 联邦用户由远端 Cervi 部署认证，当前服务器只保存必要的远端身份投影。
- 联系人是外部渠道中的客户身份，不自动拥有登录外部协作门户的能力。

同一个自然人可能同时拥有联系人身份、受管访客身份和联邦身份。系统不按手机号或邮箱自动合并这些身份，只在经过验证后建立身份关联。

### 5.3 全局身份与域名

未来联邦身份至少由以下信息定位：

```text
home_server_id
home_organization_id
remote_identity_id
```

企业独立域名用于发现和路由，但不作为不可变数据库主键。域名可能发生变更，稳定身份由服务器编号与远端身份编号组成。

## 6. 受管外部协作

### 6.1 场景定义

企业 A 部署 Cervi，企业 B 是供应商或合作伙伴但没有部署 Cervi。企业 A 希望沟通、文件和问题处理都沉淀在自己的 Cervi 中，因此邀请企业 B 的人员进入 A 托管的受限协作空间。

该场景命名为“受管外部协作”，不属于联邦通信。

### 6.2 外部组织与访客

企业 A 在本地维护合作方档案：

```text
PartnerOrganization
├── 名称
├── 合作状态
├── 备注
└── GuestUser
```

受管访客可以：

- 参加被邀请的单聊和群聊。
- 查看共享给自己的工单。
- 发送对外消息和工单回复。
- 按权限上传和下载文件。
- 查看同一共享空间中必要的参与者信息。

受管访客默认不能：

- 浏览企业 A 的完整通讯录。
- 搜索或加入未被邀请的内部群。
- 查看内部工单评论、内部字段和内部知识。
- 管理渠道、联系人、用户、角色和企业设置。
- 查看其他供应商的数据。

### 6.3 邀请流程

默认使用定向、一次性、短期有效的邀请：

1. 企业成员选择邀请外部协作者。
2. 选择或创建合作方企业。
3. 填写手机号或邮箱。
4. 选择允许访问的群聊、项目或工单。
5. 生成带过期时间的一次性链接。
6. 受邀人通过验证码或授权方式验证身份。
7. 受邀人成为当前企业托管的访客。
8. 访客进入独立的外部协作门户。

可转发的多人邀请链接属于增强功能，必须支持有效期、最大使用次数、加入审批、撤销和审计。

### 6.4 外部协作入口

入口优先级：

1. 响应式 Web/PWA 与邀请链接。
2. 短信、邮件或企业微信通知中的安全链接。
3. 微信小程序客户端壳。
4. 企业部署 Cervi 后升级为联邦通信。

小程序不是新的数据源。它只作为客户端访问企业 A 的 Cervi，消息和文件仍写入企业 A 的私有部署。

私有部署如需允许外部访问，应提供独立的外部访问网关，只暴露访客认证、共享会话、对外工单和文件接口，不直接暴露完整内部管理 API。

### 6.5 从访客升级为联邦身份

企业 B 后来部署 Cervi 并与企业 A 建立可信连接时，不重写历史消息发送人。

通过经过双方确认的身份关联，将原受管访客与新的联邦身份关联：

```text
actor_identity_links
├── guest_identity_id
├── federated_identity_id
├── verified_by
└── verified_at
```

升级后：

- 历史消息继续保留原受管访客身份。
- 界面可以将两个身份聚合显示为同一个人。
- 新消息使用企业 B 的联邦身份。
- 原受管外部群可以升级为联邦群。

## 7. 联邦通信预留

### 7.1 原则

- 不假设所有参与者都存在于同一个数据库。
- 不使用数据库自增编号作为跨部署标识。
- 会话、消息和同步事件使用全局唯一编号。
- 服务器间请求需要认证、签名、幂等和重放保护。
- 远端成员在本地保存受限身份投影，不创建成本地企业成员。
- 文件来源和下载授权不能假设文件一定存储在当前服务器。

### 7.2 建议的托管方式

联邦会话拥有明确的发起或托管服务器。参与企业服务器保存本地镜像或查询投影，按全局消息编号幂等同步。

未来协议需要覆盖：

- 企业连接申请、审批、停用和拉黑。
- 服务器身份、公钥和密钥轮换。
- 成员邀请、加入、退出和移除。
- 消息投递、确认、失败重试和补拉。
- 编辑、撤回和删除的权限边界。
- 文件元数据和跨服务器下载授权。
- 断开连接后的历史保留策略。
- 远端不可用时的本地展示和恢复同步。

首期不实现分布式事件图或多主状态冲突解决。只有联邦需求进入开发阶段时，再确定具体一致性模型和协议版本。

## 8. 数据隔离与安全边界

所有查询和写入必须显式带当前 `organization_id`，不能只凭资源 UUID 查询。

需要长期保持以下边界：

- 内部单聊和群聊按有效参与者关系授权。
- 客户会话当前按已登录和企业边界读取，ServiceSession 落地后按排队、负责人和协作者授权。
- 能查看客户收件箱不等于成为会话参与者，不把所有企业成员写入客户会话参与者表。
- 联系人只能通过其来源渠道访问对应客户会话。
- 受管访客只能访问邀请范围内的资源。
- 联邦用户只能访问双方共享并仍然有效的资源。
- 工单内部评论和内部字段不进入外部消息时间线。
- 群聊存在外部参与者时必须在界面持续标识。
- 邀请、成员变更、权限变更和跨企业操作写入审计记录。
- 退出或停用成员不物理删除历史发送者关系。

## 9. 路线阶段

### 阶段 0：客户会话数据底座

目标：以网站渠道的客户入站消息作为第一个落地场景，建立可继续支持内部聊天和联邦通信的统一会话、参与者和消息模型。

包括：

- 会话、参与者、文本消息存储。
- 网站渠道身份查找或创建联系人。
- 同一渠道身份复用长期客户会话。
- 客户入站文本消息 Action 最小闭环。
- 渠道消息重试幂等。
- 引用与话题关系字段预留。

不包括公开接入 API、客服回复、服务批次、实时推送和界面。

### 阶段 1：外部客户单聊

目标：让网站渠道联系人与客服完成真实消息闭环。

包括：

- 渠道身份进入客户会话。
- 客户消息接收和客服回复。
- 客户会话列表和历史消息。
- ServiceSession 排队、接待、转接、挂起和结束。
- 首次响应和会话时长等基础指标。
- 文件消息。

### 阶段 2：企业内部单聊与群聊

目标：让 Cervi 成为企业内部日常沟通入口。

包括：

- 发起单聊。
- 创建、编辑和归档群聊。
- 添加和移除群成员。
- 引用回复、@ 提醒和已读状态。
- 实时消息、离线补拉和通知。
- 消息搜索的基础索引。

公开群和话题模式仍不进入默认范围。

### 阶段 3：受管外部协作

目标：让未部署 Cervi 的供应商和合作伙伴安全参与。

包括：

- 合作方企业档案。
- 受管访客账户。
- 定向邀请和审批。
- 外部协作门户。
- 受管外部单聊和群聊。
- 对外文件访问和审计。

### 阶段 4：工单

目标：把消息中的问题转成可分派、可跟踪、可度量的工作。

包括：

- 工单状态、优先级、负责人和团队。
- 会话或指定消息创建工单。
- 一个工单关联多个会话。
- 内部评论和对外回复。
- SLA、流转记录和自动化。
- 客户与受管访客工单门户。

### 阶段 5：Cervi 企业联邦

目标：让多个独立部署保持各自身份和数据边界进行协作。

包括：

- 企业连接与信任管理。
- 联邦身份投影。
- 跨企业单聊和群聊。
- 消息和成员事件同步。
- 断线重试与历史补拉。
- 受管访客升级为联邦身份。

### 阶段 6：结构化大型协作

根据实际客户使用验证后选择实现：

- 企业内可发现的公开群。
- 公告或只读群。
- 群聊话题模式。
- 独立子讨论空间。
- 显式共享的跨企业协作工单。

## 10. 第一个 PR 方案

### 10.1 PR 定位

建议标题：

```text
feat: 建立客户会话与入站文本消息数据底座
```

目标：

在不暴露新前端页面和公开业务接口的前提下，建立通用会话存储结构，并通过 Action 层完成网站渠道身份、联系人、客户会话和入站文本消息的事务闭环。

该 PR 验证以下不可轻易返工的基础约束：

- 所有数据按企业隔离。
- 同一渠道外部身份只对应一条渠道身份记录。
- 同一渠道身份只对应一个长期客户会话。
- 同一条渠道消息重试不会重复创建联系人、会话或消息。
- 自动进入系统的联系人不需要伪造创建用户。
- 客户消息发送者可以通过参与者关系稳定追溯。
- 客户渠道关系通过类型扩展表表达，不解析自然键，也不污染通用会话字段。
- 客户收件箱访问与会话参与者关系保持独立。

### 10.2 PR 范围

新增领域值：

```text
ConversationType
├── direct
├── group
└── customer

ConversationStatus
├── active
└── archived

ParticipantIdentityType
├── user
├── guest
├── federated_user
├── contact
└── bot

ConversationParticipantRole
├── owner
└── member

MessageType
├── text
└── system
```

首个 PR 只创建 `customer` 会话，并只允许 `contact` 参与者发送 `text` 消息。其他枚举用于稳定契约，不在 Action 中开放创建路径。

调整现有联系人结构：

- 将 `contacts.created_by_user_id` 改为可空，手工创建联系人时仍记录当前用户，渠道自动创建联系人时保持为空。
- 将 `servermodels.Contact.CreatedByUserID` 改为 `*string`。
- 入站自动创建联系人时，`source_channel_id` 使用当前网站渠道，`stage` 使用 `visitor`，显示名称允许为空，不伪造用户、邮箱或其他联系方式。
- 已有渠道身份收到新的非空渠道显示名称时更新 `contact_channel_identities.display_name`；只有 `contacts.display_name` 为空时才补充联系人名称，不覆盖客服人工维护的名称。
- 现有手工创建和读取联系人行为保持目标模型下的一致实现，不增加历史数据兼容逻辑。

新增四张表，每个迁移文件只创建一张表：

#### conversations

```text
id                  uuid primary key default uuidv7()
organization_id     uuid not null
type                text not null
status              text not null default 'active'
title               text
created_by_identity_type text
created_by_identity_id uuid
last_message_at     timestamptz
created_at          timestamptz not null default now()
updated_at          timestamptz not null default now()
```

约束与索引：

- `(organization_id, type, status, last_message_at DESC NULLS LAST, id DESC)` 用于分类会话列表，查询排序与空值规则必须一致。
- 创建者身份两个字段必须同时为空或同时有值，由 Action 维护；空值表示系统或集成创建。
- 渠道联系人首次发消息创建客户会话时，创建者身份为该联系人。
- 客户会话 `title` 保持为空，展示名称从联系人及渠道身份读取，避免复制过期名称。
- `direct` 和 `group` 在本 PR 中不创建实例。

#### customer_conversations

```text
conversation_id              uuid primary key
organization_id              uuid not null
contact_channel_identity_id  uuid not null
created_at                   timestamptz not null default now()
```

约束与索引：

- 唯一索引命名为 `customer_conversations_organization_channel_identity_unique`，覆盖 `(organization_id, contact_channel_identity_id)`，保证同一渠道身份只对应一个长期客户会话。
- `(organization_id, conversation_id)` 用于带企业边界读取客户会话扩展。
- 联系人和渠道通过 `contact_channel_identities` 的真实字段关联，不从字符串键解析。
- `contact_channel_identities.channel_id`、`contact_id` 及客户会话扩展关系创建后不可被普通更新操作改变。
- 同一联系人拥有多个渠道身份时保留多个客户会话；跨渠道历史只在联系人或工单层聚合。
- 该表只表达客户会话类型关系，不保存 ServiceSession 状态和客服负责人。

#### conversation_participants

```text
id                  uuid primary key default uuidv7()
organization_id     uuid not null
conversation_id     uuid not null
identity_type       text not null
identity_id         uuid not null
role                text not null default 'member'
joined_at           timestamptz not null default now()
left_at             timestamptz
last_read_message_id uuid
last_read_at        timestamptz
created_at          timestamptz not null default now()
updated_at          timestamptz not null default now()
```

约束与索引：

- `(organization_id, conversation_id, identity_type, identity_id)` 唯一。
- 唯一索引命名为 `conversation_participants_org_conversation_identity_unique`，供并发写入时精确识别预期冲突；名称保持在 PostgreSQL 的 63 字节标识符限制内。
- `(organization_id, identity_type, identity_id, left_at, conversation_id)` 用于读取当前身份的会话。
- 关系退出时设置 `left_at`，不删除记录。
- `last_read_message_id` 必须由 Action 验证属于同一会话。
- `identity_type = contact` 时，`identity_id` 必须是 `contacts.id`，不能使用渠道身份编号。
- 一个客户会话最多存在一个未退出的联系人参与者，由 Action 维护。
- 身份类型和身份编号插入后不可修改；联系人以后关联访客或联邦身份时使用独立身份关联表，不重写历史参与者和消息发送者。
- 本 PR 不写客服个人已读状态，也不把能够查看收件箱的全体成员加入参与者表。

#### messages

```text
id                    uuid primary key default uuidv7()
organization_id       uuid not null
conversation_id       uuid not null
sender_participant_id uuid
type                  text not null
body                  text not null default ''
reply_to_message_id   uuid
thread_root_message_id uuid
idempotency_key       text
created_at            timestamptz not null default now()
edited_at             timestamptz
deleted_at            timestamptz
```

约束与索引：

- `(organization_id, conversation_id, created_at DESC, id DESC)` 用于消息分页。
- 部分唯一索引命名为 `messages_organization_idempotency_unique`，覆盖 `(organization_id, idempotency_key)` 且只包含非空幂等键。
- 网站入站消息的幂等键固定为 `chmsg:<channel_id>:<source_message_id>`。
- 后续本地客户端消息使用 `client:<participant_id>:<nonce>`，不得与渠道消息共用无前缀编号。
- 幂等键前缀及其作用域共同组成企业内消息命名空间，使同一来源消息即使被错误地提交到另一个会话也不能重复入库。
- `created_at` 使用当前服务器接收消息的时间；外部渠道原始发送时间以后作为来源元数据增加，不参与本 PR 的排序正确性。
- 文本消息必须具有发送参与者；系统消息允许发送参与者为空，由 Action 根据消息类型维护。
- 引用目标和话题根消息由 Action 显式校验属于同一企业和同一会话。
- 本 PR 不允许编辑、删除和话题回复，但保留字段以稳定消息结构。

迁移遵循项目约定：不创建外键和 `CHECK` 约束，使用简洁中文 `COMMENT ON`，所有关联与枚举合法性由 Action 在事务中维护。

新增 Bun 模型：

```text
internal/storage/server/models/conversation.go
internal/storage/server/models/customer_conversation.go
internal/storage/server/models/conversation_participant.go
internal/storage/server/models/message.go
```

联系人包增加可在既有事务中调用的渠道身份能力，集中维护联系人规则：

```text
internal/actions/contact/ensure_channel_identity.go
```

新增 Action 包：

```text
internal/actions/conversation/
├── receive_customer_text_message.go
├── receive_customer_text_message_test.go
├── list_customer_conversations.go
├── list_customer_messages.go
├── customer_queries_test.go
├── helpers.go
├── types.go
├── errors.go
└── validation.go
```

Action 行为：

#### ReceiveCustomerTextMessage

- 接收经过渠道适配器验证和归一化的渠道编号、外部身份编号、显示名称、渠道消息编号和文本正文。
- 该 Action 是可信业务入口，不直接接受浏览器伪造的联系人编号；公开 API 必须先完成访客会话验证。
- 校验渠道存在、已启用且类型为网站渠道，并从渠道记录取得企业编号。
- 渠道编号必须是有效 UUID。
- `external_id` 和 `source_message_id` 去除首尾空白后必填，首期各不超过 255 个 Unicode 字符。
- 渠道显示名称可空，非空时不超过 200 个 Unicode 字符，并复用联系人领域的归一化规则。
- 去除消息正文首尾空白后校验非空，首期长度上限为 10,000 个 Unicode 字符。
- `messages.body` 存储校验通过后的规范化正文；幂等比较直接比较已入库正文和本次规范化正文。
- 通过联系人包能力按 `(channel_id, external_id)` 查找渠道身份；不存在时在当前事务中创建无创建用户的联系人和渠道身份。
- 已存在渠道身份时复用联系人，仅以非空渠道显示名称更新渠道身份，同时更新最后活跃时间和联系人更新时间；联系人名称只在原值为空时补充。
- 联系人处于回收站时，新的真实渠道消息将 `deleted_at` 置空；保留原客户阶段和人工资料。
- 按渠道身份创建或读取唯一 `customer_conversations` 扩展及其通用会话。
- 客户会话已归档时，新入站消息将其恢复为 `active`；归档只表示暂时从活动列表隐藏，不表示客服结案。
- 确保联系人以 `contact` 身份、`contacts.id` 编号成为唯一有效联系人参与者。
- 按固定格式生成 `idempotency_key`，相同消息重试时返回已有结果。
- 已有幂等消息的渠道身份、发送身份和规范化正文必须与本次输入一致；渠道身份通过已有消息所属的客户会话扩展比较，发送身份通过已有 `sender_participant_id` 关联的身份类型与身份编号比较，不能依赖本次请求临时创建的参与者。
- 同一幂等键携带不同渠道身份、内容或发送身份时返回冲突错误，不静默覆盖或伪装成成功重试。
- 在同一事务中写入消息，并以新消息服务器接收时间只向后更新会话 `last_message_at` 和 `updated_at`。
- 返回联系人、渠道身份、会话和消息编号，供后续接入层触发实时通知。
- 不在本 PR 中负责访客认证、HTTP 响应或实时推送。

#### ListCustomerConversations

- 校验当前身份仍是当前企业有效用户。
- 通过 `customer_conversations` 只读取当前企业、`type = customer`、`status = active`、`last_message_at IS NOT NULL` 且联系人未删除的客户会话。
- 使用 `(last_message_at, id)` 倒序游标分页，与会话列表索引一致。
- 返回能够证明查询模型的联系人、来源渠道和最后消息记录，不在本 PR 固化最终 appservice 或前端 DTO。
- 本 PR 不计算个人未读数和客服分配状态。
- 所有统计显式限定 `organization_id`。

#### ListCustomerMessages

- 校验当前身份仍是当前企业有效用户。
- 校验目标是当前企业的客户会话。
- 使用游标按 `(created_at, id)` 倒序分页。
- 返回发送者身份、正文、引用关系和时间。
- 当前阶段所有已登录企业成员均可读取客户收件箱；ServiceSession 引入后再按接待关系收紧。
- 不把所有历史消息嵌套进会话列表结果。

### 10.3 错误语义

Action 使用语言无关错误，至少包括：

```text
ErrConversationNotFound
ErrCustomerChannelUnavailable
ErrCustomerIdentityInvalid
ErrCustomerIdentityTooLong
ErrSourceMessageIDRequired
ErrSourceMessageIDTooLong
ErrMessageBodyRequired
ErrMessageBodyTooLong
ErrMessageIdempotencyConflict
```

登录后查询时，组织外资源、无权访问资源和真实不存在资源统一返回 Not Found 语义，避免泄露其他企业中的资源存在性。入站 Action 对不存在、已停用和非网站渠道统一返回渠道不可用语义。

本 PR 不增加 appservice 本地化映射；该映射随公开服务契约 PR 一起实现。

### 10.4 事务与并发

- 联系人、渠道身份、客户会话、参与者和首条消息必须在一个事务中完成。
- 写入 Action 最多执行三次完整事务尝试。
- 每次事务先执行无副作用预检：渠道校验、读取已有渠道身份（包括已删除联系人对应的身份），并按企业和幂等键读取已有消息及其客户会话扩展、渠道身份和发送参与者。
- 预检命中已有消息且渠道身份、发送身份和规范化正文一致时，直接返回已有结果，不恢复联系人、不解档会话、不更新渠道显示名称、`last_seen_at`、联系人时间或 `last_message_at`。
- 预检命中已有消息但当前渠道身份不存在，或渠道身份、发送身份、规范化正文任一项不一致时，返回 `ErrMessageIdempotencyConflict`，同样不得产生任何写入副作用。
- 只有确认幂等消息不存在后，才依次查找或创建联系人和渠道身份、恢复联系人、查找或创建通用会话与客户扩展、恢复归档会话、确保联系人参与者、插入消息并更新最后消息时间。
- 未命中幂等消息时，新渠道身份按正常创建流程继续；并发请求通过唯一冲突整体回滚后，下一次事务会读取先提交的消息并按幂等规则返回。
- 并发收到同一新访客首条消息时，依靠 `contact_channel_identities_channel_external_unique` 和客户扩展唯一索引收敛。
- 外层只将以下命名约束视为预期竞态：`contact_channel_identities_channel_external_unique`、`customer_conversations_organization_channel_identity_unique`、`conversation_participants_org_conversation_identity_unique`、`messages_organization_idempotency_unique`。
- 命中上述预期唯一冲突后，当前事务必须整体回滚，再从头执行一次事务并读取已有记录；禁止在失败事务或 savepoint 中改绑后继续提交，不能遗留孤立联系人、通用会话或参与者。
- 联系人包的渠道身份能力不得捕获唯一冲突后在当前事务中改绑，必须把 PostgreSQL `23505` 及约束名原样交给外层重试判断。
- 只重试明确识别的预期唯一约束冲突；其他唯一冲突和第三次仍冲突直接返回错误，不降级为重复写入。
- 消息写入和会话最后消息时间更新必须在一个事务中完成。
- 渠道重试使用 `idempotency_key` 幂等；相同渠道消息编号重复提交时返回已有消息。
- 相同幂等键的渠道身份、正文或发送者不一致时返回幂等冲突，既不覆盖已有消息，也不创建新消息。
- `last_message_at` 只向后推进，延迟到达的消息不能覆盖更新的最后活跃时间。
- 唯一索引与有限事务重试是正确性机制；首个 PR 不增加咨询锁，确有热点竞争指标后再优化。
- 页面卸载和网络重试不通过取消数据库操作实现。

### 10.5 分页与未读

客户会话列表和消息列表不使用不断增大的 offset 作为主要分页方式。

游标定义：

```text
客户会话：(last_message_at, conversation_id)
消息：    (created_at, message_id)
```

本 PR 不实现个人已读状态。客服接待关系确定后，在 ServiceSession PR 中定义负责人、协作者和未读状态，避免为了未分配队列给所有企业成员创建会话参与者。

### 10.6 本 PR 明确不做

- 不修改前端消息页面。
- 不修改或手工生成 `frontend/bindings`。
- 不增加 appservice、API Proxy 和 Gin 路由。
- 不开放网站访客会话或消息 HTTP 接口。
- 不实现访客令牌、验证码或浏览器会话认证。
- 不实现 WebSocket、SSE、轮询或推送通知。
- 不实现客服回复和消息外发。
- 不实现客服分配、个人已读和未读计数。
- 不实现群聊创建和成员管理。
- 不实现内部单聊。
- 不实现 ServiceSession。
- 不实现文件消息、表情、编辑、撤回和全文搜索。
- 不实现合作方企业、访客邀请和外部协作门户。
- 不实现工单。
- 不实现服务器连接、签名或消息联邦同步。
- 不增加联邦 `global_id`、`origin_server_id` 等未确定语义的空字段。
- 不把查询结果接到现有 `LoadInbox`，也不扩展现有嵌套全部消息的收件箱 DTO。
- 不把全部企业成员写入客户会话参与者表。
- 不增加权限资源；客户会话读取当前只校验已登录和企业边界。

### 10.7 验收标准

PR 中增加数据库级 Action 测试锁定以下行为。测试和迁移验证在具体实施任务获得运行授权后执行；路线图更新与方案审查本身不运行测试或重建共享数据库。

- 网站渠道收到新外部身份的首条消息时，在同一事务中创建联系人、渠道身份、客户会话、参与者和消息。
- 自动创建联系人的 `created_by_user_id` 为空，手工创建联系人仍记录当前用户。
- 同一 `(channel_id, external_id)` 始终复用同一渠道身份和联系人。
- 同一渠道身份通过客户扩展表始终复用同一客户会话，不因客服服务结束创建新的消息容器。
- 同一联系人不同渠道身份保持不同客户会话。
- 相同渠道消息编号重试只产生一条消息。
- 相同渠道消息编号携带不同正文时明确失败，不覆盖第一次写入。
- 同一渠道消息编号被提交给另一个外部身份时明确失败，不能在另一个客户会话中重复入库。
- 并发首条消息不会遗留孤立联系人或重复客户会话。
- 已停用渠道、非网站渠道和无效外部身份不能创建消息。
- 渠道消息到达已移入回收站的联系人时恢复该联系人，但不覆盖其阶段和人工维护资料。
- 已归档客户会话收到新消息时恢复为活动状态并重新出现在列表中。
- 对已删除联系人或已归档会话重放同一来源消息时，只返回原消息，联系人保持删除、会话保持归档，所有活跃时间保持不变。
- 客户会话只有联系人发送者参与者，不因全员可查看而批量产生用户参与者。
- 会话列表和消息列表均按文档定义的双字段游标稳定分页。
- 消息列表分页稳定，不因同一时间写入多条消息出现重复或遗漏。
- 所有数据库查询显式限定当前企业。
- 所有具名 Go 函数和方法使用简洁中文注释。
- 每个迁移只创建一张表，无外键和 `CHECK` 约束，并包含中文字段注释。
- 不提交手工修改的 Wails 绑定。

### 10.8 后续 PR 衔接

第一个 PR 合并后，建议依次进行：

1. 建立网站访客会话令牌和公开入站消息 API；`external_id` 由服务端签发并绑定在访客令牌中，浏览器不能在每次加载或发消息时自造身份。
2. 建立 ServiceSession、客服接待和回复 Action。
3. 暴露客服收件箱 appservice 契约、Gin API 和 API Proxy，并生成 Wails 绑定。
4. 把消息页改成分页客户会话列表和按需加载的消息详情。
5. 接入实时消息通知、断线补拉和客服未读状态。
6. 增加文件消息及客户端直传。
7. 在客户会话闭环稳定后，复用统一模型增加内部单聊和群聊。

## 11. 暂缓决策

以下问题在对应阶段开始前再形成 ADR 或方案，不在首个 PR 中提前定死：

- 联邦会话是单一托管服务器权威，还是多个服务器共同维护状态。
- 消息编辑和删除在跨企业之间的传播与保留规则。
- 跨企业文件采用源站临时授权还是接收方复制。
- 端到端加密与企业合规审计之间的取舍。
- 小程序由 Cervi 官方统一提供，还是每个私有化客户独立配置。
- 受管访客计费和席位策略。
- 工单公开字段是否允许合作方联合编辑。
- 公开群、话题群和独立讨论空间的真实优先级。

## 12. 方案审查与取舍

Grok 对整体路线给出 `Go`，认为“统一通用会话 + 客户会话类型扩展”“参与者与客户收件箱访问分离”“先客户单聊、再内部聊天和外部协作”的方向成立。

Grok 对修订前的第一个 PR 给出 `Conditional Go`，指出三个数据正确性问题：

- 幂等重放检查必须发生在恢复联系人、解档会话和更新活跃时间之前，命中重放时不能产生写入副作用。
- 入库正文与幂等比较必须使用同一份规范化正文。
- 参与者唯一冲突也要纳入命名约束和完整事务重试；联系人能力不能在已触发 `23505` 的事务内吞掉冲突继续执行。

上述问题均已纳入本方案。独立复核后又补充和明确：

- 消息幂等键在企业内唯一，而不是只在单个会话内唯一；因此同一渠道消息被错误地提交到另一个外部身份或会话时也会冲突。
- 重放一致性同时比较客户会话渠道身份、历史发送参与者和规范化正文。
- 命名索引不得超过 PostgreSQL 的 63 字节标识符限制，避免运行时约束名被截断后无法精确分类重试。
- 客户渠道关系继续使用 `customer_conversations` 类型扩展，不改回字符串会话键，也不把渠道专属可空字段塞进通用会话表。
- 首个 PR 使用唯一索引和最多三次完整事务重试，不增加咨询锁；只有真实热点指标出现后才考虑更复杂的并发控制。
- 服务批次、公开访客 API、客服回复、最终前端 DTO、细粒度权限和联邦字段继续留在后续 PR，不扩大首个 PR。

基于以上修订，当前独立结论为：整体路线 `Go`，第一个 PR 方案 `Go`。实施时如偏离第 10.4 节的无副作用幂等预检或第 10.7 节的并发验收标准，应重新降级为 `Conditional Go` 并先修正设计或实现。

## 13. 参考产品与资料

- Rocket.Chat Rooms：<https://docs.rocket.chat/docs/rooms>
- Rocket.Chat Discussions：<https://docs.rocket.chat/docs/discussions>
- Rocket.Chat Threads：<https://docs.rocket.chat/docs/threads>
- Rocket.Chat Omnichannel Conversations：<https://docs.rocket.chat/docs/omnichannel-conversations>
- 飞书消息群组类型：<https://www.feishu.cn/hc/zh-CN/articles/552185547397-%E6%B6%88%E6%81%AF%E7%BE%A4%E7%BB%84%E7%B1%BB%E5%9E%8B%E8%AF%B4%E6%98%8E>
- Slack Guest Roles：<https://slack.com/help/articles/202518103-Understand-guest-roles-in-Slack>
- Slack Connect：<https://slack.com/help/articles/115004151203-Slack-Connect-guide--Work-with-external-organizations>
- Microsoft Teams Guest Access：<https://learn.microsoft.com/en-us/microsoftteams/guest-access>
- Jira Service Management 外部客户与组织：<https://support.atlassian.com/jira-service-management-cloud/docs/what-are-customers-and-organizations-in-your-service-project/>
- Matrix Server-Server API：<https://spec.matrix.org/latest/server-server-api/>
- Zendesk 消息工单路由：<https://support.zendesk.com/hc/en-us/articles/4408829019162-Routing-messaging-tickets-and-notifying-agents>
- Intercom Tickets API：<https://developers.intercom.com/docs/references/rest-api/api.intercom.io/tickets>
