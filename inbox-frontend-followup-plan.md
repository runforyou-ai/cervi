# 消息页成员侧后续 PR 计划（临时文件，随交付删除）

本文件保存消息页成员侧 PR F2 至 F8 的实施边界，作为已交付 PR F1 之后的交付计划。
设计基线以 `chat-roadmap.md` 与 `inbox-sidebar-prototype.html` 为准；本文件与两者冲突时以两者为准。

## 前提：PR F1 已交付

- 后端已经实现成员按未结束 ServiceSession 读取客户会话工作队列的只读列表查询，
  返回联系人名称、渠道类型、最新消息预览、`last_message_at` 和客服状态。
- 前端已经落地消息页中栏基础框架，包括顶部行、范围纵栏、搜索、会话列表和响应式主区。
- 搜索只做本地过滤；加号菜单保持占位；会话主区目前只展示头部与消息占位。
- 客户队列子筛选当前只有样式交互，尚未改变列表查询。
- 未读徽章、草稿、@我、置顶和静音等个人视图状态仍依赖后续同步与个人状态模型。

## PR F2：成员只读客户会话时间线（已完成）

### 后端

- `Backend` 增加成员按 Conversation 读取消息的查询方法，统一走 appservicegen 生成 Service、
  Gin API 与 API Proxy。
- 查询接口同时接受互斥的 `before` 和 `after` 不透明游标。`before` 用于加载更早历史，
  `after` 作为后续 HTTP 增量补拉入口；本 PR 前端不自动使用 `after`。
- 分页沿用 `(originated_at, id)` 稳定顺序：初始页和 `before` 查询倒序扫描后反转，
  `after` 正序扫描，响应消息始终正序。
- 按当前登录成员的企业边界授权，校验 Conversation 类型为 `customer` 且存在
  `customer_conversations` 扩展；历史权限不依赖最新 ServiceSession 是否未结束。
- 成员消息 DTO 保留稳定消息编号、消息类型、正文、来源时间、入库时间和可空发送者；
  发送者使用 ChatSubject 编号、类型和显示名称，不复用访客侧 `visitor/agent` 二元作者视角。
- 本 PR 只返回未删除文本消息，不修改网站访客查询，不新增数据库迁移。

### 前端

- 主区按 Conversation 加载最近一页消息，渲染访客左侧气泡、企业主体右侧气泡和日期分隔线。
- 初始页使用 `useResource` 和按 `conversationId` 隔离的资源 Key；更早历史通过 API 直接读取，
  在 feature 内累积，不手工修改 TanStack Query 缓存。
- 向上加载更早消息时按消息编号去重，并保持当前阅读位置；切换会话后只展示目标会话数据。
- 展示初始加载、空时间线、读取失败、重试和加载更早状态；窄屏 Sheet 与宽屏主区复用同一实现。
- 不添加 disabled 回复输入区，不创建未接线的 `appendAfter` 导出、通用 Timeline Provider、
  WebSocket 客户端或实时 Context。

### 验收边界

- 成员可以读取当前企业任意客户 Conversation 的完整文本历史，包括已经结束处理批次的线程。
- 跨企业、非客户会话和不存在的编号统一不可读取；`before` 与 `after` 同时传入时拒绝请求。
- 超过一页的历史可以继续向前加载，消息正序、无重复，切换会话不会串线。
- 初始响应在存在消息时返回 `after`，但本 PR 不轮询新增消息。

## PR F3：会话工作区前端骨架

- 保留现有范围纵栏和会话列表；选中区横向拆为 Conversation 工作区和独立联系人上下文栏，
  右栏分隔线从选中区顶部贯穿到底部。
- Conversation 会话头只展示会话标题、渠道和最新 ServiceSession 状态，不与联系人信息混排；
  操作区展示禁用的转给同事、交给 AI 占位，避免在 capability 接入前产生可执行假象。
