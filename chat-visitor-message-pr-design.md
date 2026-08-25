# 网站访客真实文本消息与会话列表 PR 设计

## 1. PR 定位

本 PR 基于 [chat-first-pr-design.md](chat-first-pr-design.md) 已建立的聊天数据底座，把网站访客 Messenger 从内存演示界面接到真实 PostgreSQL 数据。

没有包含未结束 ServiceSession 的 Conversation 时，访客点击“开始聊天”只进入本地草稿页面，不创建数据库记录。访客第一次提交合法文本后，服务端在同一事务中创建或取得联系人和渠道身份，并创建客户可见 Conversation、参与者、首个 ServiceSession 和 Message。访客返回首页或消息页后，会话列表从数据库读取；页面刷新后能够恢复列表和完整 Conversation 消息历史。

本 PR 采用首版单处理线程策略：同一网站渠道身份同时最多一个 `status IN (waiting, active, pending)` 的 ServiceSession，因此最多一条正在处理的 Conversation。`conversations.status = active` 不表示客服处理尚未结束。数据结构和公开契约已经以 `conversationId` 为稳定客户线程编号，后续允许并行线程时不需要替换列表主键和历史路径。

本 PR 只完成访客侧可独立验收的持久化闭环。企业成员收件箱读取、人工回复和 Agent 回复继续单独交付。当前界面的“正在等待团队成员”只表示消息已经进入持久化 ServiceSession，不表示成员端已经具备处理入口。

PR 标题使用：

```text
feat: 接入网站访客真实文本消息
```

## 2. 依赖与当前实现

### 2.1 前置依赖

本 PR 必须在数据底座 PR 合并后开发，直接使用以下表和模型：

```text
chat_subjects
conversations
customer_conversations
conversation_participants
service_sessions
messages
```

本 PR 不重复创建或重定义这些迁移，也不得修改 PR1 或更早的迁移文件。若 PR1 尚未合并，应先在 PR1 的新增迁移中修正目标结构并重新审核；若 PR1 已经合并或迁移已被环境应用，则只能在 PR2 中增加新的向前迁移，不得改写迁移历史。任何新增迁移都要在同一文件中为新增或修改的表、每一列、显式索引和具名约束写全简洁中文数据库注释，不增加双版本兼容字段。

### 2.2 当前 Messenger

网站访客 Messenger 位于 `internal/publicweb`。独立链接使用 `/chat/{channelID}`，嵌入页面使用 `/embed/widget/{channelID}`；两个入口共用 `page.html`、`chat.js` 和 `chrome.css`。

当前行为包括：

- `ChatService` 和 `EmbedService` 只接受 `GET` 和 `HEAD`。
- 点击“开始聊天”只创建 JavaScript 内存对象。
- 提交文本后立即在浏览器中拼接访客消息。
- 约一秒后自动拼接演示客服回复。
- 首页最近会话和消息页列表只读取内存状态。
- 页面刷新后联系人、会话、批次和消息全部丢失。
- 附件、语音、未读和客服在线状态都是演示。

本 PR 删除真实入口中的本地会话事实和演示客服回复。管理端 Messenger 预览继续使用本地演示，不访问数据库。

## 3. 产品对象与生命周期

### 3.1 客户可见线程

访客页面的一条“会话”始终对应一条 `Conversation`：

```text
ContactChannelIdentity
  └── CustomerConversation
        └── Conversation             客户可见线程
              ├── ServiceSession 1   一次客服处理周期
              ├── ServiceSession 2   后续再次处理
              └── Message *          完整连续历史
```

访客列表、页面内存键、公开历史路径和轮询游标都使用 `conversationId`。公开 DTO 可以携带最新 ServiceSession 摘要，但不能用 `serviceSessionId` 代替聊天线程编号。

同一 Conversation 上的多个 ServiceSession 不切断历史。访客打开一条 Conversation 时读取该线程全部有权消息，而不是只读取当前或最近批次的消息。

### 3.2 首版单处理线程

本文所称“正在处理的 Conversation”固定指包含 `waiting`、`active` 或 `pending` ServiceSession 的客户线程；“已关闭 Conversation”固定指最新 ServiceSession 为 `closed` 的客户线程，不修改 `conversations.status` 的领域含义。

首版交互规则：

| 访客动作 | 服务端结果 |
| --- | --- |
| 点击“开始聊天”，已有正在处理的 Conversation | 直接打开该 Conversation |
| 点击“开始聊天”，没有正在处理的 Conversation | 创建本地空草稿，不写数据库 |
| 空草稿首次发送，没有并发创建 | 新建 Conversation、首个 ServiceSession 和 Message |
| 空草稿首次发送时另一页面已经创建未结束线程 | 收敛到已有 Conversation 和 ServiceSession，并写入本次 Message |
| 在正在处理的 Conversation 中发送 | 复用当前 ServiceSession；`pending` 时恢复为 `active` |
| 打开最新 ServiceSession 已关闭的 Conversation 发送，且身份没有其他正在处理的线程 | 保持原 Conversation，新建下一个 ServiceSession |
| 打开最新 ServiceSession 已关闭的 Conversation 发送，但身份已有另一条正在处理的线程 | 返回冲突，不静默把消息写到另一线程 |
| 所有线程都已关闭后重新点击“开始聊天” | 新建本地草稿，首条消息创建新的 Conversation |

本 PR 不提供“已有未结束线程时仍发起新会话”的按钮。以后开放并行入站时，删除身份级未结束 ServiceSession 唯一约束并调整入口策略；Conversation 主键、消息历史和公开路径保持不变。

### 3.3 ServiceSession 状态

状态流转固定为：

