# 客户聊天数据底座 PR 设计

## 1. PR 定位

本 PR 是网站访客真实消息闭环的第一个 PR，只建立客户聊天所需的数据库结构、领域值和 Bun 模型，并暂时隐藏管理端手动添加外部联系人的入口。它不接入网站访客公开接口，不创建真实聊天数据，也不改造 Messenger 的演示行为。

后续 [chat-visitor-message-pr-design.md](chat-visitor-message-pr-design.md) 基于本 PR 实现网站访客身份、首条文本事务、真实会话列表和消息历史。两份设计共同替代原先把数据底座与访客闭环放在一个 PR 中的方案。

本 PR 基于 `main` 提交 `1bfaed739628cfd364368c010d2b6a599e1fcebb` 开发。后续实现时如果基线已经推进，只按实际主线解决机械差异，不改变本文领域边界。

PR 标题使用：

```text
feat: 建立客户聊天数据底座
```

## 2. 设计结论

### 2.1 Conversation 是客户可见聊天线程

客户聊天使用以下关系。箭头表示表中的引用或来源关系，不表示创建顺序：

```text
ContactChannelIdentity ──> Contact ──> ChatSubject

Conversation ──> CustomerConversation ──> ContactChannelIdentity
Conversation ──> ConversationParticipant ──> ChatSubject
Conversation ──> ServiceSession *
Conversation ──> Message *
                   ├── sender_participant_id ──> ConversationParticipant
                   └── service_session_id ──> ServiceSession
```

`Conversation` 是客户和企业成员实际打开、阅读和继续的稳定聊天线程，也是消息历史、参与者、同步和 Agent 上下文的边界。

`ServiceSession` 是一条客户会话上的一次等待、接待、挂起和结束周期，只保存客服工作状态、路由、负责人和指标。它不是客户可见聊天列表的主键，也不切断同一 Conversation 的消息历史。

### 2.2 同一渠道身份可以拥有多条 Conversation

同一个 `ContactChannelIdentity` 可以对应多条 `CustomerConversation`。这为网站 Messenger 以后允许客户主动创建多个独立聊天线程保留稳定模型和公开编号。

不同渠道根据自身能力决定实际基数：

- 网站 Messenger 能明确创建和选择 Cervi Conversation，长期允许一个渠道身份拥有多条 Conversation。
- Telegram Bot 私聊、微信公众号等没有独立线程选择能力的渠道，可以由对应 Adapter 固定复用该渠道身份的一条 Conversation。
- 群、频道和讨论组不进入一对一 `customer_conversations`，继续使用 `group` Conversation。

本 PR 只建立允许一对多的结构，不开放任何创建入口。

### 2.3 ServiceSession 保持两级唯一性

长期不变量是同一 Conversation 同时最多一个未结束 `ServiceSession`。

首版网站产品还采用更严格的入站控制：同一个 `ContactChannelIdentity` 同时最多一个未结束 `ServiceSession`，因此首版最多只有一条未结束客户线程。该限制用于实现类似 Intercom “阻止多个入站会话”开启时的体验，后续开放并行线程时只删除身份级部分唯一约束，不更换 Conversation 主键和公开契约。

为让数据库直接保证首版身份级不变量，`service_sessions` 保存 `contact_channel_identity_id`。它是用于排队查询、身份级并发锁定和部分唯一约束的明确业务维度，不是从 Conversation 任意复制的展示字段。Action 写入时必须校验：

```text
ServiceSession.contact_channel_identity_id
  = CustomerConversation.contact_channel_identity_id
ServiceSession.conversation_id
  = CustomerConversation.conversation_id
```

`contact_channel_identity_id` 创建后不可修改。它只从已经锁定的 ContactChannelIdentity 及其 CustomerConversation 关系写入，禁止事后从 Conversation 回填或因展示需要改写。该列是 ServiceSession 的身份级业务边界，不是 `customer_conversations` 的可变镜像缓存。

## 3. PR 范围

本 PR 完成以下内容：

