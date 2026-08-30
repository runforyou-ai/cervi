# Telegram 消息渠道首个 PR 实施方案

## 1. 目标与边界

首个 PR 交付 Telegram 渠道从“创建后可配置”到“Webhook 已注册且服务端能够接收回调”的闭环，同时统一所有消息渠道创建后的页面跳转。

本 PR 包含：

- 保持现有通用渠道创建表单及创建接口不变。
- 所有渠道创建成功后统一跳转到对应详情页，默认打开“基本信息”。
- Telegram 渠道详情、Token 配置、`getMe` 测试、保存校验和 Bot 信息展示。
- Telegram Webhook 注册、公开回调路由、Secret 校验和 Webhook 状态更新。
- Telegram 渠道启用、停用以及更换 Token 时的 Webhook 生命周期。
- Web/原生端提交当前企业服务器地址，并按渠道保存 Webhook 基础地址。

本 PR 不包含：

- Telegram 私聊消息、联系人、会话、消息和投递记录的落库。
- `sendMessage`、客服回复、附件、群聊或其他 Telegram Update 的业务处理。
- 为 Telegram 单独修改通用渠道创建表单、创建入参或创建页面布局。
- 上线引导、帮助教程和非必要说明文案。
- Bot Token 加密存储和接口脱敏；按项目当前阶段约定处理，但日志、错误和右侧信息区不得输出 Token。

## 2. 已确认的产品交互

### 2.1 创建渠道

现有创建表单的渠道类型、名称、描述、默认语言和接待设置均保持不变，仍调用通用 `CreateMessageChannel`。

创建成功后不再返回渠道列表，而是统一跳转到：

```text
/integrations/channels/{channel.type}/{channel.id}?tab=basic
```

这项规则适用于 Website、Telegram、微信公众号以及后续新增的渠道类型。

### 2.2 Telegram 详情页

Telegram 详情页使用与 Website 渠道相同的详情页骨架：页签内容在左、常驻信息区在右。页签为：

```text
基本信息 | 接待设置 | 连接
```

- “基本信息”和“接待设置”继续复用通用表单。
- “连接”页签只放 Bot Token、测试和保存操作。
- 右侧“连接信息”借鉴 Website 预览区的宽度、间距和 sticky 行为，但不是聊天预览，也不渲染 iframe。
- 切换任意页签时右侧信息区保持可见。
- 用户编辑尚未保存的 Token 时，右侧仍展示数据库中的已保存信息；只有保存成功后才更新。

右侧信息区字段固定为：

- Bot 名称
- Bot 用户名（带 `@` 展示）
- Bot ID
- Webhook 地址
- Secret Token
- Webhook 状态

Webhook 状态只有“等待连接”和“连接正常”两种。未配置 Token 或渠道已停用时显示 `—`，它表示没有活跃 Webhook，不增加第三种业务状态。

### 2.3 测试按钮

点击“测试”只使用当前表单中的 Token 调用 Telegram `getMe`：

- 不保存 Token 或 Bot 信息。
- 不调用 `setWebhook` 或 `deleteWebhook`。
- 不修改右侧信息区和 Webhook 状态。
- 成功和失败沿用系统已有连接测试的按钮 loading、toast 和字段错误交互。

后端复用 `connectiontest.Runner` 的超时、日志和错误分类，增加 Telegram 专属 category、adapter 及 i18n 错误映射。日志只记录安全元数据，不记录 Token、完整请求地址或 Telegram 原始响应。

### 2.4 保存按钮

启用中的渠道保存采用以下顺序；整段流程由 channel 级跨实例锁串行化：

```text
获取 channel 级锁
  -> getMe 校验 Token
  -> 数据库事务保存 Token、Bot 信息、客户端提交的基础地址和本次注册凭据
  -> setWebhook
  -> 立即返回保存结果
```

具体语义：