```text
waiting ──接待或首次回复──> active
active  ──等待客户────────> pending
pending ──客户发送新消息──> active
waiting ──结束────────────> closed
active  ──结束────────────> closed
pending ──结束────────────> closed
closed  ──同一线程再次发送──> 新建下一 ServiceSession
```

本 PR 只实现入站消息触发的行为：

- 首条消息创建 `waiting` ServiceSession。
- `waiting` 和 `active` 接收后续消息。
- `pending` 收到客户消息后改为 `active`。
- 最新 ServiceSession 已关闭时，在同一 Conversation 上按 `MAX(sequence) + 1` 创建新的 ServiceSession。

接待、回复、挂起和结束命令不进入本 PR。

## 4. PR 范围

本 PR 完成以下内容：

- 签发和恢复网站访客 Token。
- 提供 Messenger 初始化接口。
- 提供网站访客文本发送接口。
- 提供访客 Conversation 列表和完整消息历史查询。
- 实现事务内联系人渠道身份确保能力。
- 在首条真实消息事务中创建 Conversation、ServiceSession 和 Message。
- 按网站渠道初始路由和失败路由填充团队或负责人。
- 把独立聊天链接和嵌入 Messenger 接到真实接口。
- 从数据库恢复当前显示的会话列表和消息历史。
- 保留管理端 Messenger 预览的本地演示能力。

本 PR 不完成以下内容：

- 企业成员收件箱列表、历史和回复。
- ServiceSession 领取、转接、挂起和结束接口。
- 同一渠道身份同时拥有多个未结束 ServiceSession。
- 客服协作者关系和转接历史。
- 未读数和已读回执。
- WebSocket、SSE、通知和后台轮询。
- 附件、图片、视频、语音和文件上传。
- 消息引用、编辑、删除、反应和系统消息写入。
- 联系人合并和跨渠道身份关联。
- 演示客服回复写入数据库。

## 5. 网站访客身份

### 5.1 Token

网站访客身份使用每个渠道独立的随机 Token。Token 使用 32 位小写字母和数字，不包含联系人编号、Conversation 编号、时间和浏览器信息。

Cookie 名称：

```text
cervi_visitor_<去掉连字符的 channel_id>
```

Cookie 属性：

```text
Path=/
HttpOnly
SameSite=None; Secure            所有 HTTPS 请求
SameSite=Lax                     所有 HTTP 请求，不设置 Secure
不设置 Domain
```

公开初始化 Handler 无法稳定判断同一个 `/api` 请求来自独立页还是 iframe，因此 Cookie 策略只根据外部请求协议决定，不根据入口类型或 `Sec-Fetch-*` 猜测。反向代理后的 HTTPS 判断必须使用项目可信代理配置提供的外部协议，不能只检查 `Request.TLS`。Cookie 不设置 `Domain`。HTTP 嵌入页无法设置可用的跨站 `SameSite=None` Cookie，这种环境只依靠初始化响应中的 Header Token 维持当前页面，刷新后不保证恢复原身份。

初始化接口只在优先来源 Token 缺失或格式非法时签发新 Token，并通过响应 Cookie 和 JSON 返回。仅新签发 Token 时发送 `Set-Cookie` 并设置 `Max-Age=31536000`；已有合法 Header 或 Cookie 时不发送 `Set-Cookie`，不滑动续期。Header 与 Cookie 都存在但值不同时 Header 优先完成本次请求，不写 Cookie，也不修改原 Cookie 过期时间。

JavaScript 只在当前页面内存保存响应 Token，后续请求通过 `X-Cervi-Visitor-Token` 发送，不写入 `localStorage`、`sessionStorage` 或 URL。

服务端按 Header、Cookie 顺序读取 Token。只有初始化接口可以签发新 Token；发送消息和读取历史缺少有效 Token 时返回 `400`，不以新身份继续读写。成员 Bearer Token 不参与公开 Messenger 身份判断。

### 5.2 数据创建时机

以下动作不创建联系人或聊天记录：

- 打开独立聊天页或嵌入 Messenger。
- 初始化访客 Token。
- 点击“开始聊天”。
- 输入或放弃草稿。

第一条合法文本提交才在一个事务中创建业务记录。渠道身份外部编号使用：

```text
contact_channel_identities.external_id = web-session:<visitor_token>
```

公开 HTTP 适配器把裸 Token 规范化为 `external_id` 后再调用应用服务。Action 和 Query 只接收 `external_id`，不解析 Cookie 或 Header。

## 6. 应用分层

### 6.1 `internal/publicweb`

`internal/publicweb` 继续负责：

- 输出独立 Messenger 和嵌入 Messenger HTML。
- 输出 `widget.js`、`chat.js` 和样式。
- 校验嵌入宿主网站。
- 保留管理端预览本地演示。

它不新增聊天存储访问，也不直接创建联系人、Conversation、ServiceSession 或 Message；现有公开渠道配置 Lookup 保持只读边界。

### 6.2 公开 HTTP 适配器

公开路由注册在现有 `internal/api.Service` 的 Gin Router：

```text
/public/website-channels/:channelID/messenger
/public/website-channels/:channelID/messages
/public/website-channels/:channelID/conversations/:conversationID/messages
```

现有 Wails 服务继续只挂载一次 `/api`，对外路径为 `/api/public/...`。不新增单独挂载到 `/api/public` 的 Wails Service，避免前缀注册顺序截获请求。

公开 Handler 不调用成员认证，不要求也不解释 Bearer Token。它负责：

- 读取渠道和路径参数。
- 读取、签发和设置访客 Cookie。
- 读取 `X-Cervi-Visitor-Token`。
- 把裸 Token 规范化为网站 `external_id`。
- 绑定和限制 JSON 请求体。
- 写入 HTTP 状态、结构化错误和 `Cache-Control: no-store`。
- 按 `Accept-Language` 本地化并写入 `Content-Language` 和 `Vary`。
- 调用访客应用服务。