- 增加聊天领域值。
- 通过新增迁移把 `contacts.created_by_user_id` 改为可空，不修改联系人原建表迁移。
- 同步调整现有手工联系人 Action，使其继续写入当前用户编号的指针值。
- 创建六张聊天核心表及索引。
- 增加六张表对应的服务端 Bun 模型。
- 暂时注释管理端“添加外部联系人”菜单入口。
- 保留联系人列表、详情、编辑、删除和恢复能力。

本 PR 不完成以下内容：

- 网站访客 Token、Cookie 或公开 HTTP 接口。
- 网站访客自动创建联系人和渠道身份。
- `EnsureChannelIdentity` 或入站消息 Action。
- 任何 Conversation、ServiceSession 或 Message 的业务写入。
- 访客会话列表和消息历史 Query。
- Messenger 真实数据接入。
- 企业成员收件箱读取或回复。
- ServiceSession 领取、转接、挂起和关闭命令。
- 未读、实时、文件、外部平台投递和 Agent Run。
- 联系人合并、导入和跨渠道自然人识别。

## 4. 领域值

`internal/domain` 按概念拆分文件增加聊天领域值：

```text
ChatSubjectKind
├── organization_identity
└── contact

ConversationType
├── direct
├── group
└── customer

ConversationStatus
├── active
└── archived

ConversationParticipantRole
├── owner
└── member

ServiceSessionStatus
├── waiting
├── active
├── pending
└── closed

MessageType
├── text
└── system
```

本 PR 只固定可持久化契约，不开放对应业务 Action。这些取值只进入 `internal/domain`，本 PR 不向聊天表写入任何业务行。

现有 `internal/domain/message.go` 中用于 Messenger 演示展示的 `MessageAuthor` 与持久化 `MessageType` 不是同一层契约，本 PR 不复用、扩展或删除它。

## 5. 联系人向前迁移

### 5.1 创建人可空

保留现有 `20260818032659_create_contacts_table.sql` 原样，新增独立向前迁移：

```text
YYYYMMDDHHMMSS_alter_contacts_created_by_user_id_nullable.sql
```

Up 使用：

```sql
ALTER TABLE contacts
    ALTER COLUMN created_by_user_id DROP NOT NULL;

COMMENT ON COLUMN contacts.created_by_user_id
    IS '创建用户编号，渠道自动创建时为空';
```

Down 恢复原约束和原注释：

```sql
ALTER TABLE contacts
    ALTER COLUMN created_by_user_id SET NOT NULL;

COMMENT ON COLUMN contacts.created_by_user_id
    IS '创建人编号';
```

该迁移只做目标结构变更，不写历史数据回填或双版本读写逻辑。PR1 本身不会创建 `created_by_user_id = NULL` 的联系人，因此紧接 PR1 回滚时 Down 可以恢复非空约束；PR2 已产生渠道自动联系人后，如需回滚到 PR1 之前，必须先按环境管理要求清理后续业务数据或重建数据库，不能在 Down 中伪造创建用户。

`internal/storage/server/models.Contact.CreatedByUserID` 改为 `*string`。同步把 `CreateContactAction` 中当前用户编号转换为指针后写入，保证现有手工创建逻辑继续编译并记录创建用户。联系人 DTO、表单保存结果和列表查询不暴露该字段，不增加新的可空展示契约，也不修改 list/get 的列集。

`contacts.source_channel_id` 继续非空。后续渠道自动联系人写当前网站渠道；联系人导入、合并或无来源联系人不进入这两个 PR。

### 5.2 暂时隐藏手动添加入口

`frontend/src/features/contacts/contacts-page.tsx` 中只把跳转到 `/contacts/external?new=1`、文案为 `t("add.external")` 的那个 `DropdownMenuItem` 整段注释，并保留简洁 TODO，说明需要等手工联系人身份和可发送渠道关系明确后再恢复。

本 PR 只隐藏产品入口：

