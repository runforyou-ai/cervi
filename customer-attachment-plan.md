# 客户会话附件收发方案

本文定义客户会话中图片与文件的收发范围、数据模型、渠道能力边界和交付顺序，覆盖网站访客、企业成员、Telegram 与 AI 客服四个方向。本文实现 `chat-roadmap.md` 阶段 1D 中「文件消息复用项目既有临时上传流程，保存 Message 关联时在同一事务激活文件」的设计。

## 1. 范围

| 方向 | 当前状态 | 本方案目标 |
| --- | --- | --- |
| 网站访客发送附件 | Messenger 只在本地渲染选中的文件，不上传 | 访客上传并发送图片与文件，成员和 AI 客服可读 |
| 网站访客接收附件 | 不支持 | 展示成员与 AI 客服发送的附件，图片内联预览，文件下载 |
| 成员在客户会话发送附件 | 输入区附件按钮为禁用占位 | 与内部聊天一致的选择、上传队列和发送，按渠道能力启停 |
| Telegram 入站媒体 | 只解析文本消息，媒体被忽略 | 照片、文件、语音、视频、音乐和动画进入客户会话时间线 |
| Telegram 外发媒体 | 投递只调用 `sendMessage` | 按附件类型调用对应发送接口并复用既有投递状态机 |
| AI 客服读取附件 | 运行期支持附件上下文，调度入口只接受文本 | 打通调度入口，附件就绪后触发 |

联系人头像、知识文档和内部聊天附件不在本方案范围内。

## 2. 现有基础与既有约束

- `files` 保存企业文件元数据，`status` 走 `pending → uploaded → active`，未激活文件 24 小时过期由定时任务清理。`storage_key` 唯一，`external_id` 当前保存 Telegram 联系人头像的 `file_unique_id`。
- `message_attachments` 以 `message_id` 为主键，`file_id` 唯一，保证一个上传文件只关联一条消息。这条不变量在本方案中保留。
- `SendAttachmentMessageAction` 在一个事务内完成会话锁定、幂等核对、消息写入、附件关联、文件激活和阅读水位推进，覆盖单聊、群聊、AI 聊天和 Copilot 线程。
- `CreateUploadAction` 对 `message_attachment` 用途不设字节上限；`CompleteUploadAction` 要求登录身份且校验 `created_by_user_id` 与当前用户一致。
- 对象存储开启时文件由客户端直传预签名地址，签名有效期 15 分钟；关闭时通过 `LocalObjectService` 的 `PUT /objects/...` 写入本地最终目录。本地对象 GET 不校验身份，依赖不可枚举的存储键；PUT 要求 Bearer 令牌，CORS 允许头当前只有 `Authorization` 和 `Content-Type`。
- `ImportAction` 把服务端取得的字节写为可激活的临时文件，用途写死为 `contact_avatar`。
- `filemaintenance` 的清理任务删除过期文件时执行 `UPDATE message_attachments SET file_id = NULL`，不改附件的其他状态。
- `customerdelivery.Worker` 认领后无条件调用 `telegram.SendText`，不看消息类型，`sendTimeout` 为 20 秒。
- `SendCustomerTextMessageAction` 查找 Telegram 平台引用目标时限定 `msg.type = text`；`list_conversation_messages` 的 `reply_unavailable` 判定同样限定文本。
- `agentrun.loadCustomerInputSender` 限定 `msg.type = text`，`ScheduleCustomerAuto` 在取不到发送者时返回错误，调用方在入站事务内会因此整体回滚。
- `list_website_conversations` 以 `JOIN LATERAL (... type = text ...) ON TRUE` 取最后一条消息，`list_website_messages` 限定 `msg.type = text`。
- `load_inbox` 的未读统计限定 `type IN (text, agent_error)`，附件消息当前不计未读。
- 成员侧 `list_conversation_messages` 已包含 `attachment`，引用目标也已包含 `attachment`。
- 前端 `attachment-queue.ts` 的发送固定调用 `sendAttachmentMessage`，该接口拒绝客户会话；`ConversationComposer` 由 Web、桌面端和移动端共用。
- 访客 Messenger 是 `internal/publicweb/chat.js` 的原生脚本，已有附件选择、图片灯箱、粘贴图片和大小格式化，全部只作用于本地对象地址，且多个文件渲染在同一条本地气泡中。