`api.NewService` 通过独立依赖接收 `WebsiteVisitorService` 并把公开 Handler 注册到现有 Gin Router。不要把访客方法加入成员 `appservice.Service` 或 `Backend`，也不要新增 Wails `Route: "/api/public"`。

### 6.3 访客应用服务

`internal/appservice` 增加不注册为 Wails 绑定的 `WebsiteVisitorService`，定义公开 Messenger 使用的唯一 DTO，并转发给独立 `WebsiteVisitorBackend`。

服务端使用 `WebsiteVisitorDirectBackend` 调用访客 Action 和 Query，并把语言无关错误映射为本地化 `appservice.Error`。它不实现成员 `Backend`，也不把匿名方法加入需要成员认证的接口。

服务端构造路径在 `application_services_server.go` 的现有 DirectBackend 旁创建 WebsiteVisitorDirectBackend 和 WebsiteVisitorService，再注入现有 `/api` Gin Service；桌面端和移动端不构造该服务。

调用路径：

```text
chat.js
  → 现有 /api Gin Service 中的公开 Handler
  → WebsiteVisitorService
  → WebsiteVisitorDirectBackend
  → conversation Action / Query
  → PostgreSQL
```

## 7. 公开 HTTP 接口

### 7.1 初始化 Messenger

```http
GET /api/public/website-channels/{channelID}/messenger
X-Cervi-Visitor-Token: <token，可选>
```

响应：

```json
{
  "visitorToken": "32位访客Token",
  "conversations": [
    {
      "id": "conversation-id",
      "title": "登录后无法同步数据",
      "preview": "最后一条消息正文",
      "lastMessageAt": "2026-08-25T00:00:00Z",
      "serviceSession": {
        "id": "service-session-id",
        "status": "waiting"
      }
    }
  ]
}
```

列表来自当前渠道身份的 `customer_conversations`，按 Conversation 的 `(last_message_at DESC, id DESC)` 返回最近 20 条。首版不提供更早 Conversation 的翻页入口，超过 20 条的旧线程暂不显示。

每条已提交的网站客户 Conversation 必须有一个 ServiceSession；缺失时返回内部错误，不能省略 `serviceSession`。摘要固定返回该 Conversation 中 `sequence` 最大的 ServiceSession，不能按 `last_message_at` 猜测。

`title` 在首条文本事务中从入库正文派生：只为标题把连续空白折叠为单个空格，并按 Unicode 码点截取前 60 个字符。消息 `body` 只去除首尾空白，不折叠内部空白。标题创建后不因收敛到已有线程、开启新 ServiceSession、幂等命中或后续消息改变。

`preview` 完整返回 `conversations.last_message_id` 对应的未删除消息正文，最长可达 4000 个 Unicode 字符，由页面负责单行截断；`lastMessageAt` 必须来自同一摘要。

列表同时包含最新 ServiceSession 已关闭和未结束的 Conversation。页面寻找当前正在处理的线程时按 `serviceSession.status IN (waiting, active, pending)` 筛选；身份级部分唯一索引保证首版至多一条。

没有业务记录时返回空数组，不创建数据库记录。Token 仍只按第 5.1 节处理：仅优先来源缺失或非法时由 HTTP 适配器新签发。

Token 签发只发生在公开 HTTP 适配器。初始化 Query 只使用已经规范化的 `external_id` 查询现有数据，不调用 `EnsureChannelIdentity`，也不创建联系人。

### 7.2 发送文本消息

```http
POST /api/public/website-channels/{channelID}/messages
Content-Type: application/json
X-Cervi-Visitor-Token: <token>
```

请求：

```json
{
  "clientMessageId": "0198ddee-c056-7bc5-a1d9-586f878ee966",
  "conversationId": null,
  "body": "你好，我想了解产品。"
}
```

`conversationId` 语义：

- DTO 使用 `*string`；JSON 缺省或 `null` 都表示本地新草稿，希望创建新 Conversation。
- 空字符串 `""` 非法，返回 `400`。
- 回复已有线程时填写当前 Conversation 编号。
- 编号必须属于当前渠道 Token 对应的身份。
- 不接受 `serviceSessionId` 作为客户线程选择参数。

`clientMessageId` 和非空 `conversationId` 都使用 `common.ValidUUID` 校验为项目接受的规范 UUID。

成功响应：

```json
{
  "conversation": {
    "id": "conversation-id",
    "title": "你好，我想了解产品。",
    "preview": "你好，我想了解产品。",
    "lastMessageAt": "2026-08-25T00:00:00Z",
    "serviceSession": {
      "id": "service-session-id",
      "status": "waiting"
    }
  },
  "createdConversation": true,
  "openedNewServiceSession": true,
  "message": {
    "id": "message-id",
    "author": "visitor",
    "body": "你好，我想了解产品。",
    "originatedAt": "2026-08-25T00:00:00Z",
    "createdAt": "2026-08-25T00:00:00Z"
  }
}
```

两个响应标志只从持久事实计算，事务正常路径和幂等命中必须使用相同谓词，禁止保存或返回本次控制流中的 `created` 布尔：

```text
openedNewServiceSession
  <=> service_session.opening_message_id = message.id

createdConversation
  <=> openedNewServiceSession
      AND service_session.sequence = 1
```

因此首次创建 Conversation 时两个值都为 `true`；在同一 Conversation 上开启后续 ServiceSession 时只有 `openedNewServiceSession = true`；未结束批次中的后续消息两个值都为 `false`。