1. 使用 PostgreSQL session advisory lock 按 `channel_id` 串行化单渠道保存、启用和停用，并按 Bot ID 的固定顺序增加跨渠道锁；锁覆盖复用检测、数据库提交和远端调用，远端调用不放进数据库事务。这样既覆盖多实例部署，也避免两个渠道并发复用或交换 Bot 时绕过确认、误删或反向覆盖 Webhook。
2. 前端随保存请求提交当前企业服务器基础地址：Web 端取 `window.location.origin`；桌面端及其他原生端通过现有 `getServerURL()` 读取本地数据库中已连接的企业服务器地址。该字段不增加到可见表单，不由用户重复填写。
3. 后端把客户端字段视为不可信输入，只做 URL 语法和结构校验：必须是绝对 `http` 或 `https` URL，不允许 userinfo、query、fragment，去掉末尾 `/`，允许端口和 path prefix。包括 `127.0.0.1`、`localhost` 和内网地址在内均允许保存，不做公网可达性探测。
4. 先用表单 Token 调用 `getMe`，并校验 `is_bot=true`。失败时整个保存失败，不修改数据库，也不触碰现有 Webhook。
5. `getMe` 成功后检查 Bot ID 是否已被其他渠道保存；未复用时继续保存，已复用且请求未携带确认标记时返回带稳定 reason 的 `409 Conflict`，不修改数据库，也不调用 `setWebhook`。前端使用现有确认对话框说明继续保存会切换 Telegram Webhook，确认后携带标记重新提交。
6. 确认复用不会修改旧渠道记录。Telegram 每个 Bot 只保留最后一次 `setWebhook` 配置，当前渠道保存成功后会自然覆盖旧渠道的远端 Webhook，旧渠道因收不到新回调而自动失效。
7. 在事务中保存 Token、基础地址以及 Telegram 返回的 Bot ID、用户名和显示名称。启用中的渠道同时生成只属于本次注册代次的新 Webhook Secret，清空 `webhook_connected_at`，并把 `webhook_status` 写为 `waiting`，禁止沿用上一代的 `normal`。
8. 后端使用已保存基础地址和 channel ID 拼接完整 Webhook URL，数据库提交后调用 `setWebhook`，不能用远端 API 调用占用数据库事务。
9. 无论 `setWebhook` 成功、明确失败还是超时结果未知，Bot 信息和状态都保持 `waiting`，保存接口以 2xx 返回最新详情。失败只写入不含敏感信息的服务端日志，不新增“异常”状态，也不回滚已保存配置。
10. 保存请求不等待任何 Webhook 回调。通常立即返回 `waiting`；如果本代真实回调已在 `setWebhook` 返回之前完成，则可以直接返回 `normal`，因为异步回调条件已经满足。
11. 已停用渠道允许保存 Token、Bot 信息和当前客户端基础地址，以免出现“无 Token 不能启用、停用后又不能配置”的死锁，但不调用 `setWebhook`，Secret 和状态保持空；重新启用时才创建注册代次。
12. 前端使用保存响应更新表单和右侧信息区。除上述真实回调抢先完成的竞态外，“等待连接”只会在后续收到本次 Secret 对应的真实 Telegram 回调后变成“连接正常”。

保存接口的 HTTP 语义固定为：

| 结果 | HTTP | 响应 |
| --- | --- | --- |
| 客户端基础地址非法或 `getMe` 失败 | 对应的 4xx/5xx 字段/连接错误 | 无写库 |
| Bot 已被其他渠道使用且尚未确认 | 409 冲突及稳定 reason | 不写库，不调用 `setWebhook`；前端请求确认 |
| 确认复用或普通数据库事务成功 | 2xx | 保存当前渠道并按渠道状态注册 Webhook |
| 数据库已提交，`setWebhook` 成功、失败或结果未知 | 2xx | 最新详情，通常为 `waiting`；本代回调已到达时为 `normal` |
| 已停用渠道保存成功 | 2xx | 最新详情，`webhookStatus=null` |

