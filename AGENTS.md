# Cervi（鹿行）

## 项目背景

Cervi 是一款开源、以私有化部署为主的 AI 原生企业协作产品。项目使用 Go、Wails v3 和 React 开发，同一套代码支持服务端、Web、桌面端和移动端。

## 运行与构建

前端要求 Node.js 24.0.0 或更高版本。所有命令从仓库根目录通过 Wails v3 和 Task 执行；Task 自动加载当前 worktree 的 `.env`。

```bash
# 环境与共享依赖
cp .env.example .env
docker compose up -d postgres nats

# 数据库
wails3 task db:ensure
wails3 task migrate
wails3 task migrate:status
wails3 task migrate:rollback
wails3 task migrate:rollback STEP=3
wails3 task migrate:rollback VERSION=20260818032701
wails3 task migrate:reset
wails3 task make:migration NAME=create_example_table

# 服务端与桌面端
wails3 task run:server
wails3 task dev

# 移动端运行
wails3 task ios:install:deps
wails3 task android:install:deps
wails3 task ios:run
wails3 task android:run
wails3 task android:run:device

# 测试与绑定
go test ./...
go test -tags server ./...
wails3 generate bindings -clean=true -ts -i

# 当前宿主平台的桌面端构建与打包
wails3 task build
wails3 task package

# Web、服务端与容器
wails3 task common:build:frontend
wails3 task build:server
wails3 task build:docker CGO_ENABLED=0

# 各平台构建与打包
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

# 交叉编译准备；之后照常调用目标平台的 *:build Task
wails3 task setup:docker
```

- 每个 worktree 使用独立的 Server、Vite 端口、PostgreSQL 数据库和 NATS 命名空间；PostgreSQL 和 NATS 仅在主工作区共享启动。桌面端启动后，桌面 MCP 按 Wails3 默认配置可用。
- 客户端统一使用平台 Task，目标架构只传 `ARCH`，不要自行调用底层构建工具或设置 `GOOS`、`GOARCH`、`CGO_ENABLED`。客户端固定启用 CGO；纯静态服务端镜像使用 `CGO_ENABLED=0`。
- `darwin:build`、`windows:build` 和 `linux:build` 会按宿主环境选择原生或 Docker 工具链。交叉编译只生成二进制或未签名应用包；正式桌面安装包在目标系统或同平台 Runner 构建。Linux 不支持异架构桌面端交叉编译，Windows 宿主不支持交叉构建其他平台，WSL 按 Linux 处理。
- macOS 的 DMG、签名和公证在 macOS 完成；Windows 安装包在原生 Windows 或 Windows Runner 完成；iOS 在 macOS 完成。服务端多平台归档由 `.github/workflows/release.yml` 的 `server-assets` 作业生成。
- 仅当用户明确要求提交代码或提交 PR 时，才运行与任务相关的测试和构建；其他情况不运行测试、`go vet` 或构建。

真机在连接页手动输入可访问的企业服务端地址。Cloudflare Tunnel 由 Dashboard 管理路由，本机使用 `~/.cloudflared/cervi-dev.token` 启动一份 connector。

- `go.mod` 中的 `github.com/wailsapp/wails/v3`、`frontend/package.json` 中的 `@wailsio/runtime` 与本机 `wails3` CLI 必须使用同一精确版本；前端运行时禁止使用 `latest`、`^` 或 `~` 范围。按 `go.mod` 安装 CLI：`go install github.com/wailsapp/wails/v3/cmd/wails3@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)`。
- 升级 Wails 时先阅读目标版本的官方发布说明，再使用目标版本 CLI 在临时目录生成 React 脚手架，对比官方模板和 `build-assets`。`build/` 已包含服务端、桌面端和移动端定制，不得直接覆盖；只人工合并与当前项目相关的官方变更。
- Wails 升级后重新生成绑定，并验证前端构建、Go 测试、服务端构建和当前平台原生端构建；涉及移动端脚手架时同时验证 Android 与 iOS 构建配置。

## 代码组织