## 3. 数据模型

本方案不新增业务表，附件消息统一使用 `messages.type = attachment` 与 `message_attachments`，客户会话与内部会话共用同一份事实。

新增一条迁移调整既有列：

- `message_attachments` 增加 `transfer_status text NOT NULL`，取值 `ready`、`pending`、`failed`，表达外部渠道媒体内容是否已取回。成员和访客上传的附件在写入时即为 `ready`；Telegram 入站媒体先写 `pending`，内容取回并激活后置为 `ready`，取回失败或文件被清理置为 `failed`。
- `files` 增加可空 `uploader_channel_identity_id uuid`，表示该文件由指定渠道身份的访客上传。成员上传保持为空。

每条附件消息拥有独立的 `files` 行，不跨消息复用文件记录：

- `file_id` 唯一与 `storage_key` 唯一同时成立，同一外部文件重复入站各自下载并各自存储。
- `external_id` 只作来源标记，供排查与对账使用，查找时必须同时限定 `purpose`，避免命中联系人头像文件。
- Telegram 回调重放的防重继续依赖既有的消息幂等键 `chmsg:<channelID>:tg:<botID>:<chatID>:<messageID>`。

附件状态与文件状态的对应关系：

| 情形 | `message_attachments.file_id` | `files.status` | `transfer_status` |
| --- | --- | --- | --- |
| 取回中 | 有 | `pending` | `pending` |
| 就绪 | 有 | `active`，`expires_at` 为空 | `ready` |
| 取回失败或已清理 | 空 | 已删除 | `failed` |

清理任务在把 `file_id` 置空的同一语句中把仍被附件引用的记录置为 `failed`。

## 4. 渠道能力

在 `internal/domain/channel.go` 增加附件能力判定，与已有的 `ChannelSupportsAgentAssignee` 并列：

- `ChannelSupportsInboundAttachment(channelType)`：网站与 Telegram 返回 `true`。
- `ChannelSupportsOutboundAttachment(channelType)`：网站返回 `true`；Telegram 在外发媒体交付前返回 `false`，PR4 改为 `true`。
- `ChannelAttachmentLimit(channelType, contentType)`：返回单个附件的字节上限。网站取 20 MiB；Telegram 入站取 20 MiB（`getFile` 硬上限），外发照片取 10 MiB、其他类型取 50 MiB。
- `ChannelCaptionLimit(channelType)`：返回附件说明上限。网站取 4000，Telegram 取 1024。

能力判定作用于三处，服务端是权威：

1. 客户会话摘要返回 `attachmentSupported`、`attachmentByteLimit` 和 `attachmentCaptionLimit`，成员端据此禁用入口、在选择文件时丢弃超限文件并限制说明长度。规则由服务端计算，前端不重复实现判定。
2. `SendCustomerAttachmentMessage` 在事务内按渠道再次校验字节数与说明长度，超限分别返回附件过大和说明过长冲突。通用的 `CreateUploadAction` 不感知目标会话，因此不在创建上传阶段拦截成员上传。
3. 访客的公开上传端点是渠道专用的，在创建上传时即按网站渠道上限拒绝。

渠道不支持附件外发时，成员输入区的附件入口保留显示并禁用，禁用原因复用现有的不可发送提示。

数量约束：单条附件消息只携带一个文件，多选按选择顺序发送为独立消息，说明文字随最后一个附件发送，引用只挂在第一个附件上；一次选择的数量上限与内部聊天一致为 100，访客端的上限在 PR2 单独确定。

## 5. 网站渠道

### 5.1 访客上传

新增两条公开路由，沿用 `authorizeWebsiteVisitor` 的访客令牌授权：

```text
POST /public/website-channels/:channelID/attachments          创建上传
POST /public/website-channels/:channelID/attachments/:fileID  完成上传
```

`WebsiteVisitorService` 新增 `CreateAttachmentUpload` 与 `CompleteAttachmentUpload`，两者都由访客专用 Action 实现，不复用要求登录身份的 `CreateUploadAction` 与 `CompleteUploadAction`。

