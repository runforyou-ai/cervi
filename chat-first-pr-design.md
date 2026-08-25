# 网站访客首条消息与会话列表 PR 设计

## 1. PR 定位

本 PR 基于 `main` 提交 `1bfaed739628cfd364368c010d2b6a599e1fcebb` 开发。

本 PR 把网站访客 Messenger 从内存演示界面改成真实的客户消息入口。没有未结束批次时，访客点击“开始聊天”只进入本地草稿页面，不创建数据库记录；已有未结束批次时直接打开真实历史。访客第一次提交合法文本后，服务端在同一事务中创建联系人、渠道身份、聊天主体、长期客户会话、会话参与者、客服处理批次和文本消息。访客返回首页或消息页后，页面从服务端返回的客服处理批次中展示真实会话列表。页面刷新后仍能恢复列表和消息历史。

本 PR 同时建立客户会话后续接入企业收件箱、人工回复和 Agent 回复所需的数据底座。`chat-roadmap.md` 是总体路线参考，本设计根据当前网站 Messenger、渠道路由、联系人和 Wails HTTP 挂载方式确定本 PR 的实际边界。两份文档不一致时，以本设计明确记录的本 PR 决策为准；路线图对应章节同步说明提前引入 `ServiceSession` 的原因。

本 PR 只完成访客侧可独立验收的持久化闭环。企业成员收件箱读取和回复紧随其后单独交付，不为了让数据库中的首条消息立即出现在客服界面而扩大本 PR。当前阶段的“正在等待团队成员”表示消息已经进入持久化服务批次，不表示成员端已经具备处理入口。

PR 标题使用：

```text
feat: 接入网站访客文本消息与会话列表
```

## 2. 当前实现

网站访客 Messenger 位于 `internal/publicweb`。独立链接使用 `/chat/{channelID}`，嵌入页面使用 `/embed/widget/{channelID}`。两个入口共用 `page.html`、`chat.js` 和 `chrome.css`。

当前实现具有以下行为：

- `ChatService` 和 `EmbedService` 只接受 `GET` 和 `HEAD`。
- 点击“开始聊天”只创建 JavaScript 内存对象。
- 提交文本后立即在浏览器中拼接访客消息。
- 提交文本后约一秒自动拼接演示客服回复。
- 首页最近会话和消息页会话行只读取 JavaScript 内存状态。
- 消息页 HTML 只包含一条静态会话占位结构。
- 页面刷新后联系人、会话、批次和消息全部丢失。
- 附件、语音、未读和客服在线状态均为界面演示。

本 PR 删除真实入口中的本地会话事实和演示客服回复。管理端 Messenger 预览继续使用本地演示，不访问数据库。

## 3. 产品对象边界

本 PR 使用以下关系：

```text
Channel
  └── ContactChannelIdentity
        ├── Contact
        │     └── ChatSubject
        │           └── ConversationParticipant
        └── CustomerConversation
              └── Conversation
                    ├── ServiceSession
                    └── Message
                          ├── sender_participant_id
                          └── service_session_id
```

`Conversation` 保存一个渠道身份的长期消息时间线。`ServiceSession` 保存一次等待、接待、挂起和结束的客服处理过程。访客界面把 `ServiceSession` 展示为一条“会话”，不把底层长期 `Conversation` 直接展示为列表项。

同一个渠道身份始终复用同一个 `Conversation`。同一个渠道身份可以先后拥有多个 `ServiceSession`。一个未结束的 `ServiceSession` 接收该访客的后续消息。已经结束的批次不重新打开，访客再次发送消息时创建下一个批次。

当前不合并联系人。不同网站渠道、不同访客 Token 或清除 Cookie 后产生的新 Token 分别创建独立联系人、长期会话和服务批次。系统不按姓名、浏览器信息、IP 地址、手机号或其他字段识别同一自然人。

## 4. PR 范围

本 PR 完成以下内容：

- 调整渠道自动创建联系人的创建人字段。
- 创建聊天领域值、六张核心业务表和对应 Bun 模型。
- 给文本消息增加明确的客服处理批次归属。
- 签发和恢复网站访客 Token。
- 提供访客 Messenger 初始化接口。
- 提供网站访客文本发送接口。
- 提供访客服务批次列表和批次消息历史查询。
- 把独立聊天链接和嵌入 Messenger 接到真实接口。
- 在首次真实消息事务中建立等待中的 `ServiceSession`。
- 按网站渠道现有初始路由和失败路由填充团队或负责人。
- 保留管理端 Messenger 预览的本地演示能力。

本 PR 不完成以下内容：

- 企业成员收件箱和客户会话管理页面。
- 企业成员侧客户会话列表和消息历史 Query。
- 企业成员或 Agent 回复。
- 服务批次领取、转接、挂起和结束接口。
- 客服协作者关系和转接历史。
- 未读数和已读回执。
- WebSocket、SSE、通知和后台轮询。
- 附件、图片、视频、语音和文件上传。
- 消息引用、编辑、删除、反应和系统消息写入。
- 联系人合并和跨渠道身份关联。
- 网站访客主动修改或关闭服务批次。
- 演示客服回复写入数据库。

## 5. 领域值

`internal/domain` 增加聊天领域值。

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

本 PR 只写入 `contact`、`customer`、`active`、`member`、`waiting` 和 `text`。其余值确定核心契约，不开放对应写入入口。

## 6. 联系人调整

`contacts.created_by_user_id` 改为可空。网站渠道自动创建联系人时不伪造创建用户。

```text
created_by_user_id = NULL
source_channel_id  = 当前网站渠道编号
stage              = visitor
```

`contacts.source_channel_id` 保持非空。当前所有自动联系人都来自明确的网站渠道。联系人合并、导入和无来源联系人不进入本 PR。