前端对 2xx 使用中性的“连接配置已保存”反馈。`setWebhook` 的同步结果不直接作为界面状态，服务端只记录不含敏感信息的分类日志。

## 3. Webhook 状态模型

Webhook 状态只描述 Telegram 向 Cervi 投递回调的链路，不描述 `getMe` 是否成功，也不等同于渠道的启用状态。

数据库中的活跃 Webhook 状态为：

| 持久化值 | 界面显示 | 进入条件 | 离开条件 |
| --- | --- | --- | --- |
| `NULL` | `—` | 尚未配置，或渠道已停用 | 保存配置或重新启用 |
| `waiting` | 等待连接 | 保存或重新启用后开始注册 | 收到当前 Secret 的有效回调，或重新注册 |
| `normal` | 连接正常 | 收到当前 Secret 的有效回调并成功更新数据库 | 重新保存、重新启用或停用 |

关键状态转换：

```text
保存/重新启用 -> waiting
waiting -> 当前 Secret 的有效回调 -> normal
normal -> 再次保存或重新启用 -> waiting
任意已配置状态 -> 停用 -> NULL
```

“连接正常”只能由真实回调触发，`getMe` 和 `setWebhook` 的响应均不得把状态提升为正常。`setWebhook` 失败、地址不可达或等待时间过长都保持 `waiting`；首个 PR 不引入失败状态或 `getWebhookInfo` 定时健康检查。

### 平台约束

Telegram 的 `setWebhook` 只注册回调地址，并不会主动发送一条握手消息。首个 PR 又只订阅 `my_chat_member`，因此在 Bot 没有产生这类真实 Update 时，“等待连接”可以持续存在。保存 `127.0.0.1`、localhost、内网地址、非 HTTPS 地址，或者 Telegram 拒绝 `setWebhook` 时也会一直保持等待，界面不再区分具体原因。私聊中的 `/start` 或普通消息不会触发该类型，真 Bot 验收可通过拉黑/取消拉黑触发，自动化测试则直接模拟合法回调。该约束保留在工程方案和验收说明中，暂不为未上线功能增加额外引导文案。

## 4. 数据模型与接口

### 4.1 `telegram_channel_settings`

新增 Telegram 渠道扩展表，与 `website_channel_settings` 一样按渠道一对一保存：

- `channel_id`：主键。
- `organization_id`：企业边界。
- `bot_token`：已保存 Token，可为空。
- `bot_id`：Telegram Bot ID，使用 `BIGINT`，可为空；允许多个渠道保存同一个 Bot，以支持用户确认后把 Telegram Webhook 切换到当前渠道。
- `bot_username`：`getMe.username`，可为空。
- `bot_display_name`：优先组合 `getMe.first_name` 和 `getMe.last_name`，可为空。
- `webhook_base_url`：最近一次保存时由客户端提交并经后端规范化的企业服务器基础地址。
- `webhook_secret`：当前注册代次的 Secret，可为空。
- `webhook_status`：`waiting`、`normal` 或空。
- `webhook_connected_at`：当前注册代次最近一次收到有效回调的时间，每次合法回调均更新。
- `created_at`、`updated_at`。

`bot_id` 使用普通查询判断是否被其他渠道保存，只建立非唯一的部分索引。建表迁移为现有 Telegram 渠道回填空设置行，确保升级后详情和启停仍可用；新建 Telegram 渠道时则在现有创建事务中同步初始化空设置记录，创建表单和创建接口无需增加 Telegram 字段。

### 4.2 应用服务契约

在统一 `Backend` 契约中增加：

```text
GET  /channels/telegram/:channelID
POST /channels/telegram/:channelID/connection/test
PUT  /channels/telegram/:channelID/connection
```

建议类型：

