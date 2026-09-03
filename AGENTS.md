# Cervi（鹿行）

## 项目背景

Cervi 是一款开源、以私有化部署为主的 AI 原生企业协作产品。项目使用 Go、Wails v3 和 React 开发，同一套代码支持服务端、Web、桌面端和移动端。

## 约定作用域

- 本文件定义整个仓库共同遵循的项目级约定，以及前端和后端的目录专属约定。
- “前端开发约定”适用于 `frontend/`，“后端开发约定”适用于 `internal/`。
- 目录专属约定未覆盖的规则继续继承项目级约定。

## 工作区与命令

所有命令从仓库根目录通过 Wails v3 和 Task 执行；Task 自动加载当前 worktree 的 `.env`。

```bash
# 初始化工作区与共享依赖
cp .env.example .env
docker compose up -d postgres nats

# 当前宿主平台的桌面端构建与打包
wails3 task build
wails3 task package

# 交叉编译准备；之后照常调用目标平台的 *:build Task
wails3 task setup:docker
```

- 每个 worktree 使用独立的 Server、Vite 端口、PostgreSQL 数据库和 NATS 命名空间；PostgreSQL 和 NATS 仅在主工作区共享启动。
- 客户端统一使用平台 Task，目标架构只传 `ARCH`，不要自行调用底层构建工具或设置 `GOOS`、`GOARCH`、`CGO_ENABLED`。客户端固定启用 CGO；纯静态服务端镜像使用 `CGO_ENABLED=0`。
- `darwin:build`、`windows:build` 和 `linux:build` 会按宿主环境选择原生或 Docker 工具链。交叉编译只生成二进制或未签名应用包；正式桌面安装包在目标系统或同平台 Runner 构建。Linux 不支持异架构桌面端交叉编译，Windows 宿主不支持交叉构建其他平台，WSL 按 Linux 处理。
- macOS 的 DMG、签名和公证在 macOS 完成；Windows 安装包在原生 Windows 或 Windows Runner 完成；iOS 在 macOS 完成。服务端多平台归档由 `.github/workflows/release.yml` 的 `server-assets` 作业生成。
- 仅当用户明确要求提交代码或提交 PR 时，才运行与任务相关的测试和构建；其他情况不运行测试、`go vet` 或构建。

各平台构建和打包统一使用以下 Task：

```bash
wails3 task darwin:build ARCH=arm64
wails3 task darwin:package ARCH=arm64
wails3 task darwin:package:universal
wails3 task darwin:create:dmg
wails3 task windows:build ARCH=amd64
wails3 task windows:package ARCH=amd64 INSTALL_SCOPE=machine
wails3 task windows:package ARCH=amd64 INSTALL_SCOPE=user
wails3 task linux:build ARCH=amd64
wails3 task linux:package ARCH=amd64
wails3 task ios:package
wails3 task ios:package IOS_PLATFORM=device CODESIGN_IDENTITY="Apple Development: ..."
wails3 task ios:package:ipa IOS_PLATFORM=device CODESIGN_IDENTITY="Apple Development: ..."
wails3 task ios:xcode
wails3 task android:package
wails3 task android:package:fat
wails3 task android:bundle
wails3 task android:bundle:fat
```

真机在连接页手动输入可访问的企业服务端地址。Cloudflare Tunnel 由 Dashboard 管理路由，本机使用 `~/.cloudflared/cervi-dev.token` 启动一份 connector。

## Wails 版本