`internal/storage/server/models.Contact.CreatedByUserID` 改为 `*string`。现有手工创建 Action 继续写入当前用户编号。现有联系人查询和 appservice DTO 不暴露创建用户，不需要修改联系人读取契约。

管理界面的手工添加联系人入口、`?new=1` 路由和现有联系人 CRUD 保持不变。联系人入口是否开放是独立产品决策，不与网站聊天接入绑定。

## 7. 数据表

### 7.1 `chat_subjects`

`chat_subjects` 给不同来源的聊天主体分配稳定编号。参与者、发送者、后续提醒和反应只引用聊天主体，不重复保存来源类型和来源编号。

```sql
CREATE TABLE chat_subjects (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id  uuid NOT NULL,
    kind             text NOT NULL,
    source_id        uuid NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE UNIQUE INDEX chat_subjects_org_kind_source_unique
    ON chat_subjects (organization_id, kind, source_id);
```

本 PR 只创建 `kind = contact` 的记录。Action 校验 `source_id` 对应相同企业的联系人。`kind` 和 `source_id` 创建后不可修改。联系人进入回收站后保留聊天主体，避免历史消息失去发送者关系。

### 7.2 `conversations`

`conversations` 保存长期消息容器。

```sql
CREATE TABLE conversations (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id        uuid NOT NULL,
    type                   text NOT NULL,
    status                 text NOT NULL DEFAULT 'active',
    title                  text,
    created_by_subject_id  uuid,
    last_message_at        timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE INDEX conversations_org_type_status_last_message_index
    ON conversations (
        organization_id,
        type,
        status,
        last_message_at DESC NULLS LAST,
        id DESC
    );
```

网站访客首条消息创建 `type = customer`、`status = active` 的会话。`created_by_subject_id` 写联系人 ChatSubject。客户会话标题保持为空，界面继续使用网站渠道标题和客服身份文案。

`last_message_at` 保存该长期会话中最大的消息 `originated_at`。新消息使用 `GREATEST` 语义更新。幂等重试不修改该字段。

### 7.3 `customer_conversations`

`customer_conversations` 把长期会话绑定到联系人渠道身份。

```sql
CREATE TABLE customer_conversations (
    conversation_id              uuid PRIMARY KEY,
    organization_id              uuid NOT NULL,
    contact_channel_identity_id  uuid NOT NULL,
    created_at                   timestamptz NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE UNIQUE INDEX customer_conversations_org_channel_identity_unique
    ON customer_conversations (
        organization_id,
        contact_channel_identity_id
    );

CREATE INDEX customer_conversations_org_conversation_index
    ON customer_conversations (organization_id, conversation_id);
```

同一个渠道身份只对应一个长期客户会话。该表不保存排队、负责人、批次状态、未读、投递和限流字段。

### 7.4 `conversation_participants`

`conversation_participants` 保存聊天主体在会话中的稳定成员关系。消息引用参与者编号，不直接引用联系人或企业身份编号。

```sql
CREATE TABLE conversation_participants (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id  uuid NOT NULL,
    conversation_id  uuid NOT NULL,
    subject_id       uuid NOT NULL,
    role             text NOT NULL DEFAULT 'member',
    joined_at        timestamptz NOT NULL DEFAULT now(),
    left_at          timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE UNIQUE INDEX conversation_participants_org_conversation_subject_unique
    ON conversation_participants (
        organization_id,
        conversation_id,
        subject_id
    );

CREATE INDEX conversation_participants_org_subject_active_index
    ON conversation_participants (
        organization_id,
        subject_id,
        left_at,
        conversation_id
    );
```

网站访客使用 `role = member`。同一个联系人主体在同一个长期会话中只有一行。服务批次的团队和负责人不会自动成为会话参与者。企业成员或 Agent 实际回复或明确加入协作时才创建对应参与者。

### 7.5 `service_sessions`

`service_sessions` 保存一次客服处理过程，也是网站访客消息页展示的列表对象。

```sql
CREATE TABLE service_sessions (
    id                    uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id       uuid NOT NULL,
    conversation_id       uuid NOT NULL,
    sequence              bigint NOT NULL,
    status                text NOT NULL DEFAULT 'waiting',
    team_id               uuid,
    assignee_identity_id  uuid,
    opening_message_id    uuid NOT NULL,
    last_message_id       uuid NOT NULL,
    last_message_at       timestamptz NOT NULL,
    assigned_at           timestamptz,
    first_response_at     timestamptz,
    status_changed_at     timestamptz NOT NULL DEFAULT now(),
    closed_at             timestamptz,
    closed_by_identity_id uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
```

索引：

```sql
CREATE UNIQUE INDEX service_sessions_org_conversation_sequence_unique
    ON service_sessions (
        organization_id,
        conversation_id,
        sequence
    );

CREATE UNIQUE INDEX service_sessions_org_opening_message_unique
    ON service_sessions (
        organization_id,
        opening_message_id
    );

CREATE UNIQUE INDEX service_sessions_org_conversation_open_unique
    ON service_sessions (
        organization_id,
        conversation_id
    )
    WHERE status IN ('waiting', 'active', 'pending');

CREATE INDEX service_sessions_org_conversation_last_message_index
    ON service_sessions (
        organization_id,
        conversation_id,
        last_message_at DESC,
        id DESC
    );
```

`sequence` 从 1 开始。Action 锁定 `customer_conversations` 行后分配下一个序号。

`opening_message_id` 指向开启本批次的第一条客户消息。`last_message_id` 和 `last_message_at` 保存批次当前最后一条消息，用于稳定排序和列表预览。消息写入与这两个字段的更新处于同一事务。