```text
cervi/
├── main.go                         # 应用入口和 Wails 配置
├── application_services_*.go       # 按原生端和服务端注册服务
├── internal/
│   ├── actions/                    # 按领域组织的 Action 与 Query
│   ├── api/                        # Gin 对外 HTTP API 适配器
│   ├── apiproxy/                   # 原生端到企业服务端的类型化 API 代理
│   ├── appservice/                 # 跨平台应用服务、传输契约和平台 Backend
│   │   └── native/                 # 原生端应用服务平台能力实现
│   ├── common/                     # 无存储、无传输、无平台依赖的通用能力
│   ├── domain/                     # 各层共用的领域值
│   ├── i18n/                       # 后端本地化能力和翻译词条
│   ├── storage/
│   │   ├── server/                 # PostgreSQL 连接、迁移和服务端模型
│   │   ├── desktop/                # 桌面端 SQLite 存储、迁移和模型
│   │   └── mobile/                 # 移动端 SQLite 存储、迁移和模型
│   └── task/                       # 可靠任务能力
│       ├── task.go                 # 跨平台共享的最小执行语义
│       ├── client/                 # 客户端 SQLite 可靠任务方案与实现
│       └── server/                 # 服务端 PostgreSQL、NATS 与 Cron 实现
├── frontend/
│   ├── bindings/                   # Wails 自动生成的 TypeScript 绑定
│   └── src/
│       ├── api/                    # 认证、绑定调用和边界归一化
│       ├── apps/
│       │   ├── web/                # Web 应用入口和路由
│       │   ├── desktop/            # 桌面端应用入口和路由
│       │   └── mobile/             # 移动端独立页面
│       ├── components/form/        # 通用表单展示组件
│       ├── features/               # Web 与桌面端共用的业务功能
│       ├── i18n/                   # 国际化资源
│       └── platform/               # 运行平台识别
└── build/                          # Wails 多平台构建配置
```

## 开发约定

### 分层

- `appservice.Service` 是统一业务入口：服务端 Web 走 `DirectBackend`，桌面端和移动端走 API Proxy。Gin 只做对外 HTTP 适配，输出 Backend 给出的状态和错误体，不定义前端业务类型和主要调用契约。
- `DirectBackend` 负责认证，并把 Action 返回的语言无关错误码转成结构化、本地化错误，再调用 Action。
- Action 直接使用 Bun，按需调用 `common`；记录关联、组织边界和业务规则在事务中显式校验和维护。
- `common` 只放无数据库、无传输层、无平台依赖的通用能力。小函数和错误放在包内，完整能力使用子包。
- `domain` 只放各层共用的领域值，按概念拆文件，不放数据库、传输层和平台逻辑。
- 服务端 PostgreSQL 模型放在 `storage/server`，桌面端 SQLite 模型放在 `storage/desktop`，移动端 SQLite 模型放在 `storage/mobile`；桌面端和移动端的 SQLite 迁移保持独立。
- `task` 根包只放各平台共享的 Action 执行语义；客户端和服务端分别定义自己的投递参数、存储与运行机制，不为形式统一互相复用平台实现。

### 前端契约

- `appservice` 契约是前端业务 DTO 的唯一来源。前端不得重复声明渠道、联系人、用户、收件箱、设置等业务模型和枚举，也不要提交 Wails `$zero`。
- `frontend/bindings` 使用 `wails3 generate bindings -clean=true -ts -i` 生成，禁止手工修改，也不得用不同格式覆盖。
- 页面只通过 `src/api` 调用绑定：`client` 注入认证与错误，`service` 绑定方法并归一化可空切片。页面不直接引用 `frontend/bindings`。
- 前端只保留表单值、组件 Props、页面状态、查询参数派生类型，以及对生成类型中可空切片的边界归一化类型。
- 表单使用 React Hook Form 和 Zod，并统一启用 `shouldUseNativeValidation`。客户端字段校验由浏览器显示在校验失败的输入控件上，不在字段下方渲染 `FieldError`，也不同时弹出 Toast；服务端业务错误通过 Toast 展示，不使用 `setError` 回写字段。
- 桌面端 WebView 中，带 `legend` 的原生 `fieldset`（包括 `FieldSet`）不得作为 flex 容器的直接子项，避免 WebKit 首次布局保留额外高度。此类表单组使用单列 grid，或在 `fieldset` 外增加普通块级容器；不得依赖点击、窗口缩放等重绘行为恢复布局。
- 输入框不使用 placeholder；字段含义由标签表达，必要说明用字段帮助文案。
- 页面卸载时忽略过期结果，不要取消 Wails 绑定调用。