- `go.mod` 中的 `github.com/wailsapp/wails/v3`、`frontend/package.json` 中的 `@wailsio/runtime` 与本机 `wails3` CLI 必须使用同一精确版本；前端运行时禁止使用 `latest`、`^` 或 `~` 范围。按 `go.mod` 安装 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)`。
- 升级 Wails 时先阅读目标版本的官方发布说明，再使用目标版本 CLI 在临时目录生成 React 脚手架，对比官方模板和 `build-assets`。`build/` 已包含服务端、桌面端和移动端定制，不得直接覆盖；只人工合并与当前项目相关的官方变更。
- Wails 升级后重新生成绑定，并验证前端构建、Go 测试、服务端构建和当前平台原生端构建；涉及移动端脚手架时同时验证 Android 与 iOS 构建配置。

## 代码组织

```text
cervi/
├── main.go                         # 应用入口和 Wails 配置
├── application_services_*.go       # 按原生端和服务端注册服务
├── internal/                       # 后端业务、存储和平台能力
├── frontend/                       # Web、桌面端和移动端前端
└── build/                          # Wails 多平台构建配置
```

## 跨端约定

- `appservice.Service` 是统一业务入口：服务端 Web 走 `DirectBackend`，桌面端和移动端走 API Proxy。Gin 只做对外 HTTP API 适配。
- 各端统一使用 Bearer Token，不使用 Cookie。登录令牌保存在 `localStorage`，API Proxy 把应用服务调用转成携带 Token 的 HTTP 请求。唯一例外是网站匿名访客：公开 Messenger 使用渠道级长期 Cookie 恢复匿名身份，见 `chat-roadmap.md` 第 4.5 节。
- 企业初始化只在 Web 端完成。桌面端和移动端先检测企业服务器并确认企业名称，再连接并进入登录页；登录页展示已连接企业名称并可更换地址。读不到企业名称时，桌面端和移动端回到连接页，Web 端回到初始化页。
- Web 与桌面端共享主要业务页面，移动端保持独立入口。
- 对象存储开启时，文件由客户端通过服务端签发的预签名请求直传，服务端不转发文件内容，Endpoint 使用客户端可访问的公开地址；对象存储关闭时，文件写入企业服务器的本地最终目录。文件选择后立即上传为临时文件，保存业务数据时在事务中激活；未激活文件默认 24 小时过期，由服务端定时清理。文件读取按记录中的本地或对象存储类型处理，不受当前开关影响。

## 协作语言

- 代码审查结果、Git 提交信息以及 PR 的标题和描述使用中文。

## Pi CLI 审核

- 提交 PR 前，必须调用 Pi CLI 审核并处理反馈，每一次小的改动不需要审核，最终审核 pr 时主要是检查是否有更优雅的实现方式，以及针对本次改动，是否有啰嗦的注释、反向引用、无必要的防御性逻辑和历史兼容逻辑、注释与实现不一致的问题，以及是否要加上必要的直述型注释和 `INFO`、`WARN` 等日志。
- Pi CLI 已配置默认模型，无需指定；运行可能较慢，必须耐心等待完整结果。

## 前端开发约定

本节适用于 `frontend/` 目录，并继承本文前面的项目级约定。

### 运行与验证

前端要求 Node.js 24.0.0 或更高版本。命令仍从仓库根目录执行。

```bash
# 桌面端开发
wails3 task dev
wails3 task dev:mcp

# 移动端依赖与运行
wails3 task ios:install:deps
wails3 task android:install:deps
wails3 task ios:run
wails3 task android:run
wails3 task android:run:device

# 绑定与前端生产构建
wails3 generate bindings -clean=true -ts -i
wails3 task common:build:frontend
```

需要使用桌面 MCP 时通过 `wails3 task dev:mcp` 启动；普通 `dev` 不启用 MCP。

### 代码组织

```text
frontend/
├── bindings/                       # Wails 自动生成的 TypeScript 绑定
└── src/
    ├── api/                        # 认证与按业务域拆分的绑定调用和边界归一化
    ├── apps/
    │   ├── shared-app-routes.tsx   # Web 与桌面端共用的业务路由
    │   ├── web/                    # Web 应用入口和路由
    │   ├── desktop/                # 桌面端应用入口和路由
    │   └── mobile/                 # 移动端独立入口、路由和页面
    ├── components/                 # 跨 feature 共享的展示组件
    │   ├── form/                   # 通用表单展示组件
    │   └── ui/                     # 基础 UI 组件
    ├── contexts/                   # 跨 feature 共享的 React 上下文
    ├── features/                   # Web 与桌面端共用的业务功能，按业务域分目录
    ├── hooks/                      # 通用 hooks，含统一数据读取 useResource
    ├── i18n/                       # 国际化资源，按语言目录和 namespace 分文件
    ├── lib/                        # 通用纯函数工具
    └── platform/                   # 运行平台识别