- 不删除 `ContactForm`。
- 不删除 `createContact` Action 和 appservice 契约。
- 不修改 `creating`、`setParameters({ new })`、新增 Dialog 或 `ContactForm` 分支。
- `/contacts/employees?new=1` 和团队页 `?new=1` 继续打开 `MemberForm`。
- `/contacts/external?new=1` 仍可直接打开 `ContactForm`；本 PR 不拦截、不宣传，也不增加兼容逻辑。
- 不删除 `add.external` 国际化词条。
- 不修改联系人列表、详情、编辑、回收站和恢复。
- 不修改成员、Agent 和团队的新增入口。
- 不把 `?new=1` 作为受支持的页面入口继续宣传，也不为了直接手写 URL 增加额外兼容逻辑。

这样避免当前用户创建只有 `source_channel_id`、却没有真实 `ContactChannelIdentity` 的外部联系人后误以为可以直接发起客户聊天。

## 6. 数据表

六张新表统一把主键放在第一列，紧接 `created_at` 和 `updated_at`，再排列企业边界和其他业务字段。`customer_conversations` 没有独立 `id`，因此使用主键 `conversation_id` 作为第一列；`created_at`、`updated_at` 仍紧随其后。即使当前业务不读取或主动更新某个 `updated_at`，新表也保留该标准审计字段。本 PR 只统一本轮新增迁移，不修改任何历史迁移；历史表的字段顺序由后续独立 PR 处理。

### 6.1 `chat_subjects`

`chat_subjects` 为联系人和企业身份分配稳定聊天主体编号：

```sql
CREATE TABLE chat_subjects (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    kind             text NOT NULL,
    source_id        uuid NOT NULL
);

COMMENT ON TABLE chat_subjects IS '企业聊天主体';
COMMENT ON COLUMN chat_subjects.id IS '聊天主体编号';
COMMENT ON COLUMN chat_subjects.created_at IS '创建时间';
COMMENT ON COLUMN chat_subjects.updated_at IS '更新时间';
COMMENT ON COLUMN chat_subjects.organization_id IS '所属企业编号';
COMMENT ON COLUMN chat_subjects.kind IS '聊天主体类型：organization_identity、contact';
COMMENT ON COLUMN chat_subjects.source_id IS '主体来源记录编号';

CREATE UNIQUE INDEX chat_subjects_org_kind_source_unique
    ON chat_subjects (organization_id, kind, source_id);

COMMENT ON INDEX chat_subjects_org_kind_source_unique
    IS '企业内聊天主体来源唯一索引';
```

`kind + source_id` 创建后不可修改。业务 Action 必须校验来源记录属于相同企业。来源被停用、离开或进入回收站后保留主体，避免历史消息失去发送者关系。

### 6.2 `conversations`

`conversations` 保存各端实际打开的稳定聊天线程：

```sql
CREATE TABLE conversations (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    organization_id        uuid NOT NULL,
    type                   text NOT NULL,
    status                 text NOT NULL DEFAULT 'active',
    title                  text,
    created_by_subject_id  uuid,
    last_message_id        uuid,
    last_message_at        timestamptz
);

COMMENT ON TABLE conversations IS '聊天会话';
COMMENT ON COLUMN conversations.id IS '会话编号';
COMMENT ON COLUMN conversations.created_at IS '创建时间';
COMMENT ON COLUMN conversations.updated_at IS '更新时间';
COMMENT ON COLUMN conversations.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversations.type IS '会话类型：direct、group、customer';
COMMENT ON COLUMN conversations.status IS '会话生命周期状态：active、archived';
COMMENT ON COLUMN conversations.title IS '会话标题';
COMMENT ON COLUMN conversations.created_by_subject_id IS '创建聊天主体编号';
COMMENT ON COLUMN conversations.last_message_id IS '会话最后消息编号';
COMMENT ON COLUMN conversations.last_message_at IS '会话最后消息发生时间';

CREATE INDEX conversations_org_type_status_last_message_index
    ON conversations (
        organization_id,
        type,
        status,
        last_message_at DESC NULLS LAST,
        id DESC
    );

COMMENT ON INDEX conversations_org_type_status_last_message_index
    IS '企业会话按类型、状态和最后消息排序索引';
```

