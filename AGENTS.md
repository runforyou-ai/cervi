# Cervi（鹿行）

Cervi 是开源、以自托管为主的 AI 原生企业协作产品，使用 Go、Wails v3 和 React，同一套代码支持服务端、Web、桌面端和移动端。

本文件的项目级约定适用于整个仓库；“前端开发约定”适用于 `frontend/`，“后端开发约定”适用于 `internal/`，未覆盖的规则继承项目级约定。

## 协作

- 代码审查结果、Git 提交信息以及 PR 的标题和描述使用中文。
- 需求、范围或实现方式存在不确定或歧义时，先向用户说明待确认内容并获得确认，不自行假定。
- 执行修改前尽量先说明任务范围并询问用户是否开始；用户已明确要求立即执行或已确认该范围时，不重复询问。

## 命令与工作区

- 所有命令从仓库根目录通过 `wails3 task` 执行，Task 自动加载当前 worktree 的 `.env`。不直接调用底层构建工具。
- 每个 worktree 使用独立的 Server、Vite 端口、PostgreSQL 数据库和 NATS 命名空间；PostgreSQL 和 NATS 为共享实例。
- 开发、测试和界面验证统一通过当前 worktree 的公网域名 `https://<worktree 目录名>-dev.runforyou.app` 访问服务端，不使用 `127.0.0.1`、局域网 IP 等内网地址；企业按访问域名识别，换地址访问会进入另一个企业。该域名经常驻的 Cloudflare Tunnel 转发到该 worktree 的 `WAILS_SERVER_PORT`，由用户手动启动。
- 客户端构建使用平台 Task（如 `darwin:build`、`windows:package`），目标架构只传 `ARCH`，不自行设置 `GOOS`、`GOARCH`、`CGO_ENABLED`。客户端固定启用 CGO；纯静态服务端镜像使用 `CGO_ENABLED=0`。
- 每次测试或界面验证结束后，关闭本次启动的服务端、客户端、Vite、MCP 等进程及其子进程，并确认端口已释放；用户明确要求保留时除外。只清理本次启动的进程，不关闭其他 worktree 的进程或共享的 PostgreSQL、NATS。

## Wails 版本

- `go.mod` 中的 `github.com/wailsapp/wails/v3`、`frontend/package.json` 中的 `@wailsio/runtime` 与本机 `wails3` CLI 使用同一精确版本；前端运行时禁止 `latest`、`^` 或 `~`。
- 升级 Wails 时先读目标版本发布说明，用目标版本 CLI 在临时目录生成 React 脚手架，对比官方模板和 `build-assets`。`build/` 含项目定制，不得直接覆盖，只人工合并相关变更。
- 升级后重新生成绑定，验证前端构建、Go 测试、服务端构建和当前平台原生端构建；涉及移动端脚手架时同时验证 Android 与 iOS 构建配置。

## 注释风格

- 代码注释、文档字符串和数据库 `COMMENT` 用简洁的直述型表达，直接描述职责、字段含义、执行行为或必要约束。
- 注释不得包含讨论痕迹、需求确认、方案解释、方案取舍或历史实现对比，不使用“避免……”“不是……而是……”“不再……”等反向表述；必要约束直接写明适用条件、执行规则或不变量。
- 具名函数、方法、组件和导出函数各使用一行简洁、直述型中文注释。
- 只在一处使用且不超过 10 行的逻辑不新增私有辅助函数，在调用处直接实现，并在该段逻辑前加一行中文注释。100 行是函数或组件规模的参考线而非硬限制，略微超过（如约 102 行）可接受，明显增加阅读负担时才拆分。

## 跨端约定