```

- `src/api` 按业务域一个文件组织，页面统一从 `@/api` 聚合入口导入；共享归一化工具放 `api/normalize.ts`。
- 页面归其路由所属的 feature；被多个 feature 使用的展示组件放 `src/components`，被多个 feature 使用的上下文放 `src/contexts`，feature 私有的上下文和 hooks 留在各自目录内。features 之间不得形成循环依赖；`features/workspace` 作为路由中枢可以引用各 feature 的页面，其余 feature 不得反向引用 workspace。
- 一个例外：角色域的页面在 `features/roles`，由设置页外壳（`features/settings`）按路由引用；该依赖保持 settings → roles 单向。

### 业务契约

- `appservice` 契约是前端业务 DTO 的唯一来源。前端不得重复声明渠道、联系人、用户、收件箱、设置等业务模型和枚举，也不要提交 Wails `$zero`。
- `frontend/bindings` 使用 `wails3 generate bindings -clean=true -ts -i` 生成，禁止手工修改，也不得用不同格式覆盖。
- 页面只通过 `src/api` 调用绑定：`client` 注入认证与错误，`service` 绑定方法并归一化可空切片。页面不直接引用 `frontend/bindings`。
- 前端只保留表单值、组件 Props、页面状态、查询参数派生类型，以及对生成类型中可空切片的边界归一化类型。
- 页面卸载时忽略过期结果，不要取消 Wails 绑定调用。

### 数据读取

- 页面数据读取统一使用 `src/hooks/use-resource.ts` 的 `useResource`（基于 TanStack Query），不再手写 `useEffect` 加过期标志的取数样板。查询 key 统一在 `src/hooks/resource-keys.ts` 的 `resourceKeys` 工厂中定义，页面不得手写 key 数组；同一份后端数据在不同页面使用相同 key 以共享缓存，查询参数变化必须体现在 key 中。
- 读取错误由 `useResource` 统一做会话入口恢复；变更操作直接调用 `@/api`，成功后通过 `refresh` 或 `useResourceInvalidator` 失效相关 key，不手工修补缓存数据。
- 会话引导流程（启动探测、身份加载）保持独立实现，不强制走 `useResource`。

### 路由

- `react-router` 在 `package.json` 中锁定精确版本。其 `UNSAFE_` 内部 API 只允许出现在 `src/features/workspace/tab-scoped-router.tsx`；升级 react-router 前必须先验证该模块行为未变化。

### 表单

- 表单使用 React Hook Form 和 Zod，并统一启用 `shouldUseNativeValidation`。客户端字段校验由浏览器显示在校验失败的输入控件上，不在字段下方渲染 `FieldError`，也不同时弹出 Toast；服务端业务错误通过 Toast 展示，不使用 `setError` 回写字段。
- 桌面端 WebView 中，带 `legend` 的原生 `fieldset`（包括 `FieldSet`）不得作为 flex 容器的直接子项，避免 WebKit 首次布局保留额外高度。此类表单组使用单列 grid，或在 `fieldset` 外增加普通块级容器；不得依赖点击、窗口缩放等重绘行为恢复布局。
- 输入框不使用 placeholder；字段含义由标签表达，必要说明用字段帮助文案。
- 业务上必填的表单字段，其可见标签统一使用红色 `*` 标记，优先使用 `FieldLabel required`；复选框组、表格列和详情编辑行等非标准字段使用等效标记。未标记即表示选填，标签和帮助文案不再额外添加“选填”或“可选”说明；条件必填字段只在条件成立时标记。
- 页面表单将字段区与底部操作区分为同级布局区域；保存、取消、测试等操作所在区域与最后一个表单项的垂直间距固定为 36px，统一使用 `space-y-9`。操作区不得放入 `FieldGroup`，也不得叠加额外外边距改变该间距。

### 注释

- `src` 业务代码的文件头说明职责，具名函数、组件和导出函数各使用一行简洁、直述型中文注释。
- `bindings` 禁止手工添加注释；`components/ui` 只保留文件头。
- 开发前端 TypeScript 或 JavaScript 代码时，不为仅在一处使用且实现不超过 10 行的逻辑新增私有辅助函数；优先在调用处直接实现，并在该段逻辑前添加一行简洁、直述型中文注释。100 行是函数或组件规模的参考线而非硬限制；就地实现后仅略微超过时（例如约 102 行）仍可接受，明显增加阅读负担时才拆分为具名辅助函数或组件。

### 界面控制

- 当前任务涉及桌面端时，必要时应主动使用 Wails MCP 获取页面信息并完成相关验证，无需另行请求授权。
- 除桌面端任务中的 Wails MCP 验证外，未经用户当次明确授权，不得控制浏览器、桌面应用或系统界面。
- 默认使用命令行验证；需要界面验证时，由用户操作并反馈结果，授权不得跨任务沿用。

### 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白。工作台一级导航为固定窄轨，二级栏显示模块标题（消息页除外）。不使用面包屑；需要扩展的设置页和渠道编辑页使用与 URL 同步的页签，不展示空页签。
- 新增、编辑和设置等表单页的标题栏不展示「返回」或其他返回操作；用户通过底部「取消」、页签、二级导航或导航历史离开。非表单详情子页确需显式返回列表时，在标题行右上角放置文字「返回」。
- 操作过程中优先保持当前页面的布局、内容位置和浏览上下文稳定，尽量避免因替换主体 DOM、改变区域尺寸或插入临时内容，使既有内容突然移动、跳动或被挤压。局部交互不会引起明显布局变化时，可以直接在当前页面完成。
- 根据任务复杂度和可用空间选择当前页面、Dialog、Sheet 或独立路由，不把某类操作固定绑定到单一载体。使用 Dialog 或 Sheet 时保持底层页面状态，关闭后尽量恢复原页签、列表选择、滚动位置和触发焦点；独立路由返回时恢复仍有意义的页面状态。
- 连续操作应让用户始终清楚当前任务和返回位置。需要切换视图或分步骤完成时，避免无提示地替换当前内容，并控制浮层层级，防止用户丢失原有上下文。
- 设置页同一分组内的字段保持统一的表单行样式；权限状态等字段不得单独使用带边框、圆角和内边距的卡片包裹，应与相邻字段的标签、帮助文案和控件对齐。
- 数据列表使用带表头的表格，字段独立成列且只展示有管理价值的信息，避免卡片式字段聚合。
- 操作按主次排序；主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。
- 列表操作列中直接展示的详情、编辑、恢复等文字操作统一使用小尺寸描边按钮。
- 操作按钮以文字为主，不添加装饰性图标；图标仅用于导航、类型、状态和三点菜单。
- 界面文案保持简洁，面向最终使用者描述操作、结果和影响；只保留必要的标题、标签、校验、状态和风险提示，不用解释数据结构或实现方式的说明式文案。

## 后端开发约定

本节适用于 `internal/` 目录，并继承本文前面的项目级约定。

### 运行与验证

命令从仓库根目录执行。PostgreSQL 和 NATS 由所有工作区共享实例，每个 worktree 通过 `.env` 使用独立数据库和 NATS 命名空间。

```bash
# 数据库
wails3 task db:ensure
wails3 task migrate
wails3 task migrate:status
wails3 task migrate:rollback
wails3 task migrate:rollback STEP=3
wails3 task migrate:rollback VERSION=20260818032701
wails3 task migrate:reset
wails3 task make:migration NAME=create_example_table