如果请求指定最新 ServiceSession 已关闭的 Conversation，而同一身份已有另一条正在处理的 Conversation，返回 `409`：

```json
{
  "error": {
    "kind": "conflict",
    "message": "请先继续当前会话。",
    "fields": {
      "reason": "open_session_exists"
    }
  }
}
```

收到 `reason = open_session_exists` 后页面重新调用初始化接口并打开当前正在处理的 Conversation。切换目标后必须为保留的正文生成新 `clientMessageId`，不能拿原编号向另一线程重试。服务端不能把原正文静默写入另一 Conversation，也不在错误体中增加另一套 Conversation DTO。

服务端忽略客户端时间。`originated_at` 使用服务器首次收到该 `clientMessageId` 的时间，并在完整事务重试之间保持一致。

客户端失败时保留正文和 `clientMessageId`。原样重试继续使用相同编号；用户修改正文或切换目标 Conversation 后生成新的 `clientMessageId`。

### 7.3 读取 Conversation 消息

```http
GET /api/public/website-channels/{channelID}/conversations/{conversationID}/messages[?before=<cursor>|after=<cursor>]
X-Cervi-Visitor-Token: <token>
```

接口返回该 Conversation 的完整消息历史，不按 ServiceSession 切段。访问权通过 Token、渠道身份和 `customer_conversations` 完整校验，不能凭 `conversationID` 单独读取。

`before` 和 `after` 不能同时提供。无游标读取最近 50 条；`before` 读取更早消息；`after` 按正序读取游标之后的新消息。当前页面只调用无游标读取最近 50 条；`before` 和 `after` 作为稳定契约保留，本 PR 页面不调用，也不增加翻页或轮询控件。

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

分页使用 `(originated_at, id)` 稳定边界。无游标和 `before` 在数据库中按倒序扫描最近或更早记录后反转，`after` 直接正序扫描；三种响应的 `messages` 数组都按正序返回。游标由服务端编码方向、Conversation 编号和元组，客户端只把它作为不透明字符串回传。游标被篡改、属于其他 Conversation，或同时提交 `before` 和 `after` 时返回 `400`，不通过 `404` 暴露关系。

无游标查询存在更早记录时返回 `before`，并用最新消息生成 `after`。`before` 查询只返回下一段 `before`；`after` 查询只返回下一次 `after`。空结果在对应方向返回 `null`，客户端增量空结果时继续保留自己已有的旧游标。

### 7.4 HTTP 错误

| 状态 | 含义 |
| --- | --- |
| `400` | Token、正文、客户端消息编号、Conversation 编号或游标非法 |
| `404` | 渠道停用、不存在，或 Conversation 不属于当前访客 |
| `409` | 幂等冲突，或首版单处理线程规则冲突 |
| `405` | 公开路由上的请求方法不允许 |
| `413` | 请求体超过限制 |
| `500` | 数据库或服务端内部失败 |

错误复用现有 `appservice.Error` 结构。`ErrorKind` 增加 `conflict` 并在 `HTTPStatus()` 映射 `409`；访客错误不携带成员会话使用的 `state = setup/login/connect`。

两类冲突使用语言无关的 `fields.reason` 区分：

```text
open_session_exists    目标线程已关闭，但该身份已有另一条正在处理的线程
idempotency_mismatch   相同幂等键对应不同身份、参与者、正文或非空目标线程
```

`fields.reason` 的值固定为上述稳定码，不经过 i18n，也不进入现有字段校验使用的 FieldKey/`LocalizeMap` 映射；只有 `message` 使用请求语言本地化。WebsiteVisitorDirectBackend 必须直接保留稳定 reason。

页面只对 `open_session_exists` 重新初始化和切换线程；`idempotency_mismatch` 保留编号并显示不可原样提交的失败提示，不自动跳转。错误体不返回 SQL、约束名、访客 Token、外部身份编号或内部关联编号。

`405` 只由公开路由组显式处理，不开启 Gin 全局 `HandleMethodNotAllowed`，避免改变现有成员 `/api` 接口的错误行为。

## 8. 联系人渠道身份确保能力

`internal/actions/contact` 增加事务内 `EnsureChannelIdentity`。签名必须显式接收调用方提供的 `bun.IDB` 或等价事务接口，例如 `EnsureChannelIdentity(ctx, tx, ...)`；它不得自行调用 `RunInTx`、提交或回滚。`ReceiveWebsiteCustomerTextMessage` 是联系人、渠道身份和全部聊天事实的唯一事务提交点。

该能力接收已经规范化的 `external_id`，不感知 Cookie、Header 或网站 Token，并集中完成：

- 校验渠道、企业和类型一致。
- 按 `(channel_id, external_id)` 查找联系人渠道身份。
- 不存在时创建 `stage = visitor` 联系人和渠道身份。
- 自动联系人 `created_by_user_id = NULL`。
- 自动联系人 `source_channel_id = 当前网站渠道`。
- 联系人显示名称保持为空，不伪造“匿名访客”。
- 已存在联系人处于回收站时，直接在调用方事务中把 `deleted_at` 清空，不转调会自行开事务的成员 `RestoreContactAction`。
- 已存在联系人未删除时，不修改其 `stage`、`display_name`、`source_channel_id` 或创建用户。

该能力只维护联系人和渠道身份，不创建 ChatSubject、Conversation、ServiceSession 或 Message，后续其他渠道可以复用相同的身份维护规则。

## 9. 入站文本事务

### 9.1 输入预处理

`ReceiveWebsiteCustomerTextMessage` 接收渠道编号、规范化 `external_id`、可空 Conversation 编号、客户端消息编号和正文。

进入事务重试前：