### 认证与多端

- 各端统一使用 Bearer Token，不使用 Cookie。登录令牌保存在 `localStorage`。API Proxy 把应用服务调用转成携带 Token 的 HTTP 请求。
- 企业初始化只在 Web 端完成。桌面端和移动端先检测企业服务器并确认企业名称，再连接并进入登录页；登录页可更换地址。
- 登录页展示已连接企业的名称；读不到时，桌面端和移动端回到连接页，Web 端回到初始化页。
- Web 与桌面端共享主要业务页面，移动端保持独立入口。
- 对象存储开启时，文件由客户端通过服务端签发的预签名请求直传，服务端不转发文件内容，Endpoint 使用客户端可访问的公开地址；对象存储关闭时，文件写入企业服务器的本地最终目录。文件选择后立即上传为临时文件，保存业务数据时在事务中激活；未激活文件默认 24 小时过期，由服务端定时清理。文件读取按记录中的本地或对象存储类型处理，不受当前开关影响。

### 数据与迁移

- 本地 PostgreSQL 和 NATS 实例由所有工作区共享，不为单独工作区创建容器、端口或数据卷；每个 worktree 必须通过 `.env` 使用独立数据库和 NATS 命名空间。
- 本地不运行 S3 兼容服务；对象存储由管理页面配置，可使用任意客户端可访问的临时 S3 兼容服务。
- 重建库结构使用 `wails3 task migrate:reset`，或先回滚再 `migrate`；回滚和重建前先停止服务端。
- 建表迁移按 `YYYYMMDDHHMMSS_create_<table>_table.sql` 命名，每个文件只创建一张表，不创建外键和 `CHECK` 约束。
- 迁移中使用简洁中文 `COMMENT ON` 说明表和业务字段。

### 当前阶段

- 不考虑历史数据和旧接口兼容。改模型、迁移和接口时直接实现目标结构，不写旧数据回填、缺失记录兜底或双版本逻辑，除非任务明确要求。
- 密钥加密存储、接口响应脱敏等安全加固暂不阻塞开发和审查。
- 角色权限后续统一建设；当前只校验已登录，不按管理员或普通成员限制功能。

### 注释与语言

- Go 具名函数和方法使用简洁、直述型中文注释。
- 前端 `src` 业务代码同样：文件头说明职责，具名函数、组件和导出函数各一行注释。`frontend/bindings` 禁止加注释；`components/ui` 只保留文件头。
- 代码审查结果、Git 提交信息以及 PR 的标题和描述使用中文。

### 界面控制

- 当前任务涉及桌面端时，必要时应主动使用 Wails MCP 获取页面信息并完成相关验证，无需另行请求授权。
- 除桌面端任务中的 Wails MCP 验证外，未经用户当次明确授权，不得控制浏览器、桌面应用或系统界面。
- 默认使用命令行验证；需要界面验证时，由用户操作并反馈结果，授权不得跨任务沿用。

### 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白。工作台一级导航为固定窄轨，二级栏显示模块标题（消息页除外）。不使用面包屑；离开列表的子页在标题行右上角放置文字「返回」。需要扩展的设置页和渠道编辑页使用与 URL 同步的页签，不展示空页签。
- 设置页同一分组内的字段保持统一的表单行样式；权限状态等字段不得单独使用带边框、圆角和内边距的卡片包裹，应与相邻字段的标签、帮助文案和控件对齐。
- 数据列表使用带表头的表格，字段独立成列且只展示有管理价值的信息，避免卡片式字段聚合。
- 操作按主次排序；主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。
- 列表操作列中直接展示的详情、编辑、恢复等文字操作统一使用小尺寸描边按钮。
- 操作按钮以文字为主，不添加装饰性图标；图标仅用于导航、类型、状态和三点菜单。
- 界面文案保持简洁，面向最终使用者描述操作、结果和影响；只保留必要的标题、标签、校验、状态和风险提示，不用解释数据结构或实现方式的说明式文案。