创建上传时校验渠道启用，并按访客令牌解析出的外部编号幂等建立渠道身份（新访客的首条消息可以是附件，此时身份在创建上传这一步落库，而不是等到首条消息事务）。随后按企业当前存储配置创建 `pending` 文件：`purpose` 为 `message_attachment`，`created_by_user_id` 取渠道创建者，`uploader_channel_identity_id` 取该访客的渠道身份编号，`expires_at` 为 24 小时。返回结构与成员侧一致，对象存储模式下是预签名 PUT，本地模式下是 `/objects/...` 地址。

完成上传与后续发送都按 `uploader_channel_identity_id` 与访客令牌解析出的渠道身份比对归属，该列在文件生命周期内不清空，重试与并发上传互不影响。

本地模式的 `PUT /objects/...` 增加访客分支：请求携带 `X-Cervi-Visitor-Token` 时按存储键反查文件，校验文件处于 `pending`、未过期、用途为消息附件，且 `uploader_channel_identity_id` 与令牌解析结果一致。`LocalObjectService` 的 CORS 允许头增加 `X-Cervi-Visitor-Token`。

分片上传不向访客开放，超过上限的文件在创建上传时即被拒绝。

### 5.2 访客发送

新增 `POST /public/website-channels/:channelID/attachment-messages`，输入为 `conversationId`（可空，首条消息时由服务端创建会话）、`clientMessageId`、`fileId`、`body`、`replyToMessageId` 与图片宽高。

`ReceiveWebsiteCustomerTextMessageAction` 的会话与客服周期语义抽为共用入站函数，附件与文本在创建访客联系人与渠道身份、创建或复用 Conversation、复用或续开 ServiceSession、推进会话版本、发布受众通知上完全一致。以下差异在共用函数中显式表达：

| 项目 | 文本 | 附件 |
| --- | --- | --- |
| `messages.type` | `text` | `attachment` |
| 正文 | 必填 | 可为空 |
| 正文长度 | 按渠道上限 | 按渠道说明上限 |
| 幂等核对 | 类型与正文一致 | 类型、正文、文件编号与图片宽高一致 |
| 新会话标题 | 取正文 | 正文为空时取文件名 |
| `search_vector` | 正文 | 正文与文件名 |
| 事务内附加动作 | 无 | 写入附件关联并激活文件 |

### 5.3 访客读取

`list_website_messages` 的消息类型扩到 `text` 与 `attachment`；`list_website_conversations` 的 LATERAL 子查询同样扩展，使只含附件的会话保留在列表中，预览按最后一条消息的类型展示文件名或说明文字。

`WebsiteVisitorMessage` 增加 `attachment` 字段，携带文件名、内容类型、字节数、图片宽高、`transferStatus`，以及 `previewUrl` 与 `downloadUrl`。`transferStatus` 不是 `ready` 时两个地址为空，避免指向未激活对象。

历史列表内联返回这两个地址。对象存储模式下预签名有效期为 15 分钟，挂件长时间打开后地址会失效，因此同时提供重签端点：`GET /public/website-channels/:channelID/conversations/:conversationID/messages/:messageID/attachment`，校验该消息属于该访客的会话且未删除后返回新的地址对。访客脚本在图片加载失败或点击下载时调用该端点重签。

### 5.4 访客界面

`chat.js` 现有的选择、预览和灯箱保留，发送链路改为真实上传，并调整多选行为：

- 多个文件不再渲染在同一条本地气泡中，改为按选择顺序各自成为一条消息，说明文字随最后一条发送，与成员侧一致。
- 选中文件后立即创建上传并开始传输，气泡内显示进度与取消。
- 失败保留气泡与重试入口，切换会话保留各自的待发送附件。
- 收到的附件按类型渲染：图片内联并可点开灯箱，其余显示文件名、大小和下载入口。
- `transferStatus` 为 `pending` 时显示接收中，为 `failed` 时显示接收失败，两种状态都不提供下载。

## 6. 成员侧

新增 `SendCustomerAttachmentMessage`，`auth` 为 `member`，输入为 `conversationId`、`clientMessageId`、`fileId`、`body`、`replyToMessageId` 与图片宽高。

实现复用 `SendCustomerTextMessageAction` 的完整客服语义：