- 去除正文首尾空白。
- 校验正文非空且不超过 4000 个 Unicode 字符。
- 校验客户端消息编号是规范 UUID 字符串。
- 校验 `external_id` 符合网站身份格式和长度限制。
- 生成首次服务器接收时间。
- 预生成本次可能需要的 `contact_id`、`contact_channel_identity_id`、`chat_subject_id`、`conversation_id`、`participant_id`、`service_session_id` 和 `message_id` UUIDv7；`customer_conversations` 使用 Conversation 编号作为主键。
- 生成标题时，在已经 trim 的正文上把连续空白折叠为单个空格，再按 Unicode 码点截取前 60 个字符；入库 `body` 仍只去除首尾空白，不折叠内部空白。

Action 最多执行三次完整事务尝试。服务器接收时间、幂等键和全部预生成编号在重试之间保持不变；竞态收敛到已有行时丢弃未使用的预生成编号，不能插入孤立记录。

### 9.2 事务步骤

每次事务执行：

1. 读取已启用的网站渠道，取得企业、初始路由和失败路由，不锁定渠道配置行。
2. 按企业和 `chmsg:<channel_id>:<client_message_id>` 查询已有 Message 及完整关联。
3. 幂等记录一致时直接返回保存结果，不修改业务行。
4. 调用 `EnsureChannelIdentity` 查找或创建联系人渠道身份。
5. 使用 `FOR UPDATE` 锁定当前 `contact_channel_identities` 行，作为同一访客创建或恢复未结束线程的串行化点；不同访客不互相阻塞。
6. 确保联系人拥有唯一 `kind = contact` ChatSubject。
7. 查询该渠道身份当前 `waiting/active/pending` ServiceSession 及其 Conversation；数据库保证至多一条。
8. 请求携带 `conversationId` 时，校验它通过 `customer_conversations` 属于当前渠道身份、企业和渠道。
9. 请求未携带 Conversation 编号且存在未结束 ServiceSession 时，选择该 ServiceSession 及其 Conversation。
10. 请求未携带编号且不存在未结束线程时，创建新的 `type = customer`、`status = active` Conversation 和 CustomerConversation；客户入站的 `created_by_subject_id` 留空，标题使用首条正文派生值，并标记需要以 `nextSequence = 1` 新建首个 ServiceSession。
11. 请求携带 Conversation 编号且该线程有 `waiting/active` ServiceSession 时复用；访客后续消息不得把 `waiting` 自动改为 `active`。
12. 请求携带 Conversation 编号且该线程有 `pending` ServiceSession 时复用并改为 `active`，更新 `status_changed_at`。
13. 请求携带 Conversation 编号、该线程最新 ServiceSession 为 `closed` 且身份不存在其他未结束 ServiceSession 时，在已经锁定身份行的前提下计算 `nextSequence = MAX(sequence) + 1`，并标记需要新建 `waiting` ServiceSession；禁止使用 `COUNT(*) + 1`，本步不插入行。
14. 请求指定的 Conversation 最新 ServiceSession 为 `closed`，但身份存在另一条 `waiting/active/pending` ServiceSession 时，返回 `open_session_exists` 冲突，不写消息。
15. 仅当请求携带 Conversation 编号、该客户线程已经通过归属校验、但一行 ServiceSession 都没有时返回内部错误，不写消息，不能按已关闭线程重新开启。
16. 已选定目标 Conversation 且本事务确定继续写入时，如果 `conversations.status = archived`，在本事务恢复为 `active`；访客列表和处理状态仍以最新 ServiceSession 为准。尚未确定写入或已经返回 `open_session_exists` 时不得修改状态。
17. 确保联系人 ChatSubject 是目标 Conversation 的有效 `member` 参与者；已有行 `left_at` 非空时清空并复用。
18. 需要新 ServiceSession 时解析渠道初始路由和失败路由，只生成路由快照，不在本步写入 ServiceSession。
19. 需要新 ServiceSession 时执行唯一一次 INSERT，`opening_message_id = last_message_id = 预生成 message_id`，`last_message_at = originated_at`，同时写入渠道身份、`nextSequence` 和路由快照。
20. 创建 `type = text` Message，写入目标 Conversation、当前 ServiceSession 和联系人参与者；`created_at`、`updated_at` 均使用插入时的默认值。
21. 复用已有未结束 ServiceSession 时，仅在新 Message 的 `(originated_at, id)` 严格大于当前摘要时，同时更新 `last_message_id`、`last_message_at` 和 `updated_at`；消息必须属于该 Session，已经关闭的 Session 禁止回写。
22. 按相同 `(originated_at, id)` 规则同时更新 Conversation 的 `last_message_id`、`last_message_at` 和 `updated_at`；当前摘要为空时允许更新。
23. 更新渠道身份 `last_seen_at`。
24. 从保存的 Message 和 ServiceSession 持久事实计算两个响应谓词，提交并返回 Conversation 摘要、ServiceSession 摘要和 Message。

新 Conversation、ServiceSession 和 Message 存在互相引用。无外键时的写入顺序固定为：

```text
Conversation
→ CustomerConversation
→ Participant
→ ServiceSession（带预生成消息编号和路由快照）
→ Message
→ 按比较规则更新 Conversation 摘要
→ contact_channel_identities.last_seen_at
```

事务锁顺序固定为渠道身份行在前，必要时再更新 Contact 或 Conversation；禁止先锁 Conversation 再锁渠道身份。任一步失败时由唯一的调用方事务整体回滚。

### 9.3 路由快照

新 ServiceSession 使用渠道路由：