# 服务端运行与测试
wails3 task run:server
wails3 task test:server
wails3 task test:desktop
wails3 task test

# 服务端与容器构建
wails3 task build:server
wails3 task build:docker CGO_ENABLED=0
```

- `test:server` 根据当前 worktree 的 `POSTGRES_DB` 使用 `<POSTGRES_DB>_test` 作为测试数据库，并在每次运行前重建；同一 worktree 同一时刻运行一次 `test:server`。
- 服务端集成测试共享当前 worktree 的测试数据库。企业安装测试使用本轮新建的空数据库；其他集成测试通过唯一业务键或测试清理保持数据隔离。

### 代码组织

```text
internal/
├── actions/                        # 按领域组织的 Action 与 Query
├── api/                            # Gin 对外 HTTP API 适配器
├── apiproxy/                       # 原生端到企业服务端的类型化 API 代理
├── appservice/                     # 跨平台应用服务、传输契约和平台 Backend
│   └── native/                     # 原生端应用服务平台能力实现
├── clientsession/                  # 原生端当前登录凭据管理
├── common/                         # 无存储、无传输、无平台依赖的通用能力
├── config/
│   └── server/                     # 企业服务端运行配置加载与校验
├── domain/                         # 各层共用的领域值
├── i18n/                           # 后端本地化能力和翻译词条
├── ingress/                        # 企业服务端 HTTPS 与公网流量入口
├── integration/
│   ├── connectiontest/             # 外部连接探测的通用执行语义
│   ├── connector/                  # 外部系统连接器只读探测
│   └── modelprovider/              # 模型服务供应商连接探测适配器
├── publicweb/                      # 网站渠道公开嵌入脚本和访客聊天页
├── storage/
│   ├── server/                     # PostgreSQL 连接、迁移、服务端模型和存储适配器
│   ├── desktop/                    # 桌面端 SQLite 存储、迁移和模型
│   └── mobile/                     # 移动端 SQLite 存储、迁移和模型
├── task/                           # 可靠任务能力
│   ├── task.go                     # 跨平台共享的最小执行语义
│   ├── client/                     # 客户端 SQLite 可靠任务方案与实现
│   └── server/                     # 服务端 PostgreSQL、NATS 与 Cron 实现
└── tools/
    └── appservicegen/              # 从 Backend 接口生成三层适配样板