`type` 创建后不可修改。客户入站创建的 `created_by_subject_id` 允许为空；它不用于识别客户参与者或授权访问，后续内部会话再根据真实创建入口写入。

Conversation 最后消息摘要的粒度是整条 Conversation 时间线：

- `last_message_id` 和 `last_message_at` 两列同时为空，或同时指向同一条消息。
- `last_message_at` 必须等于该消息的 `originated_at`。
- 仅当新消息的 `(originated_at, id)` 严格大于当前摘要时，两列一起更新。
- 补拉更早消息、编辑消息和删除最后一条消息都不回退摘要；删除后的列表预览回退由消息删除功能另行实现。

表允许创建时暂时没有消息，客户 Conversation 也允许两列为空。首条消息是否在同一事务填充两列由后续网站消息 PR 保证，本 PR 不通过插入业务行验收该行为。

### 6.3 `customer_conversations`

`customer_conversations` 把一条 `customer` Conversation 绑定到一个联系人渠道身份：

```sql
CREATE TABLE customer_conversations (
    conversation_id              uuid PRIMARY KEY,
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    organization_id              uuid NOT NULL,
    contact_channel_identity_id  uuid NOT NULL
);

COMMENT ON TABLE customer_conversations IS '客户会话渠道关系';
COMMENT ON COLUMN customer_conversations.conversation_id IS '客户会话编号';
COMMENT ON COLUMN customer_conversations.created_at IS '创建时间';
COMMENT ON COLUMN customer_conversations.updated_at IS '更新时间';
COMMENT ON COLUMN customer_conversations.organization_id IS '所属企业编号';
COMMENT ON COLUMN customer_conversations.contact_channel_identity_id IS '联系人渠道身份编号';

CREATE INDEX customer_conversations_org_channel_identity_created_index
    ON customer_conversations (
        organization_id,
        contact_channel_identity_id,
        created_at DESC,
        conversation_id DESC
    );

COMMENT ON INDEX customer_conversations_org_channel_identity_created_index
    IS '企业内渠道身份的客户会话创建时间索引';

CREATE INDEX customer_conversations_org_conversation_index
    ON customer_conversations (organization_id, conversation_id);

COMMENT ON INDEX customer_conversations_org_conversation_index
    IS '企业客户会话查询索引';
```

这里不再创建 `(organization_id, contact_channel_identity_id)` 唯一索引。同一渠道身份可以拥有多条 Conversation；一条 Conversation 仍只能通过主键关联一条客户会话扩展。

该表不保存排队、负责人、批次状态、未读、投递位置、Worker 租约或限流字段。`updated_at` 仅用于统一新表审计格式，当前两个 PR 不依赖它排序，也没有需要主动更新该关系的业务操作。

### 6.4 `conversation_participants`

`conversation_participants` 保存 ChatSubject 在一条 Conversation 中的稳定成员关系：

```sql
CREATE TABLE conversation_participants (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    organization_id  uuid NOT NULL,
    conversation_id  uuid NOT NULL,
    subject_id       uuid NOT NULL,
    role             text NOT NULL DEFAULT 'member',
    joined_at        timestamptz NOT NULL DEFAULT now(),
    left_at          timestamptz
);

COMMENT ON TABLE conversation_participants IS '会话参与者关系';
COMMENT ON COLUMN conversation_participants.id IS '会话参与者关系编号';
COMMENT ON COLUMN conversation_participants.created_at IS '创建时间';
COMMENT ON COLUMN conversation_participants.updated_at IS '更新时间';
COMMENT ON COLUMN conversation_participants.organization_id IS '所属企业编号';
COMMENT ON COLUMN conversation_participants.conversation_id IS '会话编号';
COMMENT ON COLUMN conversation_participants.subject_id IS '聊天主体编号';
COMMENT ON COLUMN conversation_participants.role IS '会话参与角色：owner、member';
COMMENT ON COLUMN conversation_participants.joined_at IS '首次加入时间';
COMMENT ON COLUMN conversation_participants.left_at IS '离开时间';

CREATE UNIQUE INDEX conversation_participants_org_conversation_subject_unique
    ON conversation_participants (
        organization_id,
        conversation_id,
        subject_id
    );

COMMENT ON INDEX conversation_participants_org_conversation_subject_unique
    IS '企业会话内聊天主体唯一索引';

CREATE INDEX conversation_participants_org_subject_active_index
    ON conversation_participants (
        organization_id,
        subject_id,
        left_at,
        conversation_id
    );

COMMENT ON INDEX conversation_participants_org_subject_active_index
    IS '企业聊天主体参与会话状态索引';
```