| 路由结果 | `team_id` | `assignee_identity_id` |
| --- | --- | --- |
| 公共队列 | 空 | 空 |
| 团队队列 | 团队编号 | 空 |
| 指定成员或 Agent | 空 | 企业身份编号 |

写入前重新校验团队、成员或 Agent 属于当前企业并处于可用状态。初始目标不可用时使用失败路由；失败路由也不可用时进入公共队列。该降级属于服务端路由解析，不把管理表单使用的 `ValidationError` 透出公开接口。直接路由到成员或 Agent 时同时写入 `assigned_at`。

渠道路由在事务期间改变不修改已经创建的 ServiceSession。后续新 Conversation 或已关闭线程的新 ServiceSession 读取届时配置。

### 9.4 幂等

网站消息幂等键：

```text
chmsg:<channel_id>:<client_message_id>
```

该键必须作为非空字符串写入 `messages.idempotency_key`，禁止用空字符串表达未设置。

等价比较包含：

- 同一渠道身份。
- 同一发送参与者。
- 同一规范化正文。
- 请求指定非空 Conversation 时，该编号必须等于已保存 Message 的 Conversation。

请求 `conversationId = null` 时，幂等重试直接返回首次实际创建或并发收敛到的 Conversation；空编号不要求重新执行“新建”选择。客户端收到成功结果后即使用返回的 Conversation 编号，切换到其他目标线程或修改正文时必须生成新的客户端消息编号。

只有非幂等写入路径恢复回收站联系人。幂等命中不恢复联系人、不更新渠道身份最后活跃时间、不改变 Session 状态、不更新标题、最后消息和路由快照。

命中时沿两条关系核对完整性：

```text
Message → Participant → ChatSubject → Contact
Message → ServiceSession → CustomerConversation → ContactChannelIdentity
```

Conversation、ServiceSession、渠道身份、参与者和企业任一关系缺失或矛盾时返回内部错误，不能把残缺记录视为成功。

等价比较不一致时立即返回 `409 reason = idempotency_mismatch`。请求指定最新 Session 已关闭的 Conversation、但身份级已有另一条未结束 Session 时返回 `409 reason = open_session_exists`；请求 `conversationId = null` 时仍按步骤 9 收敛到现有线程，不返回该冲突。两类业务冲突都不进入数据库唯一冲突重试。

以下命名唯一约束属于预期并发竞态，命中后整笔事务回滚并从第一步重试：

```text
contact_channel_identities_channel_external_unique
chat_subjects_org_kind_source_unique
conversation_participants_org_conversation_subject_unique
service_sessions_org_conversation_open_unique
service_sessions_org_channel_identity_open_unique
service_sessions_org_conversation_sequence_unique
messages_organization_idempotency_unique
```

`customer_conversations` 不再有渠道身份唯一约束，不把有意创建不同 Conversation 当成冲突。

只捕获上述约束名对应的 PostgreSQL `23505`。其他唯一冲突视为实现错误直接返回内部失败；三次完整事务尝试仍持续命中预期唯一竞态时返回 `500`，不能伪装成业务 `409`。

现有渠道身份唯一索引只覆盖 `(channel_id, external_id)`。`EnsureChannelIdentity` 命中已有行后仍必须校验它的 `organization_id` 与渠道企业一致，不能因为唯一键命中而跳过企业边界。

## 10. 查询规则

### 10.1 Conversation 列表

列表关系：

```text
channel_id + external_id
  → contact_channel_identities
  → customer_conversations
  → conversations
  → conversations.last_message_id
  → messages
  → 每条 Conversation 的最新 service_sessions.sequence
```

查询只返回当前网站渠道身份的 Conversation。不同 Token、渠道、联系人和企业的数据不会进入结果。

列表包含 `conversations.status = active/archived` 的有权线程，按 `(conversations.last_message_at DESC, conversations.id DESC)` 排序并返回最近 20 条。预览使用 `last_message_id`，不能按 `customer_conversations.created_at`、ServiceSession 摘要或最大消息 `created_at` 临时猜测。消息删除落地后再实现跳过已删除消息的回退逻辑。

### 10.2 Conversation 消息历史

历史查询同时使用：

```text
organization_id  = 当前渠道企业
conversation_id  = 当前列表项
customer_conversation.contact_channel_identity_id = 当前渠道身份
```

不增加 `service_session_id` 过滤。ServiceSession 变化以后可以作为系统事件展示，但不切割客户线程。

查询读取该 Conversation 的全部 Message，不按发送者类型过滤，否则后续成员回复会破坏游标连续性。发送者通过 `Message → Participant → ChatSubject` 解析；本 PR 合法数据只有联系人消息并映射为 `author = visitor`，遇到当前 DTO 尚不支持的主体时返回内部错误，不能静默丢弃。企业成员或 Agent 回复落地时同步增加访客端展示映射。

网站问候语是界面配置，不是持久消息，不进入历史接口。

## 11. Messenger 页面改造

### 11.1 初始化

真实独立链接和嵌入 Messenger 加载后立即请求初始化接口。独立链接从当前路径读取渠道编号，嵌入页面从服务端输出的 `data-channel-id` 读取。`page.html` 的真实页和预览页都显式输出该属性，但 `data-preview="true"` 分支禁止使用它请求公开 API。

所有请求使用 `credentials: "same-origin"`。初始化完成前禁用“开始聊天”和发送；初始化期间保留现有页面框架并显示加载状态，失败时只显示页内重试，不创建本地草稿或进入空白成功状态。

成功后在内存保存 Token，并用返回的真实 Conversation 渲染首页最近会话和消息页列表。

管理端预览通过 `data-preview="true"` 识别，不请求初始化接口、不设置访客 Cookie、不创建数据库记录。

### 11.2 新建草稿

点击“开始聊天”时：