- `appservice.Service` 是统一业务入口：服务端 Web 走 `DirectBackend`，桌面端和移动端走 API Proxy。Gin 只做对外 HTTP API 适配。
- 各端统一使用 Bearer Token，不使用 Cookie；登录令牌保存在 `localStorage`，API Proxy 把应用服务调用转成携带 Token 的 HTTP 请求。唯一例外：公开 Messenger 的网站匿名访客使用渠道级长期 Cookie（`cervi_visitor_<channel_id>`）恢复匿名身份。
- 企业初始化只在 Web 端完成。桌面端和移动端先检测企业服务器并确认企业名称，再连接并进入登录页；登录页展示已连接企业名称并可更换地址。读不到企业名称时，桌面端和移动端回到连接页，Web 端回到初始化页。
- Web 与桌面端共享主要业务页面，移动端保持独立入口。
- 对象存储是部署级配置，整个部署共用一个存储桶，对象键按企业编号隔离。开启时客户端通过服务端签发的预签名请求直传文件，服务端不转发文件内容，Endpoint 使用客户端可访问的公开地址；关闭时文件写入企业服务器的本地最终目录。文件选择后立即上传为临时文件，保存业务数据时在事务中激活；未激活文件默认 24 小时过期，由服务端定时清理。读取按记录中的本地或对象存储类型处理，不受当前开关影响。

## 前端开发约定

### 命令

```bash
wails3 task dev                              # 桌面端开发
wails3 task dev:mcp                          # 桌面端开发并启用 Wails MCP
wails3 generate bindings -clean=true -ts -i  # 生成绑定
wails3 task common:build:frontend            # 前端生产构建
```

- 前端构建和类型检查统一走 Task，不直接调用 `npx vite build` 等。Wails 的 Vite 插件会按当前注册的服务重写 `frontend/bindings`，脱离 Task 执行会删除其他构建目标的绑定；误删后用 `git checkout -- frontend/bindings` 还原，不得提交。

### 代码组织

- `src/api` 按业务域一个文件，页面统一从 `@/api` 导入；共享归一化工具放 `api/normalize.ts`。
- `src/apps` 放 Web、桌面端入口和路由，Web 与桌面端共用路由在 `apps/shared-app-routes.tsx`；移动端入口、路由和页面在 `apps/mobile`。
- 页面归其路由所属的 `features/<业务域>`；被多个 feature 使用的展示组件放 `src/components`，上下文放 `src/contexts`；feature 私有的上下文和 hooks 留在各自目录。
- features 之间不得循环依赖。`features/workspace` 作为路由中枢可以引用各 feature 的页面，其余 feature 不得反向引用 workspace。例外：设置页外壳 `features/settings` 按路由引用 `features/roles` 的页面，保持 settings → roles 单向。

### 国际化

- 通用操作、分页、表格操作列和加载状态优先复用 `common` namespace，业务 namespace 不重复定义。`useTranslation` 显式声明所需 namespace；同一个 `t` 使用多个 namespace 时，跨 namespace 引用使用带 namespace 的键。
- 按语义复用文案，不仅按中文字符串去重；关闭会话、取消发送等业务操作，以及业务标题、校验、成功、失败和风险提示保留在所属 namespace。调整词条时同步更新中英文和调用处，并删除被替代的旧键。

### 业务契约

- `appservice` 契约是前端业务 DTO 的唯一来源。前端不重复声明渠道、联系人、用户、收件箱、设置等业务模型和枚举，也不提交 Wails `$zero`。
- `frontend/bindings` 只由上述生成命令生成，禁止手改或用不同格式覆盖，禁止手工添加注释。
- 页面只通过 `src/api` 调用绑定，不直接引用 `frontend/bindings`：`client` 注入认证与错误并按 `NonNullArrays` 声明结果，`service` 绑定方法。
- 生成类型中的可空切片由服务端保证为数组，前端不逐个字段归一化；只有枚举 `$zero` 收敛和判别式联合在 `src/api` 中显式声明。
- 前端只保留表单值、组件 Props、页面状态和查询参数派生类型。
- 页面卸载时忽略过期结果，不取消 Wails 绑定调用。

### 数据读取

- 页面数据读取统一使用 `src/hooks/use-resource.ts` 的 `useResource`（TanStack Query），不手写 `useEffect` 加过期标志的取数样板。
- 查询 key 统一在 `src/hooks/resource-keys.ts` 的 `resourceKeys` 中定义，页面不手写 key 数组；同一份后端数据在不同页面使用相同 key，查询参数变化必须体现在 key 中。
- 读取错误由 `useResource` 统一做会话入口恢复；变更操作直接调用 `@/api`，成功后通过 `refresh` 或 `useResourceInvalidator` 失效相关 key，不手工修补缓存。
- 会话引导流程（启动探测、身份加载）保持独立实现，不强制走 `useResource`。