- `TelegramChannel`：嵌入 `MessageChannelSummary`，并带 `connection`。
- `TelegramChannelConnection`：`botToken`、`botId`、`botUsername`、`botDisplayName`、`webhookURL`、`webhookSecret`、`webhookStatus`；`botId` 使用不会在 JSON/JavaScript 间损失精度的字符串契约。`webhookURL` 始终由数据库中已保存的基础地址派生，不跟随当前页面地址临时变化，`webhookSecret` 展示当前注册代次随 `setWebhook.secret_token` 一并提交的 Secret Token。
- `TelegramChannelConnectionInput`：`botToken`、`webhookBaseURL`、`confirmBotReuse`；基础地址由客户端自动解析，不渲染为用户输入字段，确认标记只在后端返回 Bot 复用冲突且用户确认后提交。
- 测试接口成功返回空响应，保存接口返回最新 `TelegramChannel`。

业务接口继续经过 `appservice.Service -> DirectBackend -> Action`，并由现有生成器生成 Gin 和 API Proxy 适配代码。测试和保存均校验登录身份、企业边界以及渠道类型必须为 Telegram。

### 4.3 客户端基础地址

复用并泛化 Website 渠道已有的 `resolveWebsiteChannelOrigin()` 逻辑，形成通用的企业服务器基础地址解析能力：

- Web 端返回 `window.location.origin`，即当前浏览器页面的 scheme、host 和 port。
- 桌面端及其他原生端调用现有 `ServerURL`/`getServerURL()`，其值来自本地数据库保存的当前企业服务器连接。
- 解析失败时前端不提交保存，沿用现有会话恢复或网络错误反馈。
- 基础地址只随“连接”表单保存请求提交；测试按钮仍只提交 Token。
- 后端规范化并持久化 `webhook_base_url`，不得直接相信 Host、Origin 或转发头，也不得在详情读取时用当前请求覆盖已保存值。
- 允许合法的绝对 HTTP/HTTPS 地址，包括回环、内网、自定义端口和 path prefix。可保存不代表 Telegram 一定能够访问。

后端使用 URL 路径拼接而非字符串直接连接，保留可选 path prefix，生成：

```text
{webhook_base_url}/api/public/telegram-channels/{channelID}/webhook
```

## 5. Telegram Bot API 适配器

在 `internal/integration/telegram` 提供小型、可注入 HTTP client 的适配器，只实现首个 PR 所需能力：

- `getMe(token)`：读取 Bot 身份，用于测试和保存。
- `setWebhook(token, url, secret)`：注册当前回调。
- `deleteWebhook(token)`：停用和换 Bot 时清理旧注册。

`setWebhook` 固定参数：

```json
{
  "url": "<derived webhook URL>",
  "secret_token": "<current random secret>",
  "allowed_updates": ["my_chat_member"],
  "drop_pending_updates": true
}
```

每次都使用 JSON body 显式传入 `allowed_updates`，因为省略字段会沿用上次设置；不能用空数组表示“不接收消息”，Telegram 会把空数组解释为接收除少数类型外的全部 Update。`deleteWebhook` 固定使用 `drop_pending_updates=true`。

Webhook Secret 使用 `crypto/rand` 生成并编码为 32–64 位 hex 或 URL-safe 字符串，只能包含 Telegram 允许的 `A-Z a-z 0-9 _ -`。

适配器统一校验 Telegram 响应的 HTTP 状态、`ok`、`getMe.is_bot=true` 和必需字段，并映射到 `connectiontest` 的认证、限流、超时、网络和协议错误。由于 Bot Token 位于请求 URL 中，适配器返回分类错误前必须剥离原始 `url.Error`、请求 URL 和响应体；日志只保留 category、adapter、stage、kind、HTTP status、Telegram `error_code` 等安全字段。错误字符串、日志和前端响应均不得出现 Token 或 `api.telegram.org/bot...`。

## 6. Webhook 公开回调

新增无需 Bearer Token 的手写 Gin 路由：

```text
POST /api/public/telegram-channels/:channelID/webhook
```