1. 从初始化列表筛选最新 ServiceSession 为 `waiting/active/pending` 的 Conversation。
2. 存在时直接打开该 Conversation 并读取完整历史。
3. 不存在时创建本地草稿并进入聊天页。

草稿没有服务器编号，不进入首页和会话列表；空草稿返回后直接丢弃。

只有本地空草稿显示 `cv-conversation-intro` 问候语。问候语位于时间线外，不追加成 assistant 气泡。真实入口不能调用当前会追加问候气泡的 `startConversation()` 路径；打开任何真实 Conversation 都只显示持久历史，预览继续保留原演示路径。

首条文本成功后，响应中的 `conversation.id` 写入当前草稿，该对象变为真实会话。列表按该编号 upsert：已有项更新 `title`、`preview`、`lastMessageAt` 和 `serviceSession`，没有才追加；随后按 `(lastMessageAt DESC, id DESC)` 重排，禁止无条件插入首位产生重复项。

空草稿发送成功后，只要本地还没有该 `conversation.id` 的完整历史，就必须重读该 Conversation 再合并当前响应，不能用 `createdConversation` 作为是否重读的唯一判断。

### 11.3 打开已有 Conversation

点击列表项后按 Conversation 编号读取最近消息。重复打开已经加载的线程时复用内存消息，页面刷新后重新读取。

页面首版只渲染最近 50 条，不实现向上加载更早历史的界面。`before` 保留为稳定公开契约，`after` 留给后续客服回复轮询；两者不在本 PR 增加后台轮询或额外控件。

已关闭 Conversation 仍展示完整历史。身份没有其他未结束线程时允许访客从该页面发送，新消息在同一 Conversation 上创建新的 ServiceSession；如果已经存在另一条未结束线程，输入区提示继续当前会话并提供跳转，不发送到错误线程。

### 11.4 发送文本

真实入口遵循：

- 提交前生成 `clientMessageId`。
- 请求进行中禁用发送按钮。
- 请求成功后才显示持久消息并清空输入框。
- 请求失败时保留正文和客户端消息编号。
- 原样重试复用编号。
- 修改正文或切换 Conversation 后生成新编号。
- 服务端响应决定消息时间、编号、Conversation 和 ServiceSession。
- 不用乐观消息制造持久成功假象。
- 失败使用简短页内文案，不生成字段错误或 Toast。

真实入口删除 `scheduleDemoReply`。发送成功后，最新 ServiceSession 为 `waiting` 且最后消息来自访客时显示“正在等待团队成员”。刷新或从列表打开时根据真实摘要和历史恢复该提示。

管理端预览继续使用本地访客消息和演示客服回复。

### 11.5 会话列表

消息页把静态单行结构改为可重复渲染容器。每个列表项以 `conversation.id` 为键，显示：

- Conversation 标题。
- 最后一条消息正文。
- 本地化最后消息时间。
- 最新 ServiceSession 状态需要的简短展示。

首页最近会话使用列表第一项；没有真实 Conversation 时隐藏最近会话卡片并显示空状态。

返回首页或消息页时使用当前内存中的服务端响应重新渲染；页面刷新后通过初始化接口恢复相同数据库列表。

本 PR 没有客服回复和已读事实，所有未读圆点保持隐藏。真实入口固定发送 `cervi:unread = false` 或不再发送 `true`，不能沿用演示回复制造的未读状态。

### 11.6 其他控件

真实入口只接入文本和表情，表情作为文本内容提交。

附件和语音按钮在真实入口中禁用并显示不可用状态，不创建本地假消息。发送、粘贴图片、附件和语音处理都必须按 `data-preview` 分支，不能只删除 `scheduleDemoReply`。管理端预览继续展示附件和语音演示；文件能力在后续 PR 接入现有临时上传流程。

## 12. 访问控制与请求约束

公开请求不使用企业成员 Bearer Token。访客 Token 只授权当前渠道身份下的客户 Conversation。

发送和历史请求缺少 Token 或优先来源 Token 格式非法时返回 `400`；渠道停用或不存在返回 `404`。公开 Handler 忽略 `Authorization`，不得调用成员 `authenticate()` 或复用会读取 Bearer Token 的成员 RequestMeta。HTTP 适配器生成 `web-session:<32 位小写字母数字>`，Action 再校验前缀和总长度；裸 Token 不进入 Action。

每次读写显式校验：

- 渠道存在、启用且类型为 `website`。
- 渠道身份属于当前渠道和企业。
- 联系人属于相同企业。
- ChatSubject 属于相同企业和联系人。
- CustomerConversation 属于相同企业和渠道身份。
- ServiceSession 的 Conversation、渠道身份和企业一致。
- Message、Participant 和 Conversation 属于相同企业。

公开 JSON 接口不启用跨域。嵌入 Messenger iframe 与接口同源请求 `/api/public`。管理端预览不调用真实接口。公开接口的成功和失败响应都写入 `Cache-Control: no-store`。

请求体上限为 16 KiB；正文最多 4000 个 Unicode 字符。服务端拒绝额外文件正文和未声明的大型字段。

## 13. 日志

成功日志记录：

```text
organization_id
channel_id
conversation_id
service_session_id
message_id
created_contact
inserted_conversation
created_service_session
```

日志不记录访客 Token、Cookie、消息正文、外部身份编号和完整请求体。

日志中的 `inserted_conversation` 记录本次事务是否实际插入 Conversation，用于运维诊断；响应 `createdConversation` 是根据 `sequence = 1` 和 `opening_message_id` 计算的持久谓词。幂等命中时前者为假，后者仍可能为真。

失败日志记录稳定错误类别和内部错误；内部错误不直接返回公开客户端。