`team_id` 和 `assignee_identity_id` 复用现有渠道路由概念：

| 路由结果 | `team_id` | `assignee_identity_id` |
| --- | --- | --- |
| 公共队列 | 空 | 空 |
| 团队队列 | 团队编号 | 空 |
| 指定成员或 Agent | 空 | 企业身份编号 |
| 团队成员领取 | 团队编号 | 企业身份编号 |

初始路由不可用时使用渠道失败路由。失败路由也不可用时进入公共队列。直接路由到成员或 Agent 时同时写入 `assigned_at`。

本 PR 不增加团队队列、负责人队列和公共队列索引。访客查询固定从渠道身份定位长期会话，再按长期会话读取批次。企业收件箱 PR 根据实际列表条件增加队列索引。

### 7.6 `messages`

`messages` 保存长期会话中的消息事实。`service_session_id` 明确一条客户服务消息属于哪个处理批次，使访客列表项可以加载自己的消息历史，不依赖时间范围猜测。

```sql
CREATE TABLE messages (
    id                     uuid PRIMARY KEY DEFAULT uuidv7(),
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
    created_at             timestamptz NOT NULL DEFAULT now(),
    edited_at              timestamptz,
    deleted_at             timestamptz
);
```

索引：

```sql
CREATE INDEX messages_org_conversation_originated_index
    ON messages (
        organization_id,
        conversation_id,
        originated_at DESC,
        id DESC
    );

CREATE INDEX messages_org_service_session_originated_index
    ON messages (
        organization_id,
        service_session_id,
        originated_at DESC,
        id DESC
    )
    WHERE service_session_id IS NOT NULL;

CREATE UNIQUE INDEX messages_organization_idempotency_unique
    ON messages (organization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

网站客户文本消息必须具有 `service_session_id` 和 `sender_participant_id`。Action 校验服务批次、参与者、会话和企业完全一致。未来内部单聊和群聊消息的 `service_session_id` 保持为空。未来系统消息的 `sender_participant_id` 允许为空。

网站消息幂等键使用：

```text
chmsg:<channel_id>:<client_message_id>
```

`client_message_id` 是浏览器为一次文本提交生成的 UUID。相同键、相同渠道身份、相同发送参与者和相同规范化正文返回原消息。请求中的 `serviceSessionId` 和本次服务器接收时间不参与等价比较。相同键对应不同渠道身份、发送参与者或正文时返回幂等冲突。

## 8. 迁移规则

六张新表分别使用一个建表迁移。迁移不创建外键和 `CHECK`。迁移为表、业务字段和索引添加简洁中文说明。

`contacts.created_by_user_id` 直接在现有联系人建表迁移中改为可空。当前阶段不保留旧结构兼容迁移，不写历史数据回填。

该迁移同步修改 `contacts.created_by_user_id` 的中文 `COMMENT ON`，明确人工创建时记录用户、渠道自动创建时为空。Bun 模型使用 `*string` 表达该字段。

所有模型关系、枚举、企业边界和非空业务规则由 Action 显式维护。

## 9. 网站访客身份

### 9.1 Token

网站访客身份使用每个渠道独立的随机 Token。Token 使用 32 位小写字母和数字，不包含联系人编号、会话编号、时间和浏览器信息。

Cookie 名称使用：

```text
cervi_visitor_<去掉连字符的 channel_id>
```

Cookie 属性使用：

```text
Path=/
HttpOnly
Max-Age=31536000
SameSite=Lax                     独立聊天链接
SameSite=None; Secure            HTTPS 嵌入 Messenger
```

HTTPS 请求统一设置 `Secure`。Cookie 不设置 `Domain`。HTTP 嵌入页不能设置可用的跨站 `SameSite=None` Cookie，因为该属性必须同时具有 `Secure`。这种环境只依靠初始化响应中的 Header Token 维持当前页面，刷新后不保证恢复原身份。

访客初始化接口在 Token 缺失或格式非法时签发新 Token，并通过响应 Cookie 和响应 JSON 返回。浏览器 JavaScript 只在当前页面内存中保存响应 Token，并在后续请求中通过 `X-Cervi-Visitor-Token` 发送。Token 不写入 `localStorage`、`sessionStorage` 或 URL。

服务端按 `X-Cervi-Visitor-Token`、Cookie 的顺序读取 Token。Header 解决当前页面处于禁止第三方 Cookie 的浏览器环境时的连续请求问题。第三方 Cookie 完全被阻止时，页面刷新后不保证恢复原访客身份。

只有初始化接口可以在 Token 缺失或非法时签发新 Token。发送消息和读取历史缺少有效 Token 时返回 `400`，不签发替代 Token，也不以新身份继续写入或读取。成员 Bearer Token 不参与公开 Messenger 身份判断。

### 9.2 数据创建时机

打开聊天页、打开嵌入 Messenger、点击“开始聊天”、输入草稿和获取初始化状态都不创建联系人或聊天记录。

第一条合法文本提交在同一事务中创建业务记录。渠道身份使用：

```text
contact_channel_identities.external_id = web-session:<visitor_token>
```

公开 HTTP 适配器把 Token 规范化为上述 `external_id` 后再调用应用服务。Action 和 Query 只接收 `external_id`，不接收裸 Token，不感知 Cookie 或 Header。查询渠道身份时同时校验渠道编号、企业编号和外部编号。

## 10. 应用分层

### 10.1 公开页面

`internal/publicweb` 继续负责以下内容：

- 输出独立 Messenger HTML。
- 输出嵌入 Messenger HTML。
- 输出主题化 `widget.js`。
- 输出 `chat.js` 和页面样式。
- 校验嵌入宿主网站。
- 管理端预览本地演示。

`internal/publicweb` 不直接访问 Bun，不创建联系人、会话、服务批次或消息。

### 10.2 公开 HTTP 适配器

公开路由直接注册到现有 `internal/api.Service` 的 Gin Router，Gin 内部路径使用：

```text
/public/website-channels/:channelID/messenger
/public/website-channels/:channelID/messages
/public/website-channels/:channelID/service-sessions/:serviceSessionID/messages
```

现有 Wails 服务继续只挂载一次 `/api`，因此对外路径仍为 `/api/public/...`。不新增挂载到 `/api/public` 的 Wails Service。Wails v3 按注册顺序进行前缀匹配，先注册的 `/api` 会截获 `/api/public`；单独挂载会使新 Gin Router 无法收到请求。

公开 Handler 不调用成员认证逻辑，不要求也不解释 Bearer Token。请求即使携带成员 Bearer Token，也只按网站访客 Token 确定身份，不扩大访问范围。

公开适配器负责：

- 读取渠道编号和路径参数。
- 读取、签发和设置访客 Cookie。
- 读取 `X-Cervi-Visitor-Token`。
- 把裸 Token 规范化为 `web-session:<token>`。
- 绑定和限制 JSON 请求体。
- 写入 HTTP 状态、错误体和 `Cache-Control: no-store`。
- 按 `Accept-Language` 本地化并写入 `Content-Language` 和 `Vary: Accept-Language`。
- 调用访客应用服务。

公开适配器不直接读写聊天表。

### 10.3 访客应用服务

`internal/appservice` 增加未注册为 Wails 绑定的 `WebsiteVisitorService`。该服务定义公开 Messenger 使用的唯一 DTO，并把请求转给独立的 `WebsiteVisitorBackend` 接口。服务端实现使用单独的 `WebsiteVisitorDirectBackend`，直接调用访客 Action 和 Query，并把 Action 的语言无关错误映射为使用请求语言的 `appservice.Error`。它不实现成员 `Backend`，也不把匿名方法加入现有 `DirectBackend` 的成员鉴权接口。

`WebsiteVisitorService` 不加入当前注册给 Wails 的 `appservice.Service`，不生成桌面端和移动端绑定。公开 Messenger JavaScript 只调用 HTTP 接口。

持久命令和查询经过以下路径：

```text
chat.js
  → 现有 /api Gin Service 中的公开 Handler
  → WebsiteVisitorService
  → WebsiteVisitorDirectBackend
  → conversation Action / Query
  → PostgreSQL