### 路由

- `react-router` 锁定精确版本。其 `UNSAFE_` 内部 API 只允许出现在 `src/features/workspace/tab-scoped-router.tsx`；升级 react-router 前先验证该模块行为未变化。

### 表单

- 使用 React Hook Form 和 Zod，统一启用 `shouldUseNativeValidation`。客户端字段校验由浏览器显示在输入控件上，不渲染 `FieldError`，也不弹 Toast；服务端业务错误用 Toast 展示，不用 `setError` 回写字段。
- 桌面端 WebView 中，带 `legend` 的原生 `fieldset`（包括 `FieldSet`）不得作为 flex 容器的直接子项（WebKit 首次布局会保留额外高度）。改用单列 grid，或在外层加普通块级容器，不依赖重绘恢复布局。
- 输入框不使用 placeholder；字段含义由标签表达，必要说明用帮助文案。
- 业务必填字段的可见标签用红色 `*` 标记，优先 `FieldLabel required`；复选框组、表格列、详情编辑行等使用等效标记。未标记即选填，不写“选填”“可选”；条件必填只在条件成立时标记。
- 字段区与底部操作区为同级布局区域，操作区与最后一个表单项间距固定 36px，统一用 `space-y-9`；操作区不得放入 `FieldGroup`，不叠加额外外边距。

### 注释

- `src` 业务代码的文件头说明职责；`components/ui` 只保留文件头。

### 界面验证

- 文案、数字格式、间距、对齐等不改变布局结构或交互流程的简单修改，不做可视化验证，也不为此启动服务或客户端，检查代码差异即可；用户明确要求时除外。
- 新增页面、调整布局结构或交互流程时，主动验证界面效果：优先使用浏览器，涉及桌面端时使用 Wails MCP（`wails3 task dev:mcp`，普通 `dev` 不启用 MCP）。
- 移动端优先使用 `dev:mcp` 启动的 `mobile-preview` 小窗口验证；仅涉及 iOS 或 Android 特有能力时使用模拟器或真机。MCP 缺少所需方法时，可通过 Computer Use 控制桌面窗口或模拟器。
- 本地验证账号可直接登录：`ai.shellphy@gmail.com` / `12345678`；第二账号 `jack@jack.com` / `12345678`，用于双账号群聊、成员访问与群主转让验证。
- 遇到验证码、授权确认等需用户手动完成的步骤，说明当前步骤并暂停，待用户完成后继续，不因此放弃验证或标记为无法完成。

### 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白。工作台一级导航为图标加文字的单行列表，宽度可由用户拖动调整并记在本机；二级栏显示模块标题（消息页除外）。不使用面包屑；需要扩展的设置页和渠道编辑页使用与 URL 同步的页签，不展示空页签。
- 新增、编辑、设置等表单页标题栏不放「返回」；用户通过底部「取消」、页签、二级导航或历史离开。非表单详情子页确需返回列表时，在标题行右上角放文字「返回」。
- 操作过程中保持当前页面布局、内容位置和浏览上下文稳定，不因替换主体 DOM、改变区域尺寸或插入临时内容让既有内容移动、跳动或被挤压。不引起明显布局变化的局部交互可直接在当前页面完成。
- 按任务复杂度和可用空间选择当前页面、Dialog、Sheet 或独立路由，不把某类操作固定绑定到单一载体。Dialog 或 Sheet 保持底层页面状态，关闭后恢复原页签、列表选择、滚动位置和触发焦点；独立路由返回时恢复仍有意义的页面状态。
- 连续操作让用户始终清楚当前任务和返回位置；分步骤或切换视图时不无提示地替换当前内容，控制浮层层级。
- 设置页同一分组内字段使用统一的表单行样式；权限状态等字段不单独用带边框、圆角和内边距的卡片包裹，与相邻字段的标签、帮助文案和控件对齐。
- 数据列表使用带表头的表格，字段独立成列，只展示有管理价值的信息，不用卡片式字段聚合。
- 表格操作列放最右侧，宽度收窄到操作内容所需，表头与按钮左对齐；固定布局表格为操作列保留紧凑宽度，列内同样左对齐。
- 操作列中的按钮和菜单项因权限、身份或状态不可用时保留显示并设置 `disabled`，不通过隐藏改变操作位置；禁用操作不响应点击或键盘。
- 操作按主次排序：主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。操作列中直接展示的详情、编辑、恢复等文字操作统一使用小尺寸描边按钮。
- 管理页面的操作按钮以文字为主，不加装饰性图标；图标仅用于导航、类型、状态和三点菜单。消息页会话头等高频操作区改用图标按钮，操作全部平铺展示，不收进三点菜单，悬停时以提示说明操作。
- 界面文案简洁，面向最终使用者描述操作、结果和影响；只保留必要的标题、标签、校验、状态和风险提示，不写解释数据结构或实现方式的说明文案。
- 确认类弹窗的主操作按钮统一使用「确认」，具体动作和影响由标题和说明表达，不在按钮上重复动词；进行中状态可替换为对应的进行文案。两个动作互斥的岔路弹窗各自写明动作，不套用该规则。