## 14. 测试与验收

### 14.1 访客身份

- 首次初始化签发渠道级 Cookie 和响应 Token。
- HTTPS 初始化统一设置 `SameSite=None; Secure`，HTTP 初始化设置 `SameSite=Lax` 且不设置 `Secure`。
- 只有新签发 Token 设置 365 天有效期；合法 Token 初始化不滑动续期。
- 初始化不创建联系人、渠道身份或聊天记录。
- 相同 Cookie 刷新后恢复相同 Conversation 列表。
- Header Token 优先于 Cookie。
- 非法 Token 不读取其他访客数据。
- 发送和历史接口缺少 Token 时返回 `400`，不签发替代身份。
- 两个渠道使用不同 Cookie 名称和数据边界。

### 14.2 首条消息

- 第一条合法文本恰好创建一个联系人、渠道身份、ChatSubject、Conversation、CustomerConversation、Participant、ServiceSession 和 Message。
- 自动联系人创建用户为空，来源渠道正确。
- Conversation 标题来自规范化首条正文。
- 新 ServiceSession `sequence = 1`、`status = waiting`。
- Message 同时更新 Conversation 和 ServiceSession 的最后消息编号与时间。
- 联系人、渠道身份和聊天事实由同一个调用方事务提交。
- 任一步失败不留下部分记录。

### 14.3 Conversation 与 ServiceSession

- 所有线程已关闭后点击开始聊天并发送，创建新的 Conversation。
- 回复未结束线程复用相同 Conversation 和 ServiceSession。
- `waiting` 收到后续访客消息仍保持 `waiting`。
- `pending` 收到访客消息后恢复为 `active`。
- 回复已关闭线程时保持 Conversation 编号，按 `MAX(sequence) + 1` 创建 ServiceSession，并返回该线程完整历史。
- 同一 Conversation 同时最多一个未结束 ServiceSession。
- 同一渠道身份在首版同时最多一个未结束 ServiceSession，因此最多一条正在处理的 Conversation。
- 指定已关闭线程但另一线程未结束时返回 `open_session_exists`，不串写消息。
- 指定没有任何 ServiceSession 的客户 Conversation 时返回内部错误。
- 并发空草稿收敛到一个未结束 Conversation 和 ServiceSession。
- 同一网站渠道的不同访客只锁定各自渠道身份行，不互相等待。

### 14.4 幂等

- 相同 `clientMessageId` 重试返回相同 Conversation、ServiceSession 和 Message。
- 新草稿幂等重试保持相同 `createdConversation` 语义。
- `openedNewServiceSession` 在重试后保持稳定。
- 两个布尔都只按持久谓词计算，不依赖本次是否执行 INSERT。
- 幂等重试不更新最后活跃时间、状态、最后消息或路由。
- 相同键对应不同正文、身份、参与者或非空目标 Conversation 时返回 `idempotency_mismatch`。
- 并发相同消息只创建一条 Message。
- 必需关联缺失或矛盾时返回内部错误。

### 14.5 路由

- 公共队列写空团队和负责人。
- 团队路由写团队编号。
- 成员和 Agent 路由写企业身份编号及分配时间。
- 初始目标不可用时使用失败路由。
- 两级目标都不可用时进入公共队列。
- 跨企业或不可用目标不写入 ServiceSession。

### 14.6 查询

- 初始化只返回当前渠道 Token 对应的 Conversation。
- 列表按 Conversation 最后消息稳定倒序排列。
- 列表主键是 Conversation，不是 ServiceSession。
- 列表标题稳定，预览来自 `conversations.last_message_id`。
- 历史包含同一 Conversation 跨多个 ServiceSession 的全部消息。
- `before` 和 `after` 按 `(originated_at, id)` 不重不漏。
- 游标绑定 Conversation；跨线程或篡改游标返回 `400`。
- 查询不按发送者过滤，响应数组始终按正序返回。
- 其他 Token、渠道和企业的 Conversation 返回不存在。
- 未登录可访问公开路由；成员 Bearer Token 不扩大范围。

### 14.7 Messenger

- 没有未结束线程时，点击开始聊天不创建服务端记录。
- 有未结束线程时，点击开始聊天直接打开它。
- 初始化完成前不能创建草稿或发送。
- 空草稿返回后不进入列表。
- 首条文本成功后，首页和消息页显示数据库 Conversation。
- 空草稿并发命中已有线程时立即重读完整历史。
- 刷新页面后恢复相同列表和历史。
- 已关闭线程再次发送后仍是同一列表项和完整历史，只更新 ServiceSession 摘要。
- 另一线程未结束时不允许把消息写入已关闭线程。
- `open_session_exists` 后重新初始化，切换线程前为保留正文生成新的客户端消息编号。
- 网络失败时正文仍在输入框。
- 重试不创建重复消息。
- 真实入口不显示演示客服回复。
- 真实入口附件和语音不创建本地假消息。
- 管理端预览对 `/api/public` 零请求并保持演示交互。

## 15. 实施顺序

1. 实现事务内 `EnsureChannelIdentity`。
2. 实现入站文本 Action、身份级锁和事务集成测试。
3. 实现 Conversation 列表和完整历史 Query。
4. 实现未注册 Wails 绑定的访客应用服务。
5. 在现有 `/api` Gin Service 注册公开路由并完成 HTTP 测试。
6. 改造 `chat.js` 和页面结构，接入初始化、发送、列表和历史。
7. 保留预览分支，关闭真实入口演示回复、附件和语音假消息。

每一步保持代码可编译。数据库业务不进入 `internal/publicweb`，匿名访客方法不进入桌面端和移动端 Wails 绑定，企业成员收件箱不进入本 PR。