```

## 11. HTTP 接口

### 11.1 初始化 Messenger

```http
GET /api/public/website-channels/{channelID}/messenger
X-Cervi-Visitor-Token: <token，可选>
```

响应：

```json
{
  "visitorToken": "32位访客Token",
  "serviceSessions": [
    {
      "id": "service-session-id",
      "status": "waiting",
      "preview": "最后一条消息正文",
      "lastMessageAt": "2026-08-25T00:00:00Z"
    }
  ]
}
```

`serviceSessions` 来源于当前渠道身份长期会话下的 `service_sessions`。列表按 `(last_message_at DESC, id DESC)` 返回最近 20 条。本 PR 不提供访客端历史批次翻页入口。访客产品文案可以继续把列表项称为“会话”，公开契约始终使用 `serviceSession`，不把客服处理批次命名为 `conversation`。

列表包含未结束和已经关闭的批次。页面需要恢复“开始聊天”目标时，只在 `status IN (waiting, active, pending)` 的条目中选择至多一条，不使用列表第一项代替未结束批次。数据库部分唯一索引保证同一长期会话最多返回一条未结束批次。

没有业务记录时返回空数组。接口仍签发访客 Token，但不创建数据库记录。

### 11.2 发送文本消息

```http
POST /api/public/website-channels/{channelID}/messages
Content-Type: application/json
X-Cervi-Visitor-Token: <token>
```

请求：

```json
{
  "clientMessageId": "0198ddee-c056-7bc5-a1d9-586f878ee966",
  "serviceSessionId": null,
  "body": "你好，我想了解产品。"
}
```

`serviceSessionId` 在“开始聊天”产生的新草稿中为空，在已有列表项中继续发送时填写当前批次编号。

响应：

```json
{
  "serviceSession": {
    "id": "service-session-id",
    "status": "waiting",
    "preview": "你好，我想了解产品。",
    "lastMessageAt": "2026-08-25T00:00:00Z"
  },
  "reusedExistingServiceSession": false,
  "message": {
    "id": "message-id",
    "author": "visitor",
    "body": "你好，我想了解产品。",
    "originatedAt": "2026-08-25T00:00:00Z",
    "createdAt": "2026-08-25T00:00:00Z"
  }
}
```

服务端忽略客户端时间。`originated_at` 使用服务端首次收到该提交的时间，并在完整事务重试之间保持一致。

`reusedExistingServiceSession` 表示返回消息不是该批次的开启消息，稳定按 `service_sessions.opening_message_id != message.id` 计算。该语义在幂等重试后保持不变，不取决于当前事务是否实际执行了插入。只有本地空草稿得到该值时才立即重读返回批次的历史，不能只替换草稿编号。已经打开真实批次后的普通后续发送即使返回 `true`，也只合并响应消息和批次摘要，不整页重读历史。

客户端提交失败时保留正文和 `clientMessageId`。原样重试继续使用相同编号。用户修改正文后生成新的 `clientMessageId`。

### 11.3 读取批次消息

```http
GET /api/public/website-channels/{channelID}/service-sessions/{serviceSessionID}/messages[?before=<cursor>|after=<cursor>]
X-Cervi-Visitor-Token: <token>
```

`before` 和 `after` 不能同时提供。无游标时读取最近 50 条；`before` 读取更早消息；`after` 按正序读取游标之后的新消息。当前页面只使用无游标和 `before`，`after` 在本 PR 固定契约，供下一步客服回复轮询直接使用。

响应：

```json
{
  "messages": [
    {
      "id": "message-id",
      "author": "visitor",
      "body": "你好，我想了解产品。",
      "originatedAt": "2026-08-25T00:00:00Z",
      "createdAt": "2026-08-25T00:00:00Z"
    }
  ],
  "before": null,
  "after": "服务端编码游标"
}
```

`before` 和 `after` 都是可空字符串。空结果返回：

```json
{
  "messages": [],
  "before": null,
  "after": null
}
```

接口每次最多返回 50 条，使用 `(originated_at, id)` 作为稳定边界，统一按正序返回给页面。游标由服务端编码方向和元组，客户端只把游标作为不透明字符串回传，不自行拼接。无游标和 `before` 查询可以在数据库中倒序读取后再反转；`after` 查询直接正序读取。

无游标查询返回消息时，`after` 指向最新一条，供后续增量读取；存在更早记录时，`before` 指向当前最早一条，否则为 `null`。`before` 查询只返回下一段 `before`，没有更早记录时为 `null`，`after` 固定为 `null`。`after` 查询返回新消息时以最新一条生成下一次 `after`，没有新记录时返回 `after = null`，`before` 固定为 `null`；客户端在空结果时继续保留自己提交的旧游标。空历史同时返回 `before = null` 和 `after = null`。页面首次只加载最近 50 条，本 PR 不增加向上加载历史的界面，也不启动轮询。

接口通过访客 Token、渠道身份、客户会话和服务批次完整关系校验访问权。接口不凭 `serviceSessionID` 单独查询。

### 11.4 HTTP 错误

公开接口使用以下状态：

| 状态 | 含义 |
| --- | --- |
| `400` | Token 缺失或非法，或文本、客户端消息编号、游标非法 |
| `404` | 渠道停用、不存在，或批次不属于当前访客 |
| `409` | 幂等键对应不同渠道身份、发送参与者或正文 |
| `405` | 请求方法不允许 |
| `413` | 请求体超过限制 |
| `500` | 数据库或服务端内部失败 |

公开错误复用现有结构：

```json
{
  "error": {
    "kind": "invalid",
    "message": "本地化简短文案",
    "fields": {}
  }
}
```

`appservice.ErrorKind` 增加 `conflict`，默认映射 `409`。公开接口不建立另一套业务错误码。错误体不返回 SQL、约束名、访客 Token 或内部关联编号。

## 12. 入站文本事务

`ReceiveWebsiteCustomerTextMessage` 接收渠道编号、规范化后的 `external_id`、可空服务批次编号、客户端消息编号和正文。裸访客 Token 只停留在 HTTP 适配器。

`internal/actions/contact` 增加事务内 `EnsureChannelIdentity` 能力。该能力集中完成渠道与企业一致性校验、按 `(channel_id, external_id)` 查找或创建身份、自动联系人创建和回收站联系人恢复。它接收已经规范化的外部编号，不感知网站 Cookie/Header，后续其他渠道可以复用相同的身份维护规则。

Action 在进入事务重试前完成以下操作：

- 去除正文首尾空白。
- 校验正文非空且不超过 4000 个 Unicode 字符。
- 校验客户端消息编号是规范的 `8-4-4-4-12` UUID 字符串。
- 校验 `external_id` 符合网站身份格式和长度限制。
- 生成本次服务器接收时间。
- 生成新记录需要的 UUIDv7。

Action 最多执行三次完整事务尝试。每次事务执行以下步骤：

1. 读取已启用的网站渠道，取得企业编号、初始路由和失败路由，不对渠道行加锁。渠道是所有访客共享的配置行，不能作为访客消息的串行化点。
2. 按企业和幂等键查询已有消息及其渠道身份、长期会话、服务批次、发送参与者和正文。
3. 幂等记录完全一致时返回已有结果，不修改任何业务记录。
4. 调用 `EnsureChannelIdentity` 按 `(channel_id, external_id)` 查找联系人渠道身份。
5. 渠道身份不存在时创建 `stage = visitor` 联系人和渠道身份。联系人显示名称为空，创建用户为空，来源渠道为当前渠道，不伪造“匿名访客”名称。
6. 渠道身份存在且联系人处于回收站时恢复联系人。只有真实新消息执行恢复，界面在名称为空时展示本地化的“匿名访客”。
7. 确保联系人拥有唯一 `kind = contact` ChatSubject。
8. 确保渠道身份拥有唯一长期客户会话和客户会话扩展。已有长期会话为 `archived` 时恢复为 `active`。
9. 创建或取得 `customer_conversations` 后立即使用 `FOR UPDATE` 锁定该行。此行只对应当前渠道身份，用于串行分配批次序号和选择唯一未结束批次，不阻塞同一网站渠道的其他访客。
10. 确保联系人 ChatSubject 是该长期会话中的有效 `member` 参与者。已有参与者的 `left_at` 非空时清空并复用原行，保留首次 `joined_at`，不插入第二行。
11. 请求携带 `serviceSessionId` 时校验该批次属于当前访客和长期会话。该编号只表达访客当前打开的列表项，不允许绕过长期会话中的唯一未结束批次。
12. 长期会话存在 `waiting` 或 `active` 批次时复用该批次。请求携带其他已结束批次编号时仍然返回并使用当前未结束批次。
13. 长期会话存在 `pending` 批次时复用该批次，并改为 `active`、更新 `status_changed_at`。请求携带其他已结束批次编号时仍然返回并使用该批次。
14. 当前不存在未结束批次时创建 `sequence + 1`、`status = waiting` 的新批次。请求未携带批次编号或携带已结束批次编号都执行该规则。
15. 新批次按第一步读取的网站渠道初始路由设置 `team_id` 或 `assignee_identity_id`。写入前重新校验团队、成员或 Agent 仍属于当前企业并处于可用状态；初始目标不可用时解析并同样校验失败路由，两级都不可用时进入公共队列。渠道路由在事务期间发生变化不修改旧批次，下一次创建的新批次读取新路由。
16. 创建 `type = text` 消息，写入联系人参与者和当前服务批次。
17. 新批次同时写入 `opening_message_id`、`last_message_id` 和 `last_message_at`；已有批次更新 `last_message_id`、`last_message_at` 和 `updated_at`。
18. 使用 `GREATEST` 更新 `conversations.last_message_at` 和 `updated_at`。
19. 更新渠道身份 `last_seen_at`。本 PR 不根据重复消息更新显示名称。
20. 提交事务并返回批次摘要和消息。

新批次和首条消息需要互相引用。Action 在事务开始前生成服务批次和消息 UUIDv7，先写入包含 `opening_message_id` 的服务批次，再写入包含 `service_session_id` 的消息。任一写入失败时整个事务回滚。

幂等命中的等价集固定为同一渠道身份、同一发送参与者和同一规范化正文。请求中的 `serviceSessionId` 和本次服务器接收时间不参与比较。幂等命中直接返回已保存的批次与消息，不恢复联系人或会话，不更新参与者、`last_seen_at`、批次状态、最后消息字段和路由快照。

幂等命中沿 `Message → Participant → ChatSubject → Contact` 和 `Message → ServiceSession → CustomerConversation → ContactChannelIdentity` 两条关系核对发送主体、渠道身份、企业和会话归属。任一必需关系缺失或互相矛盾时返回内部错误，不能把残缺记录视为幂等成功。

以下命名唯一约束产生的 PostgreSQL `23505` 属于预期并发竞态：

```text
contact_channel_identities_channel_external_unique
chat_subjects_org_kind_source_unique
customer_conversations_org_channel_identity_unique
conversation_participants_org_conversation_subject_unique
service_sessions_org_conversation_open_unique
service_sessions_org_conversation_sequence_unique
messages_organization_idempotency_unique
```

命中上述约束后整笔事务回滚，并从第一步重新执行。其他数据库错误直接返回。

## 13. 服务批次规则

状态流转使用：

```text
waiting ──接待或首次回复──> active
active  ──等待客户────────> pending
pending ──客户发送新消息──> active
waiting ──结束────────────> closed
active  ──结束────────────> closed
pending ──结束────────────> closed
closed  ──客户发送新消息──> 新建下一批次
```

本 PR 只实现入站消息触发的以下行为：

- 首条消息创建 `waiting` 批次。
- `waiting` 和 `active` 批次接收后续消息。
- `pending` 批次收到客户消息后恢复为 `active`。
- 已经 `closed` 时创建下一批次。

接待、首次回复、挂起和结束命令由后续企业收件箱 PR 实现。本 PR 先固定表结构、唯一性和入站行为，避免后续用新的客户会话替代标准服务批次。

## 14. 会话查询

### 14.1 访客会话列表

列表 Query 接收 HTTP 适配器已经规范化的 `external_id`，不接收或解析裸访客 Token。

列表查询执行以下关系：

```text
channel_id + external_id
  → contact_channel_identities
  → customer_conversations
  → service_sessions
  → service_sessions.last_message_id
  → messages