1. `deliveryaction.Prepare` 读取外发路由并按渠道类型加锁。
2. 校验渠道支持附件外发、字节数与说明长度在渠道上限内。
3. 锁定客户会话与当前客服周期，校验周期未关闭且负责人为本人或无人负责。
4. 幂等核对沿用 `mmsg:<identityID>:<clientMessageID>` 键，核对内容扩展到文件编号与图片宽高。
5. 校验文件属于本企业、由本人上传、`uploader_channel_identity_id` 为空、处于 `uploaded` 且未过期。
6. 首次回复隐式领取、记录首次响应时间、写入消息与附件、激活文件。
7. 渠道支持外发媒体时在同一事务入队投递。

前端改动不止于打开按钮：

- `attachment-queue.ts` 的 `send` 按批次目标分派，客户会话调用 `sendCustomerAttachmentMessage`，其余保持 `sendAttachmentMessage`，并处理两者不同的返回结构。
- 队列批次保存输入区当前的引用摘要，随第一个附件发送并在本地气泡中展示；发送提交后清除输入区引用。
- `conversation-composer.tsx` 的附件入口条件加入 `ConversationTypeCustomer`，同时按运行平台限定：PR1 只对 Web 与桌面端开放，移动端在 PR5 打开。
- 禁用条件合并渠道外发能力与当前可发送状态；选择文件时按 `attachmentByteLimit` 丢弃超限文件并提示。
- `loadMessageAttachments` 返回 `transferStatus`，`ConversationAttachment` 在非 `ready` 时不请求下载地址。
- 客户会话附件气泡的外部投递状态随 PR4 的 Telegram 外发一并展示，网站渠道不产生投递记录。

`GetAttachmentDownload` 按 `authorizeConversationHistory` 判定可见性，客户会话已在覆盖范围内，无需改动。

`load_inbox` 的未读统计加入 `attachment`，使客户和成员发送的附件与文本一样计入未读。该改动同时作用于内部会话，内部聊天收到的附件此前不计未读。

无效文件（不存在、未完成上传、已激活、由访客上传）统一返回文件未找到，与内部附件一致，不落为笼统的发送失败。

## 7. Telegram

### 7.1 入站

`TelegramWebhookInput` 扩展解析 `photo`、`document`、`voice`、`video`、`video_note`、`audio` 和 `animation`，取 `file_id`、`file_unique_id`、`file_name`、`mime_type` 和 `file_size`；照片取最大尺寸档位并带上宽高。缺少文件名的类型按种类使用默认名（语音 `voice.ogg`、视频备注 `video-note.mp4` 等）。`caption` 作为附件说明写入正文。相册按 `media_group_id` 分组的多个文件各自成为一条消息，与内部聊天的多选行为一致。

Webhook 保持同步受理，媒体内容不在回调内下载：

1. 事务内创建 `pending` 文件记录与附件消息，`transfer_status` 为 `pending`，通过 `TxEnqueuer.EnqueueIn` 投递取回任务。
2. 事务提交后任务调用 `getFile` 下载内容，写入存储，把文件置为 `active`，附件置为 `ready`，推进会话版本并发布受众通知，随后调度 AI 客服。
3. 超过 20 MiB、`getFile` 拒绝或重试窗口耗尽时，附件置为 `failed`，消息保留在时间线中并显示无法接收，同样调度一次 AI 客服。
4. `getFile` 返回的下载链接官方只保证至少一小时有效，过期后重新调用 `getFile` 取得新链接。取回任务因此在每次重试时重新调用 `getFile`，不缓存下载链接；重试窗口按 `files` 记录 24 小时的过期窗口设定，窗口内始终失败才置 `failed`。

`ImportAction` 增加用途参数，供取回任务按 `message_attachment` 写入内容。

入站附件出现后，Telegram 引用目标从只认 `text` 扩到同时接受 `attachment`：`SendCustomerTextMessageAction` 的平台引用查询和 `list_conversation_messages` 的 `reply_unavailable` 判定同步调整，使成员可以引用客户发来的照片。

### 7.2 外发

`customerdelivery.Worker` 的 `claim` 扩展读取消息类型、附件元数据与存储位置，`Execute` 按类型分派：

- 文本消息继续调用 `SendText`。
- 附件消息按内容类型选择 `sendPhoto`、`sendVoice`、`sendVideo`、`sendAudio`、`sendAnimation` 或 `sendDocument`，以 multipart 上传文件内容，`caption` 取消息正文，引用沿用现有的 `reply_parameters`。
- 媒体发送使用独立的更长超时，按渠道上限的 50 MiB 与常见上行带宽设定，与文本的 20 秒分开。