参与者采用“一主体在一条 Conversation 中一行到底”的模型。退出后设置 `left_at`，重新加入时清空并复用原行，保留首次 `joined_at`。客户联系人参与者使用 `member`；`owner` 留给以后内部会话。本 PR 不写入任何参与者。

### 6.5 `service_sessions`

`service_sessions` 保存客户 Conversation 的客服处理周期：

```sql
CREATE TABLE service_sessions (
    id                           uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now(),
    organization_id              uuid NOT NULL,
    conversation_id              uuid NOT NULL,
    contact_channel_identity_id  uuid NOT NULL,
    sequence                     bigint NOT NULL,
    status                       text NOT NULL DEFAULT 'waiting',
    team_id                      uuid,
    assignee_identity_id         uuid,
    opening_message_id           uuid NOT NULL,
    last_message_id              uuid NOT NULL,
    last_message_at              timestamptz NOT NULL,
    assigned_at                  timestamptz,
    first_response_at            timestamptz,
    status_changed_at            timestamptz NOT NULL DEFAULT now(),
    closed_at                    timestamptz,
    closed_by_identity_id        uuid
);

COMMENT ON TABLE service_sessions IS '客户会话客服处理周期';
COMMENT ON COLUMN service_sessions.id IS '客服处理周期编号';
COMMENT ON COLUMN service_sessions.created_at IS '创建时间';
COMMENT ON COLUMN service_sessions.updated_at IS '更新时间';
COMMENT ON COLUMN service_sessions.organization_id IS '所属企业编号';
COMMENT ON COLUMN service_sessions.conversation_id IS '客户会话编号';
COMMENT ON COLUMN service_sessions.contact_channel_identity_id IS '联系人渠道身份编号';
COMMENT ON COLUMN service_sessions.sequence IS '会话内处理周期序号';
COMMENT ON COLUMN service_sessions.status IS '客服处理状态：waiting、active、pending、closed';
COMMENT ON COLUMN service_sessions.team_id IS '负责团队编号';
COMMENT ON COLUMN service_sessions.assignee_identity_id IS '负责人企业身份编号';
COMMENT ON COLUMN service_sessions.opening_message_id IS '处理周期首条消息编号';
COMMENT ON COLUMN service_sessions.last_message_id IS '处理周期最后消息编号';
COMMENT ON COLUMN service_sessions.last_message_at IS '处理周期最后消息发生时间';
COMMENT ON COLUMN service_sessions.assigned_at IS '首次分配时间';
COMMENT ON COLUMN service_sessions.first_response_at IS '首次客服响应时间';
COMMENT ON COLUMN service_sessions.status_changed_at IS '处理状态最后变更时间';
COMMENT ON COLUMN service_sessions.closed_at IS '关闭时间';
COMMENT ON COLUMN service_sessions.closed_by_identity_id IS '关闭人企业身份编号';

CREATE UNIQUE INDEX service_sessions_org_conversation_sequence_unique
    ON service_sessions (
        organization_id,
        conversation_id,
        sequence
    );

COMMENT ON INDEX service_sessions_org_conversation_sequence_unique
    IS '企业客户会话处理周期序号唯一索引';

CREATE UNIQUE INDEX service_sessions_org_opening_message_unique
    ON service_sessions (organization_id, opening_message_id);

COMMENT ON INDEX service_sessions_org_opening_message_unique
    IS '企业客服处理周期首条消息唯一索引';

CREATE UNIQUE INDEX service_sessions_org_conversation_open_unique
    ON service_sessions (organization_id, conversation_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_org_conversation_open_unique
    IS '企业客户会话未结束处理周期唯一索引';

CREATE UNIQUE INDEX service_sessions_org_channel_identity_open_unique
    ON service_sessions (organization_id, contact_channel_identity_id)
    WHERE status IN ('waiting', 'active', 'pending');

COMMENT ON INDEX service_sessions_org_channel_identity_open_unique
    IS '企业渠道身份未结束处理周期唯一索引';

CREATE INDEX service_sessions_org_conversation_last_message_index
    ON service_sessions (
        organization_id,
        conversation_id,
        last_message_at DESC,
        id DESC
    );

COMMENT ON INDEX service_sessions_org_conversation_last_message_index
    IS '企业客户会话处理周期最后消息索引';

CREATE INDEX service_sessions_org_channel_identity_last_message_index
    ON service_sessions (
        organization_id,
        contact_channel_identity_id,
        last_message_at DESC,
        id DESC
    );

COMMENT ON INDEX service_sessions_org_channel_identity_last_message_index
    IS '企业渠道身份处理周期最后消息索引';
```