## 后端开发约定

### 命令

```bash
wails3 task db:ensure
wails3 task migrate
wails3 task migrate:status
wails3 task migrate:rollback                         # 可加 STEP=3 或 VERSION=<时间戳>
wails3 task migrate:reset
wails3 task make:migration NAME=create_example_table
wails3 task run:server
wails3 task test:server
wails3 task test:desktop
wails3 task test
wails3 task build:server
```

- `test:server` 使用 `<POSTGRES_DB>_test` 作为测试数据库并在每次运行前重建；同一 worktree 同一时刻只运行一次。
- 集成测试共享当前 worktree 的测试数据库。企业安装测试使用本轮新建的空数据库；其他集成测试通过唯一业务键或测试清理保持数据隔离。

### 代码组织

- `actions/` 按领域组织 Action 与 Query；`api/` 是 Gin 对外 HTTP 适配器；`apiproxy/` 是原生端到企业服务端的类型化代理；`appservice/` 放跨平台应用服务、传输契约和平台 Backend，`appservice/native/` 放原生端平台能力。
- `common` 只放无数据库、无传输层、无平台依赖的通用能力，小函数和错误放包内，完整能力使用子包。`domain` 只放各层共用的领域值，按概念拆文件。
- 服务端 PostgreSQL 模型放 `storage/server`，桌面端 SQLite 模型放 `storage/desktop`，移动端 SQLite 模型放 `storage/mobile`；桌面端和移动端的 SQLite 迁移保持独立。
- `task` 根包只放各平台共享的 Action 执行语义；`task/client` 与 `task/server` 各自定义投递参数、存储与运行机制，不互相复用平台实现。
- 跨 Action、应用服务和存储的真实数据库集成测试放 `integrationtest/`。

### 分层