```

### 分层

- `appservice.Service` 是统一业务入口。Gin 只做对外 HTTP API 适配，输出 Backend 给出的状态和错误体，不定义前端业务类型和主要调用契约。
- `appservice/backend.go` 中的 `Backend` 接口是业务调用的唯一契约源：每个方法必须携带 `cervi:route` 指令；`Service` 委托、Gin 路由与 Handler、API Proxy 转发由 `go generate ./internal/appservice` 统一生成到各包的 `*_gen.go`，禁止手改生成文件。
- 新增业务方法的步骤：在 `Backend` 接口补方法与指令（GET 的查询结构体在 `types.go` 为每个字段显式加 `query` 标签，不传输的字段使用 `query:"-"`），运行生成器，然后只手写 `DirectBackend` 实现和 Action。无法按统一模式生成的层用 `manual=service,api,proxy` 标记并在对应包手写；API Proxy 的响应归一化在 `normalizeOutput` 中按类型补分支。
- `DirectBackend` 负责认证，并把 Action 返回的语言无关错误码转成结构化、本地化错误，再调用 Action。
- 只读 Query 信任 `DirectBackend` 已解析的当前身份，不重复查询用户状态；写 Action 在事务开始时通过 `actions/identity.LockActiveUser` 校验并锁定活跃用户账号。
- Action 直接使用 Bun，按需调用 `common`；记录关联、组织边界和业务规则在事务中显式校验和维护。
- `common` 只放无数据库、无传输层、无平台依赖的通用能力。小函数和错误放在包内，完整能力使用子包。
- `domain` 只放各层共用的领域值，按概念拆文件，不放数据库、传输层和平台逻辑。
- 服务端 PostgreSQL 模型放在 `storage/server`，桌面端 SQLite 模型放在 `storage/desktop`，移动端 SQLite 模型放在 `storage/mobile`；桌面端和移动端的 SQLite 迁移保持独立。
- `task` 根包只放各平台共享的 Action 执行语义；客户端和服务端分别定义自己的投递参数、存储与运行机制，不为形式统一互相复用平台实现。

### 数据与迁移

- 本地不运行 S3 兼容服务；对象存储由管理页面配置，可使用任意客户端可访问的临时 S3 兼容服务。
- 重建库结构使用 `wails3 task migrate:reset`，或先回滚再 `migrate`；回滚和重建前先停止服务端。
- 已合入 `main` 的迁移不可修改、重命名或重排；后续结构变化必须新增时间戳更晚的增量迁移，并提供对应的 Down 迁移。
- 同一未合并 PR 内调整数据结构时，直接修改或合并该结构对应且尚未合入 `main` 的迁移，只保留目标最终结构；不得为 PR 内已放弃的中间方案追加过渡、修正或清理迁移。本地已执行旧版本时，手动调整开发库或迁移记录并重新验证，禁止把这种一次性修正写入产品迁移。
- 服务端新迁移文件的 `YYYYMMDDHHMMSS` 必须取创建文件时的本地实际时间并精确到秒，统一通过 `wails3 task make:migration NAME=<name>` 生成；禁止手工编造整点、整分、日期或序号占位时间戳，即使分钟或秒为 `00` 也必须来自命令运行时的真实时间。
- 建表迁移按 `YYYYMMDDHHMMSS_create_<table>_table.sql` 命名，每个文件只创建一张表，不创建外键和 `CHECK` 约束。
- 迁移中使用简洁中文 `COMMENT ON` 说明表和业务字段。

### 当前阶段

- 不考虑历史数据和旧接口兼容。改模型、迁移和接口时直接实现目标结构，不写旧数据回填、缺失记录兜底或双版本逻辑，除非任务明确要求。
- 密钥加密存储、接口响应脱敏等安全加固暂不阻塞开发和审查。
- 角色权限后续统一建设；当前只校验已登录，不按管理员或普通成员限制功能。

### 注释

- Go 具名函数和方法使用简洁、直述型中文注释。
- 开发后端 Go 代码时，不为仅在一处使用且实现不超过 10 行的逻辑新增私有辅助函数；优先在调用处直接实现，并在该段逻辑前添加一行简洁、直述型中文注释。100 行是函数体规模的参考线而非硬限制；就地实现后仅略微超过时（例如约 102 行）仍可接受，明显增加阅读负担时才拆分为具名辅助函数。