`sequence` 在每条 Conversation 内从 1 开始。客户完整历史仍按 `conversation_id` 查询，ServiceSession 摘要只覆盖 `service_session_id = 本行` 的消息：

- `opening_message_id` 是本批次第一条消息，创建后不可修改。
- `last_message_id` 和 `last_message_at` 必须指向本批次同一条消息，且时间等于该消息的 `originated_at`。
- ServiceSession 必须随首条消息创建，三个消息摘要字段都非空；只有一条消息时，开启和最后消息编号相同。
- 只在写入属于本 ServiceSession 的消息且 `(originated_at, id)` 严格变大时，同时更新最后消息编号和时间。
- ServiceSession 关闭后摘要冻结，禁止把 Conversation 摘要复制到已经关闭的批次。
- 当本 ServiceSession 的最后消息同时也是 Conversation 全局最后消息时，两级摘要数值相同；这是数据结果，不是写入捷径。

两个部分唯一索引分别表达长期和首版产品不变量：

- Conversation 级唯一长期保留。
- 渠道身份级唯一在以后明确开放并行入站线程时删除。

### 6.6 `messages`

`messages` 保存 Conversation 中的消息事实：

```sql
CREATE TABLE messages (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    organization_id        uuid NOT NULL,
    conversation_id        uuid NOT NULL,
    service_session_id     uuid,
    sender_participant_id  uuid,
    type                   text NOT NULL,
    body                   text NOT NULL DEFAULT '',
    reply_to_message_id    uuid,
    thread_root_message_id uuid,
    idempotency_key        text,
    originated_at          timestamptz NOT NULL,
    edited_at              timestamptz,
    deleted_at             timestamptz
);

COMMENT ON TABLE messages IS '会话消息';
COMMENT ON COLUMN messages.id IS '消息编号';
COMMENT ON COLUMN messages.created_at IS '创建时间';
COMMENT ON COLUMN messages.updated_at IS '更新时间';
COMMENT ON COLUMN messages.organization_id IS '所属企业编号';
COMMENT ON COLUMN messages.conversation_id IS '所属会话编号';
COMMENT ON COLUMN messages.service_session_id IS '所属客服处理周期编号';
COMMENT ON COLUMN messages.sender_participant_id IS '发送参与者编号';
COMMENT ON COLUMN messages.type IS '消息类型：text、system';
COMMENT ON COLUMN messages.body IS '消息文本内容';
COMMENT ON COLUMN messages.reply_to_message_id IS '回复目标消息编号';
COMMENT ON COLUMN messages.thread_root_message_id IS '讨论串根消息编号';
COMMENT ON COLUMN messages.idempotency_key IS '消息写入幂等标识';
COMMENT ON COLUMN messages.originated_at IS '消息在来源端发生时间';
COMMENT ON COLUMN messages.edited_at IS '最后编辑时间';
COMMENT ON COLUMN messages.deleted_at IS '删除时间';

CREATE INDEX messages_org_conversation_originated_index
    ON messages (
        organization_id,
        conversation_id,
        originated_at DESC,
        id DESC
    );

COMMENT ON INDEX messages_org_conversation_originated_index
    IS '企业会话消息发生时间索引';

CREATE INDEX messages_org_service_session_originated_index
    ON messages (
        organization_id,
        service_session_id,
        originated_at DESC,
        id DESC
    )
    WHERE service_session_id IS NOT NULL;

COMMENT ON INDEX messages_org_service_session_originated_index
    IS '企业客服处理周期消息发生时间索引';

CREATE UNIQUE INDEX messages_organization_idempotency_unique
    ON messages (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

COMMENT ON INDEX messages_organization_idempotency_unique
    IS '企业消息幂等标识唯一索引';
```