- Gin 只输出 Backend 给出的状态和错误体，不定义前端业务类型和主要调用契约。
- `Service` 的每个带结果方法都对结果调用 `normalizeSlices`，nil 切片输出为空数组；`manual=service` 的手写方法同样遵守。
- `appservice/backend.go` 的 `Backend` 接口是业务调用的唯一契约源，每个方法必须带 `cervi:route` 指令。`Service` 委托、服务端认证分发、Gin 路由与 Handler、API Proxy 转发由 `go generate ./internal/appservice` 生成到各包的 `*_gen.go`，禁止手改。
- `appservice/operator_backend.go` 的 `OperatorBackend` 接口是官方托管运营调用的契约源，同一条生成命令按其指令生成运营认证分发和 Gin 适配，不生成 `Service` 委托、API Proxy 和 Wails 绑定。`Backend` 面向各端客户端，`OperatorBackend` 面向 SaaS 后端的服务间调用，新增方法按消费者归入其中一个，不跨契约暴露。
- 运营指令不接受 `auth` 和 `manual` 选项：分发层一律先校验运营服务凭据，再把运营身份交给 `operatorOperations` 中的业务实现。运营错误使用带稳定错误码的 `OperatorError`，目标企业只取自路径或请求体中显式给出的企业编号。
- 新增业务方法：在 `Backend` 补方法与指令（GET 的查询结构体在 `types.go` 为每个字段显式加 `query` 标签，不传输的字段用 `query:"-"`），运行生成器，然后只手写 `directOperations` 实现和 Action。无法按统一模式生成的层用 `manual=service,api,proxy` 标记并在对应包手写；API Proxy 的 `normalizeOutput` 只按响应类型补全企业服务器文件地址，不重复切片归一化。
- 认证由 `direct_backend_gen.go` 生成的分发层统一处理：`auth` 默认 `member`，先解析登录身份再调用业务实现；无需登录的方法标记 `auth=public`。
- `directOperations` 直接接收已解析的 `identity`，不重复认证，只负责把 Action 返回的语言无关错误码转成结构化、本地化错误并调用 Action。其 Action 与 Query 字段按业务域分组在 `<域>Ops` 结构体中，新增依赖只改对应实现文件。
- 只读 Query 信任分发层已解析的身份，不重复查询用户状态；写 Action 在事务开始时通过 `actions/identity.LockActiveUser` 校验并锁定活跃用户。
- Action 直接使用 Bun，按需调用 `common`；记录关联、组织边界和业务规则在事务中显式校验和维护。

### 数据与迁移

- 本地不运行 S3 兼容服务；对象存储由服务端部署配置的 `storage.s3` 字段和 `S3_*` 环境变量管理，可使用任意客户端可访问的临时 S3 兼容服务。
- 回滚和重建库结构前先停止服务端；重建使用 `migrate:reset`，或先回滚再 `migrate`。
- 已合入 `main` 的迁移不可修改、重命名或重排；后续结构变化新增时间戳更晚的增量迁移，并提供 Down 迁移。
- 同一未合并 PR 内调整数据结构时，直接修改或合并尚未合入 `main` 的对应迁移，只保留最终结构，不为 PR 内已放弃的中间方案追加过渡、修正或清理迁移。本地已执行旧版本时手动调整开发库或迁移记录，不把一次性修正写入产品迁移。
- 新迁移文件统一通过 `wails3 task make:migration NAME=<name>` 生成，时间戳取命令运行时的本地实际时间；禁止手工编造时间戳。
- 建表迁移命名为 `YYYYMMDDHHMMSS_create_<table>_table.sql`，每个文件只建一张表，不创建外键和 `CHECK` 约束。
- 迁移中用简洁中文 `COMMENT ON` 说明表和业务字段。

### 当前阶段

- 以贯通 MVP 主流程和验证产品价值为优先，不为尚未出现的生产规模问题预先增加配额、限流、复杂重试、降级、穷举式参数限制或防御性分支；保持可扩展的清晰边界，上线前再集中补齐安全、容量和异常边界。已有约定的认证、企业数据隔离、事务一致性和业务幂等仍须遵守。
- 不考虑历史数据和旧接口兼容。改模型、迁移和接口时直接实现目标结构，不写旧数据回填、缺失记录兜底或双版本逻辑，除非任务明确要求。
- 优先建立长期正确、语义清晰的领域模型和接口契约；不用借用字段语义、查询过滤、兼容分支或局部兜底掩盖模型问题。基础契约不合理时直接调整数据模型、业务边界和调用链路，并删除被替代的旧实现。
- 迁移只保留主键和用于业务约束、幂等及并发正确性的唯一索引，普通性能索引上线前统一评估补充。
- 密钥加密存储、接口响应脱敏等安全加固暂不阻塞开发和审查。
- 角色权限后续统一建设；当前只校验已登录，不按管理员或普通成员限制功能。

### 国际化

- 用户可见文案统一由 `internal/i18n` 管理。语义一致的文案复用同一个 `Key`，共享键按共同语义命名，不按调用入口重复定义；字面相同而语境不同的文案保留独立键。
- 合并翻译键时同步更新 Go 常量、中英文词条和调用处，删除旧键；业务错误类型、原因码和校验规则保持独立，不随文案复用合并。