该路由不进入需要登录身份的通用 `Backend`，而是仿照 Website Visitor 使用独立 Service 和手写 Gin 路由。Handler 只负责受限请求体和 HTTP 状态适配；渠道查询、Secret 校验和状态更新交给独立 Action。服务端组装时显式注入该能力，所有响应只返回裸状态码，不使用登录 API 的 JSON 错误信封。

处理顺序：

1. 校验 channel ID，先只读查询 Telegram 渠道、启用状态和当前 Secret；不存在、类型不符、已停用或 Secret 为空返回 `404`，请求头缺失或常量时间比较不匹配返回 `401`。该预检确保错误 Secret 不必解析请求体。
2. 把请求体限制为 64 KiB，解析首个 PR 所需的最小 Update 契约；格式错误返回 `400`。
3. Action 开启事务，按 `channel_id` 再次联查 Telegram 渠道和设置并 `FOR UPDATE`，重新校验启用状态及请求头与当前 Secret。这样保存、停用或换 Secret 发生在预检之后时，旧请求仍不能写入新代次。事务内不存在/停用返回 `404`，Secret 已变化返回 `401`。
4. 仅当 Update 包含 `my_chat_member` 时，把当前行更新为 `normal`，将 `webhook_connected_at` 写为本代最近一次有效回调时间，然后提交事务并返回 `204`。
5. 重复 Update 的状态写入必须幂等；每次合法回调都刷新 `webhook_connected_at`。
6. 任何尚未支持的业务 Update 不得修改状态或返回 2xx，事务结束后统一返回 `503`，避免 Telegram 把未落库的消息视为已消费。
7. 数据库读取或更新失败返回 `503`，让 Telegram 按其 Webhook 机制重试。

回调成功所证明的是“Telegram 已经使用当前 Secret 到达当前 Cervi 实例”，所以它是 `normal` 的唯一提升来源。

## 7. Webhook 生命周期

### 7.1 更换或重复保存 Token

- 保存、启用、停用均持有同一 channel 级 PostgreSQL session advisory lock；涉及已保存或新识别的 Bot 时，再按 Bot ID 排序持有跨渠道锁。锁覆盖本地事务和随后的远端 API 调用，使用专用数据库连接并保证释放，不能用跨外部调用的长数据库事务替代。
- 每次保存均用新 Token 调用 `getMe` 并生成新 Secret，使状态重新进入本次注册代次的等待流程。
- 数据库事务提交前保留旧 Token 到内存，仅用于后续远端清理，不写日志。
- 如果 Bot ID 改变，且旧 Bot 没有被任何其他渠道引用，在新配置持久化后对旧 Token 执行一次带 `drop_pending_updates=true` 的 best-effort `deleteWebhook`；旧 Bot 仍被其他渠道引用时跳过删除，避免误删其他渠道当前生效的远端 Webhook。同一 Bot 仅刷新 Secret 时不先删除，直接用 `setWebhook` 覆盖。
- 当前公开路由只接受数据库中的新 Secret，因此旧 Bot 或旧注册的在途请求会返回 `401`。
- 新 Bot ID 已被其他渠道使用时，首次保存返回确认冲突；用户确认后允许保存，并通过当前 `setWebhook` 覆盖 Telegram 端的旧注册。旧渠道数据库记录保持不变。
- `setWebhook` 结果不回写 Webhook 状态，因此不会覆盖已经由本代回调写入的 `normal`；本代状态只由保存事务写入 `waiting`、真实回调写入 `normal`。

### 7.2 停用和重新启用

现有通用启停 API 和列表交互保持不变，但 `DirectBackend` 对 Telegram 类型委托专用生命周期 Action，不把远端调用直接塞进当前只更新 `channels.enabled` 的通用 Action：