客户消息必须具有 `service_session_id` 和 `sender_participant_id`。内部单聊和群聊消息不创建 ServiceSession，因此 `service_session_id` 允许为空；系统消息的发送参与者以后允许为空。

`messages.updated_at` 是统一的记录更新时间，首个网站消息 PR 创建消息时由默认值写入，当前不额外更新它。未来编辑、删除等消息变更必须更新 `updated_at`；`edited_at` 仍只表达用户可见内容最后编辑时间，不能由通用更新时间替代。

`idempotency_key` 的空字符串不是“未设置”：它会进入部分唯一索引。所有 Action 必须把没有幂等键表达为 `NULL`，禁止写入空字符串。

## 7. 迁移与模型规则

本 PR 只增加迁移文件，不修改、重排、重命名或删除仓库中任何已有迁移。新增一条联系人字段变更迁移，六张新表分别使用一个建表迁移；建表迁移命名遵循：

```text
YYYYMMDDHHMMSS_create_<table>_table.sql
```

七个新增迁移的时间戳必须晚于现有 `20260822150002`，顺序固定为：

```text
alter_contacts_created_by_user_id_nullable
chat_subjects
conversations
customer_conversations
conversation_participants
service_sessions
messages
```

联系人字段变更迁移只调整 `contacts.created_by_user_id` 的可空性和注释。每个建表迁移只创建一张表，不创建外键和 `CHECK`。所有新增或修改的数据库对象都必须在同一迁移中写全简洁中文注释，不允许留到模型或后续迁移补充：

- 每张新表都有 `COMMENT ON TABLE`。
- 每一列都有 `COMMENT ON COLUMN`，包括编号、时间、可空摘要和技术字段，不因字段含义看似明显而省略。
- 六张新表都把主键、`created_at`、`updated_at` 放在字段列表最前面；`customer_conversations.conversation_id` 是该表主键。六张表都具有两个标准审计时间，即使当前业务暂不使用 `updated_at` 也不得省略。
- `kind`、`type`、`status`、`role` 等枚举型字段的列注释必须在字段含义后列出当前全部可用值，并与 `internal/domain` 中的领域值完全一致；新增枚举值时通过新的向前迁移同步更新列注释。
- 每个显式创建的索引都有 `COMMENT ON INDEX`。
- 每个显式命名的约束都有 `COMMENT ON CONSTRAINT ... ON ...`。
- 联系人变更迁移在 Up 中写入新的可空语义，在 Down 中恢复原字段注释。

七个新增迁移能够在空库和已执行现有迁移的数据库中顺序执行。六个建表迁移的 Down 使用各自的 `DROP TABLE`，联系人字段变更迁移的 Down 恢复非空约束和原注释；整体按反序逐个回滚。迁移已经被其他环境应用后视为不可变，更正结构只能继续增加新的向前迁移。