```

查询只返回当前网站渠道身份的服务批次。不同渠道、不同 Token、其他联系人和其他企业的批次不会进入结果。

列表摘要跳过已删除消息的能力在消息删除功能落地时实现。本 PR 没有删除消息，直接读取 `last_message_id` 对应正文。

### 14.2 批次消息历史

消息历史同时使用以下条件：

```text
organization_id       = 当前渠道企业
conversation_id       = 当前渠道身份的长期会话
service_session_id    = 当前列表项
```

发送者通过以下关系解析：

```text
message.sender_participant_id
  → conversation_participants.subject_id
  → chat_subjects.kind + source_id
```

本 PR 只返回联系人消息，并映射为 `author = visitor`。企业成员或 Agent 回复落地时再为 `kind = organization_identity` 确定访客端展示值。网站渠道问候语是界面配置，不是持久消息，不进入历史接口。

`author` 只是访客 DTO 根据 `chat_subjects.kind` 生成的展示映射，不是聊天领域的发送者模型。本 PR 的访客 DTO 不复用或扩展现有占位 `domain.MessageAuthor`；持久发送者始终通过 `Message → Participant → ChatSubject` 解析。

## 15. Messenger 页面改造

### 15.1 初始化

真实独立链接和嵌入 Messenger 加载后立即请求初始化接口。独立链接从当前路径读取渠道编号，嵌入 Messenger 从服务端输出的 `data-channel-id` 读取，不从可编辑文案或查询参数推断。所有请求使用 `credentials: "same-origin"`。页面在请求期间保留现有框架，消息页显示加载状态。请求成功后保存内存 Token，并用返回批次渲染首页最近会话和消息页列表。初始化失败显示页内重试入口，不进入空白的本地成功状态。

管理端预览通过 `data-preview="true"` 识别。预览不请求初始化接口、不设置访客 Cookie、不创建数据库记录。

### 15.2 新建草稿

点击“开始聊天”时在初始化列表中筛选 `waiting`、`active`、`pending` 状态，只取至多一条未结束批次。存在该批次时直接打开它并读取真实历史，不创建空草稿。列表第一项可能是已经关闭的批次，不能用第一项代替上述筛选。不存在未结束批次时才创建本地草稿对象并进入聊天页。草稿没有服务端编号，不进入首页最近会话和消息页列表。空草稿返回上一层后直接丢弃。

只有本地空草稿显示 `cv-conversation-intro` 问候语。问候语作为时间线外的界面介绍，不追加成 `assistant` 气泡。打开任何已有服务端批次时不调用现有会插入问候气泡的 `startConversation()` 路径，刷新和重复打开都只显示历史接口返回的持久消息。

第一次提交合法文本成功后，响应中的 `serviceSession.id` 写入当前草稿。该对象从草稿变成真实服务批次，并插入列表首位。

初始化之后可能由另一个页面创建未结束批次。本地空草稿发送后，服务端返回 `reusedExistingServiceSession = true` 时，当前页面立即重读该批次完整历史，再进入真实时间线，不能只替换草稿编号或只显示本次发送的消息。已经打开真实批次后的普通发送不因该字段重读历史。

从已结束批次继续发送时，服务端创建下一批次并返回新的 `serviceSession.id`。页面保留旧批次列表项，新增并切换到新批次，当前时间线只显示新批次历史，不能把新消息追加到已结束批次。

### 15.3 发送文本

真实入口执行以下交互：

- 提交前生成 `clientMessageId`。
- 请求进行中禁用发送按钮。
- 请求成功后才把消息标记为已发送并清空输入框。
- 请求失败时保留正文和客户端消息编号。
- 原样重试复用客户端消息编号。
- 修改失败正文后生成新的客户端消息编号。
- 服务端响应决定消息时间、消息编号和服务批次编号。
- 响应批次编号与当前批次不同时按服务端编号切换时间线。
- 页面不使用乐观消息制造持久成功假象。
- 页面显示简短发送失败文案，不生成 Toast 或字段错误。

真实入口删除 `scheduleDemoReply`。发送成功后显示“正在等待团队成员”，不生成假的客服消息和未读状态。从列表或刷新结果打开 `status = waiting` 且最后一条持久消息来自访客的批次时，同样恢复该等待提示，不能只在当前页面发送成功后显示。其他状态不根据本地猜测显示等待提示。

管理端预览继续使用本地访客消息和演示客服回复，保持渠道界面配置预览能力。

### 15.4 会话列表

消息页把当前静态单行结构改为可重复渲染的容器。每个列表项使用服务批次编号作为键，显示渠道标题、最后消息正文和本地化时间。

首页最近会话使用列表第一项。没有真实服务批次时隐藏最近会话卡片并显示消息空状态。

页面返回首页或消息页时使用当前内存中的服务端响应重新渲染列表。页面刷新后使用初始化接口恢复相同列表。

点击列表项后请求该批次最近消息。请求成功后进入聊天页并显示真实历史。重复打开已经加载的批次时复用内存消息，页面刷新后重新读取。

本 PR 没有客服回复和已读事实，所有未读圆点保持隐藏。`cervi:unread` 不再根据演示回复发送 `true`。

### 15.5 文本以外的控件

真实入口只接入文本和表情。表情继续作为文本内容提交。

附件和语音按钮在真实入口中禁用并使用不可用状态，不再创建本地假消息。管理端预览继续展示附件和语音交互。文件和语音在文件消息 PR 中接入现有临时上传流程。

## 16. 访问控制与请求约束

公开 Messenger 请求不使用企业成员 Bearer Token。访客 Token 只授权访问当前渠道身份对应的服务批次和消息。

每次查询和写入都显式校验：

- 渠道存在、启用且类型为 `website`。
- 渠道身份属于当前渠道和企业。
- 联系人属于相同企业。
- ChatSubject 属于相同企业和联系人。
- CustomerConversation 属于相同企业和渠道身份。
- ServiceSession 属于相同企业和长期会话。
- Message 属于相同企业、长期会话和服务批次。
- Participant 属于相同企业和长期会话。

公开 JSON 接口不启用跨域访问。嵌入 Messenger 的 iframe 与接口同源请求 `/api/public`。管理端预览不会调用真实接口。

请求体设置小型固定上限。正文按 Unicode 字符限制为 4000，JSON 请求体上限设置为 16 KiB。服务端拒绝额外的文件正文和未声明的大型字段。

## 17. 日志

成功日志记录：

```text
organization_id
channel_id
conversation_id
service_session_id
message_id
created_contact
created_conversation
created_service_session
```

日志不记录访客 Token、Cookie、消息正文、外部身份编号和完整请求体。

失败日志记录稳定错误类别和内部错误，不把内部错误返回公开客户端。

## 18. 测试与验收

本 PR 通过数据库集成测试、公开 HTTP 接口和真实 Messenger 页面验收访客侧闭环。企业收件箱仍为空不属于本 PR 缺陷；界面中的“正在等待团队成员”只确认消息已经进入持久化服务批次。

### 18.1 迁移与模型

- 六张建表迁移可以在空库中顺序执行和逐步回滚。
- 每个迁移只创建一张表，不包含外键和 `CHECK`。
- 字段、默认值、中文说明、唯一索引和查询索引与本设计一致。
- `contacts.created_by_user_id` 可空，`source_channel_id` 保持非空。
- Bun 模型完整读写新增字段。

### 18.2 访客身份

- 首次初始化签发渠道级 Cookie 和响应 Token。
- Cookie 名称使用去掉连字符的渠道编号，不依赖尚不存在的渠道公开码。
- 初始化不创建联系人、渠道身份、聊天主体、会话、参与者、批次和消息。
- 相同 Cookie 刷新页面后恢复相同访客列表。
- Header Token 优先于 Cookie。
- 非法 Token 不读取其他访客数据。
- 发送和历史接口缺少或携带非法 Token 时返回 `400`，不创建记录，也不签发新身份后继续成功。
- 两个渠道使用不同 Cookie 名称和数据边界。

### 18.3 首条消息

- 第一条合法文本恰好创建一个联系人、渠道身份、ChatSubject、Conversation、CustomerConversation、Participant、ServiceSession 和 Message。
- 自动联系人 `created_by_user_id` 为空，`source_channel_id` 为当前网站渠道。
- 新服务批次 `sequence = 1`、`status = waiting`。
- 新消息同时更新长期会话和服务批次的最后消息字段。
- 数据库记录全部在同一事务提交。
- 任一步失败后不留下部分记录。

### 18.4 后续消息和服务批次

- 同一渠道身份的后续消息复用联系人、长期会话和参与者。
- 已退出的联系人参与者收到真实新消息时清空 `left_at` 并复用原行。
- 已归档的长期会话收到真实新消息时恢复为 `active`。
- `waiting` 和 `active` 批次接收后续消息而不创建新批次。
- `pending` 批次收到客户消息后改为 `active`。
- 最新批次 `closed` 后的新消息创建 `sequence + 1` 的批次。
- 同一长期会话同时只有一个未结束批次。
- 并发首条不同消息收敛到同一个长期会话和同一个未结束批次。
- 同一网站渠道的两个访客并发入站时不在共享渠道行上互相等待，各自只锁定自己的 `customer_conversations` 行。

### 18.5 幂等

- 相同 `clientMessageId` 重试返回相同联系人、会话、批次和消息编号。
- 幂等重试返回相同的 `reusedExistingServiceSession` 语义；开启消息始终为 `false`，已有批次中的后续消息始终为 `true`。
- 幂等重试不更新联系人、渠道身份最后活跃时间、批次状态和最后消息时间。
- 幂等比较忽略请求中的批次编号和本次接收时间。
- 相同幂等键对应不同正文、渠道身份或发送参与者时返回冲突。
- 幂等消息缺失必需参与者、主体、客户会话或渠道身份关系时返回内部错误，不作为成功结果返回。
- 并发提交相同消息只创建一条 Message。

### 18.6 路由

- 公共队列创建空团队和空负责人批次。
- 团队路由创建带团队编号的批次。
- 成员和 Agent 路由创建带企业身份编号及分配时间的批次。
- 初始目标不可用时使用失败路由。
- 两个目标都不可用时进入公共队列。
- 跨企业团队和企业身份不会写入服务批次。
- 已停用或已离开当前企业的成员、Agent 和团队不会写入新批次。

### 18.7 查询

- 初始化列表只返回当前渠道 Token 对应的服务批次。
- 列表按最后消息时间和批次编号稳定倒序排列。
- 列表同时包含关闭和未结束批次；页面只按 `waiting`、`active`、`pending` 筛选未结束批次，不把列表第一项当作未结束批次。
- 列表摘要来自服务批次的最后消息。
- 消息历史只返回指定服务批次中的消息。
- 相同 `originated_at` 的消息使用消息编号稳定分页。
- `before` 稳定读取更早消息，`after` 按正序读取更新，两者不重不漏。
- 空历史和对应方向已经到达边界时返回 `null` 游标。
- 其他 Token、渠道和企业的批次统一返回不存在。
- 公开路由注册在现有 `/api` Gin Service 下，未登录可访问，携带成员 Bearer Token 也不进入成员鉴权或扩大访问范围。

### 18.8 Messenger

- 不存在未结束批次时，点击“开始聊天”不创建服务端记录。
- 已存在未结束批次时，点击“开始聊天”直接打开该批次并显示完整历史。
- 空草稿返回后不出现在会话列表。
- 首条文本成功后，返回首页和消息页都显示真实列表项。
- 空草稿并发命中已有未结束批次时立即重读完整历史，不只显示刚发送的消息。
- 已经打开真实批次后的普通发送即使返回 `reusedExistingServiceSession = true`，也不整页重读历史。
- 在已结束批次发送后切换到新批次，旧批次保持关闭且不显示新消息。
- 刷新页面后仍显示相同列表项。
- 点击列表项后显示真实消息历史。
- 问候语只显示在空草稿的时间线外，不出现在历史接口或已有批次时间线。
- 刷新或从列表打开等待中的批次时恢复“正在等待团队成员”提示。
- 网络失败时正文仍在输入框中。
- 重试不会创建重复消息。
- 真实入口不再显示演示客服回复。
- 真实入口的附件和语音不会创建本地假消息。
- 管理端预览对 `/api/public` 零请求，并继续展示演示交互。
- 初始化失败可以重试，发送期间按钮不可重复提交。
- 独立链接和嵌入 Messenger 使用同一业务接口和数据。

## 19. 实施顺序

本 PR 按以下顺序实现：

1. 增加领域值、迁移和 Bun 模型。
2. 调整联系人创建人字段，并实现事务内 `EnsureChannelIdentity`。
3. 实现入站文本 Action 及事务集成测试。
4. 实现服务批次列表和带 `before`、`after` 的消息历史 Query。
5. 实现未注册为 Wails 绑定的访客应用服务。
6. 在现有 `/api` Gin Service 注册公开路由，实现 Token 解析、签发和 HTTP 测试。
7. 改造 `chat.js` 和 `page.html`，接入初始化、发送、列表和历史。
8. 保留预览分支，关闭真实入口中的演示回复、附件和语音假消息。

每一步保持当前代码可编译。数据库业务不进入 `internal/publicweb`，匿名访客接口不进入桌面端和移动端 Wails 绑定。