- 停用：先持久化渠道停用并清空 Webhook Secret、状态和时间；Bot 未被其他渠道引用时再 best-effort 调用 `deleteWebhook`，仍被其他渠道引用时跳过远端删除。Bot Token 和 Bot 信息继续保留；公开路由会因渠道已停用返回 `404`。
- 停用状态下保存：允许更新 Token、Bot 信息和客户端基础地址，不调用 `setWebhook`，Webhook 状态继续为空；如更换 Bot，仍 best-effort 清理旧 Bot 的 Webhook。
- 重新启用：没有已保存 Token 或基础地址时拒绝启用并保持停用；两者都有时生成新 Secret，在事务中持久化启用并写入 `waiting`，然后使用已保存地址注册 Webhook。无论 `setWebhook` 结果如何，启用接口均以 2xx 返回最新渠道状态；只有真实回调能变成 `normal`。
- 启停远端调用不得放在数据库事务内；生命周期 Action 明确表达本地状态优先和远端失败语义。

## 8. 状态刷新

右侧信息区从 Telegram 详情接口读取持久化状态，不直接从浏览器访问 Telegram：

- 保存响应显示 `waiting`，或在本代回调已经抢先完成时显示 `normal`。
- 保存请求进行中时暂停详情轮询并保留右侧上一次已保存的信息；收到保存响应后再替换详情。
- 状态为 `waiting` 时，详情页以 5–10 秒低频轮询刷新 Telegram 详情；一旦回调把数据库更新为 `normal`，右侧随下一次刷新变化。
- 切换页签不停止右侧信息区；离开页面或状态离开 `waiting` 后停止轮询。
- 轮询只读 Cervi 数据库，不在每次前端轮询时调用 Telegram。

轮询挂在 Telegram 详情页资源层，而不是“连接” tab 内；三个 TabsContent 均保留挂载，避免切换页签后停止刷新或丢失尚未保存的 Token 草稿。首个 PR 不做定时 `getWebhookInfo` 核对，也不承诺持续监控已经变为 `normal` 的链路。

## 9. 前端实现落点

- `MessageChannelForm`：只修改创建成功后的统一跳转，不增加 Telegram 分支字段。
- `MessageChannelFormPage`：按渠道类型加载 `TelegramChannel`；Telegram 支持 `connection` tab。
- `TelegramChannelConnectionForm`：password 类型的 Token 输入、测试按钮、保存按钮，复用现有表单和连接测试反馈风格；保存时自动解析并提交客户端基础地址，不增加可见字段，也不提供空 Token 解除绑定语义。
- `TelegramChannelInfoPanel`：右侧只读紧凑描述列表，复用 Website 双栏响应式布局与 sticky 视觉规则，但不复用跟随草稿变化的 `onPreviewChange`。
- `channels` API 封装和生成绑定：补充 Telegram 详情、测试和保存方法。
- 中英文 `channels` 文案：只增加字段名、页签名、按钮反馈、Webhook 两个状态和必要错误，不添加上线引导。

无 Bot username 时不显示孤立的 `@`；尚未保存基础地址时 Webhook 地址显示 `—`。右侧只能展示后端返回的已保存 Webhook URL，不能随当前 `window.location` 临时变化。窄屏时双栏自然折为单栏，先显示当前表单，再显示连接信息；桌面宽屏保持与 Website 详情一致的内容密度，不照搬 Website 预览框的固定高度。

## 10. 错误与并发语义

- `getMe` 失败：不写数据库，不改变已有 Webhook 状态。
- 数据库保存失败或 Bot 复用尚未确认：不调用 `setWebhook`。
- Bot 复用确认：第二次保存成功，旧渠道记录不变，远端 Webhook 切换到当前渠道。
- 数据库已保存但 `setWebhook` 失败或客户端超时：保留 Bot 信息、基础地址和 `waiting`，不增加失败状态；若 Telegram 实际已接受且本代回调到达，则更新为 `normal`。
- 保存过程中到达的旧 Secret 回调：`401`，不得影响当前状态。
- 两次并发保存：channel 级跨实例 advisory lock 串行化整个远端流程，确保最终数据库 Secret 与 Telegram 最后一次注册尝试的 Secret 属于同一代；公开回调事务内重检当前 Secret 作为第二道保护。
- 新 Secret 写入事务时状态固定为 `waiting`，不能继承旧 `normal`。
- `setWebhook` 结果与合法回调并发：注册结果不回写状态，因此不得覆盖已经成功的真实回调。
- 公开回调：锁行、校验当前 Secret、校验启用状态和写入 `normal` 必须在同一事务中，避免 TOCTOU。
- Bot API 远端调用分别使用独立的超时预算；不能让保存中的 `getMe` 和 `setWebhook` 共用一次总超时。