因为迁移不创建外键，`opening_message_id` 和 `last_message_id` 可以先保存尚未插入的预生成消息编号。后续入站 Action 必须在事务外预生成所需 UUID；本 PR 不插入 Fixture 或业务行证明该写入顺序。

服务端模型分别放在 `internal/storage/server/models`，保留 `//go:build server`。Bun 结构体字段顺序与表一致：主键后紧跟 `CreatedAt`、`UpdatedAt`，再放其他业务字段。模型只表达数据库字段，不增加 `json` tag，也不在模型 Hook 中维护企业边界、枚举、关联一致性或状态流转；这些规则由后续 Action 显式维护。聊天模型不是 appservice DTO，不进入 Wails 绑定。

本 PR 不修改桌面端和移动端 SQLite 迁移。聊天事实当前只保存在企业服务器 PostgreSQL。

## 8. 验收标准

### 8.1 数据库

- 仓库中所有已有迁移文件保持内容、名称和顺序不变。
- 空库和已执行现有迁移的数据库都能够继续执行一条联系人字段变更迁移和六张新表迁移，不要求 `migrate:reset`。
- 七个新增迁移可以按反序逐个回滚；联系人字段迁移的 Down 同时恢复非空约束和原注释。
- 每个新建表迁移只创建一张表，不包含外键和 `CHECK`；枚举可用值只写入列注释，不通过数据库约束限制；联系人字段变更迁移不创建新表。
- 每张新表、每一列、每个显式索引和每个具名约束都在所属迁移中具有简洁中文数据库注释，无遗漏或后补注释。
- 所有枚举型字段的列注释完整列出当前可用值，并与领域值定义一致。
- 六张新表的字段顺序均为主键、`created_at`、`updated_at`、其他业务字段，且模型完整包含两个审计时间；本 PR 不修改历史迁移来统一旧表。
- `contacts.created_by_user_id` 可空，`source_channel_id` 保持非空。
- `customer_conversations` 不再限制一个渠道身份只能出现一次。
- 迁移中存在 Conversation 级和渠道身份级两个未结束 ServiceSession 部分唯一索引；本 PR 不插入业务数据测试状态流转。
- Conversation 最后消息字段可空且必须成对维护，ServiceSession 的开启和最后消息字段非空，迁移和模型与该可空性一致。

### 8.2 模型

- 六个 Bun 模型完整表达数据库字段和可空性。
- `Contact.CreatedByUserID` 使用 `*string`。
- `CreateContactAction` 继续用指针写入当前用户编号。
- 聊天 Bun 模型保留 server build tag，不增加 `json` tag。
- 模型中不增加 Action、HTTP、Cookie、路由解析或界面展示逻辑。

### 8.3 联系人界面

- 新增菜单中不再显示“添加外部联系人”。
- 新增成员、AI 员工和团队的入口保持可用。
- 只注释目标 `DropdownMenuItem`，直接访问 `/contacts/external?new=1` 仍保留原表单行为。
- 联系人列表、详情、编辑、删除和恢复保持不变。
- `ContactForm`、联系人 Action 和 appservice 契约不删除。

### 8.4 边界

- 打开网站聊天页、点击开始聊天和提交演示文本仍不会写入新表。
- 本 PR 不注册任何 `/api/public` 路由。
- 管理端收件箱仍是占位数据，不因为表已经存在而伪造会话。
- 不生成新的 Wails 前端业务绑定；本 PR 没有 appservice 契约变化。

## 9. 实施顺序

1. 增加聊天领域值。
2. 新增联系人字段可空性迁移并调整 Bun 模型，同步修改 `CreateContactAction` 的指针赋值；不修改原联系人建表迁移。
3. 按依赖顺序增加六张建表迁移，在各迁移中写全表、列、显式索引和具名约束的中文注释。
4. 增加六个服务端 Bun 模型。
5. 注释管理端手动添加外部联系人菜单项。
6. 验证迁移、模型和现有联系人行为。

每一步保持当前代码可编译。本 PR 不为了提前证明下一 PR 而加入临时写入 Action、Fixture、演示数据或未使用的公开 DTO。