- 时间线继续只读取当前 Conversation，不按联系人拼接其他 Conversation；ServiceSession 不切断消息历史。
- 底部回复区整体禁用，只建立编辑区、附件操作位和发送操作的布局，不调用回复或上传接口。
- 上下文栏顶部独立展示当前联系人；正文先建立资料、AI 助手和业务三个 tab 骨架，只使用当前
  `InboxConversation` 中已有的联系人字段，不按联系人名称推断其他会话。
- `xl` 以上显示 20rem 上下文栏，并使用贯穿全高分隔线上的中部把手收起或展开；更窄视口
  通过会话头的单一入口打开 Sheet。
- 时间线占满 Conversation 工作区的可用宽度，不使用居中的固定最大宽度容器。
- 本 PR 不修改 appservice DTO、生成绑定、消息查询、资源 Key、`PageSplit` 或 WebSocket 相关代码。

## PR F4：成员文本回复

- 回复统一经过 `appservice.Service`，在事务中校验企业、客户 Conversation 和最新未结束
  ServiceSession。
- 对无负责人的 `waiting` 批次，首次回复时隐式领取并激活；当前负责人回复 `pending` 批次时
  恢复为 `active`；其他成员负责的批次禁止直接抢占。
- 首次实际回复时建立或恢复成员 ConversationParticipant，写入文本 Message，并按
  `(originated_at, id)` 推进 Conversation 与 ServiceSession 摘要和首次响应时间。
- 网站渠道回复直接写 Cervi 时间线，不创建 Delivery；发送成功后刷新或追加当前时间线。
- 不做显式领取按钮、挂起、结束、转接、队列筛选、实时和文件消息。

## PR F5：双方 HTTP 增量补拉

- 网站访客挂件使用现有公开历史接口的 `after` 游标轮询客服新消息。
- 成员打开的客户时间线使用 F2 成员接口的 `after` 游标轮询访客新消息。
- 页面不可见或会话失焦时降低或停止轮询，恢复后立即补拉；请求结果按消息编号幂等合并。
- 本 PR 仍使用普通 HTTP，不创建 SSE、JSON WebSocket 或统一实时基础设施。

## PR F6：公开访客端点防滥用

- 为初始化、发送和历史接口增加来源 IP 与渠道维度限速。
- 发送同时限制访客身份的消息长度、发送频率和未回复 Conversation 数量。
- 限制参数进入渠道配置，不硬编码业务阈值；渠道停用继续作为紧急止血手段。
- 本 PR 可与 F5 独立开发，但必须先于阶段 1B Telegram Bot 交付。

## PR F7：ServiceSession 显式状态命令

- 增加显式领取、挂起和结束命令，全部通过 `appservice.Service` 并在事务中校验状态机。
- 显式领取允许成员先占用排队批次而不立即回复；挂起和结束只允许当前负责人执行。
- 继续保持一条 Conversation 同时最多一个未结束 ServiceSession。
- 不做转接、指标、满意度、队列筛选和 AI 接待。

## PR F8：客户队列子筛选落地

- 列表查询增加状态和负责人维度，前端接通「排队 / 我负责 / 同事 / 已关闭」。
- 「排队」按等待时长升序并显示等待时间；「已关闭」按需读取历史批次。
- 筛选与 URL 查询参数同步；默认落点为「我负责」。
- 「AI 接待」继续保持结构预留，等待阶段 2D 网站 AI 客服。
- 不做未读、全文搜索、联系人搜索分组和转接。

## 统一实时的后续边界

WebSocket 不作为 F2 至 F8 的默认下一步。阶段 1 网站文本闭环使用 HTTP `after` 补拉。
如产品决定提前实时，必须整体前移 Realtime Gateway、同步水位、`realtime_outbox`、Core NATS、
Protobuf、成员与访客短期票据、背压和断线 HTTP 恢复，不能先增加进程内广播、SSE 或 JSON WebSocket。

F2 的 `after` 只覆盖新增文本消息，不是最终统一同步游标。编辑、删除、反应、成员变化和迟到事件
仍由阶段 2E 的 Conversation Sync Event、会话序号、用户 Mailbox 与共享客户收件箱水位处理。