## 11. 验收标准

### 前端

- 任意渠道创建成功后进入其详情页基本信息 tab，创建表单本身无 Telegram 专属字段。
- Telegram 详情具有三个 tab，右侧信息区在三个 tab 中持续展示。
- 测试只调用 `getMe`，不改变右侧信息和数据库。
- Web 端保存时自动提交 `window.location.origin`；桌面端及其他原生端自动提交本地保存的当前企业服务器地址，不增加用户输入项。
- 保存成功后 Bot 信息和已保存 Webhook URL 立即更新；状态显示“等待连接”，不得因为 `setWebhook` 或 `getMe` 成功显示“连接正常”；只有本代回调已经抢先完成时响应可直接为“连接正常”。
- 收到当前 Secret 的有效回调并刷新详情后显示“连接正常”。
- 即使保存的是 `127.0.0.1` 或 `setWebhook` 失败，也持续显示“等待连接”；界面不存在“异常”“未连接”“连接失败”等 Webhook 状态。
- 停用状态下保存只更新 Bot 信息和基础地址，状态按约定显示 `—`，不注册 Webhook。

### 后端

- Telegram 设置遵守企业和渠道类型边界；Bot ID 可跨渠道重复保存，但首次复用必须由用户明确确认。
- Token 或客户端基础地址错误时保存零写入；测试始终零写入。
- 后端允许并保存合法的 HTTP/HTTPS、回环、内网地址，使用已保存基础地址派生 Webhook URL。
- `setWebhook` 使用已保存的派生地址、随机 Secret、`my_chat_member` 白名单和 `drop_pending_updates=true`；Telegram 拒绝地址时状态仍为 `waiting`。
- 公开回调正确覆盖 `404`、`401`、`400`、`204` 和 `503` 分支，Secret 使用常量时间比较。
- 只有当前 Secret 的有效真实回调能把状态提升为 `normal`。
- 停用后回调不可达业务 Action，重新启用重新进入等待流程；换 Bot 后旧 Secret 失效。
- Telegram API 测试覆盖成功、`is_bot=false`、无效 Token、限流、超时、非 JSON/`ok=false` 等响应；错误字符串、日志和前端错误均不泄漏 Token。
- 自动化测试覆盖并发保存、注册结果与回调竞态、回调校验与停用/换 Secret 竞态；两次并发保存后，Fake Telegram 记录的最终 Secret 必须与数据库当前 Secret 一致。
- 真 Bot 验收明确使用能产生 `my_chat_member` 的动作，例如私聊拉黑/取消拉黑；发送 `/start` 或普通消息不作为本 PR 的连接验收方式。

## 12. 后续 PR

下一个业务 PR 再扩展 `allowed_updates` 并实现 Telegram 私聊 Update 的同步持久化、幂等去重、联系人/会话/消息映射和可靠投递。在这些落库能力完成前，首个 PR 不对业务消息返回 2xx，避免静默丢失用户消息。

持续 Webhook 健康监控若以后确有需要再单独设计；当前产品状态只有“等待连接”和“连接正常”，因此不引入 `getWebhookInfo`、投递异常状态或失败降级。

## 13. 官方协议依据

- [Telegram Bot API：getMe](https://core.telegram.org/bots/api#getme)
- [Telegram Bot API：setWebhook](https://core.telegram.org/bots/api#setwebhook)