`internal/integration/telegram/send.go` 增加 `SendMedia`，输入为聊天编号、方法、字段名、文件名、内容类型、内容读取器、说明和引用编号，返回平台消息编号。失败分类沿用现有的 `SendError`：网络与超时进入 `uncertain`，平台明确拒绝进入 `failed`。

超出渠道上限的字节数与说明长度在发送校验阶段就被拒绝，不进入投递队列。

## 8. AI 客服

`agentrun.loadCustomerInputSender` 的消息类型从 `text` 扩到 `text` 与 `attachment`，否则附件调用 `ScheduleCustomerAuto` 会返回错误并回滚入站事务。

自动触发只针对客户入站消息：`ScheduleCustomerAuto` 要求发送者是联系人且当前负责人是合格 AI 员工，成员发送会隐式领取为本人，因此成员附件不触发 AI。调度时机按附件内容是否就绪区分：

- 网站访客发送的附件在落库事务内即为 `ready`，与文本一致在同一事务内调度。
- Telegram 入站媒体在落库事务内不调度，由取回任务在 `ready` 或 `failed` 后调度一次。`failed` 时运行期按既有降级规则只使用文件名与说明。
- `agent_inputs` 只有 `(lane_id, input_seq)` 唯一约束，重复调度会产生重复输入。取回任务因此把 `transfer_status` 从 `pending` 改为 `ready` 或 `failed` 与调度放在同一事务，并以该转换的影响行数为准：只有真正完成状态转换的那一次才调用 `ScheduleCustomerAuto`，重试落到已是终态的记录时不再调度。

运行期不需要改动：`customerRunPolicy` 已把附件消息投影为模型上下文，图片按模型输入模态直传或降级为链接。AI 客服本身不发送附件，只产出文本回复。Copilot 背景资料随客户会话上下文自动包含附件消息。

## 9. 交付顺序

| PR | 范围 | 依赖 | 验收 |
| --- | --- | --- | --- |
| 1 | 迁移、渠道能力判定、`SendCustomerAttachmentMessage`、前端队列分派与上传前预检、Web 与桌面端输入区、AI 调度入口接受附件类型 | 无 | 网站渠道会话中成员可发图片和文件并可携带引用；`ScheduleCustomerAuto` 接受附件类型消息（供 PR2、PR3 的客户入站使用）；Telegram 渠道附件入口禁用且不入队；超限文件在选择时被丢弃、在服务端被拒；无效文件返回文件未找到；移动端入口保持关闭 |
| 2 | 访客上传两条端点、本地对象访客分支与 CORS、附件消息端点、访客历史与会话列表含附件、下载重签端点、`chat.js` 真实上传 | 1 | 访客首条消息即可为附件并创建会话；只含附件的会话仍出现在访客列表；图片内联预览且签名过期后可重签；超限文件在创建上传时被拒 |
| 3 | Telegram 入站媒体：七类媒体解析、`pending` 附件与取回任务、失败终态、引用目标扩到附件、清理任务同步附件状态 | 1 | 七类媒体进入时间线；同一文件重复发送各自独立入库；取回中与取回失败各自可见；AI 在就绪或失败后恰好触发一次；成员可引用客户发来的照片 |
| 4 | `SendMedia`、Worker 按类型分派、媒体发送超时、Telegram 外发能力开关打开 | 1（引用入站媒体弱依赖 3） | 成员发送的附件送达 Telegram；引用保持；网络失败进入 `uncertain` 并可人工确认；平台拒绝进入 `failed` 并可重试 |
| 5 | 移动端客户会话附件：输入区开放、上传队列、发送与应用内预览 | 1、2 | 移动端客户会话可发可看附件，复用登录工作区上传队列 |

## 10. 暂不包含

- 访客端的分片上传与断点续传。
- 公开上传端点的频率限制与配额，按当前阶段约定统一后置。
- 微信公众号等其他渠道的媒体收发。
- 客服侧的附件转存、内部备注附件和 Copilot 线程向客户会话转发附件。
- 图片压缩、缩略图生成和视频转码。
- 附件内容的病毒扫描与敏感内容审核。
