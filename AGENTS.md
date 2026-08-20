# Cervi（鹿行）

## 项目背景

Cervi 是一款开源、以私有化部署为主的 AI 原生企业协作产品。项目使用 Go、Wails v3 和 React 开发，同一套代码支持服务端、Web、桌面端和移动端。

## 运行方式

主要开发命令：

```bash
# 启动本地 PostgreSQL
docker compose up -d postgres

# 服务端数据库迁移
wails3 task migrate
wails3 task migrate:status
wails3 task migrate:rollback
wails3 task migrate:rollback STEP=3
wails3 task migrate:rollback VERSION=20260818032701
wails3 task migrate:fresh
wails3 task make:migration NAME=create_example_table

# 启动桌面端开发环境
wails3 dev

# 启动服务端开发环境
wails3 task run:server

# 首次准备移动端开发依赖
wails3 task ios:install:deps
wails3 task android:install:deps

# 在 iOS 模拟器中运行
wails3 task ios:run

# 在 Android 模拟器中运行
wails3 task android:run

# 在 Android 真机中运行
wails3 task android:run:device

# 运行测试
go test ./...
go test -tags server ./...

# 重新生成前端类型绑定
wails3 generate bindings -clean=true -ts -i
```

前端要求 Node.js 22.22.0 或更高版本，项目构建使用 Wails v3 和 Task。

## 代码组织

```text
cervi/
├── main.go                         # 应用入口和 Wails 配置
├── application_services_*.go      # 按原生端和服务端注册服务
├── internal/
│   ├── actions/                    # 按领域组织的 Action 与 Query
│   ├── api/                        # Gin 对外 HTTP API 适配器
│   ├── apiproxy/                   # 原生端到企业服务端的类型化 API 代理
│   ├── appservice/                 # 跨平台应用服务、传输契约和平台 Backend
│   ├── common/                     # 按能力组织的通用代码
│   ├── domain/                     # Action 与 AppService 共用的领域枚举
│   ├── i18n/                       # 后端本地化能力和翻译词条
│   └── storage/
│       ├── server/                 # PostgreSQL 连接、迁移和服务端模型
│       ├── desktop/                # 桌面端 SQLite 存储、迁移和模型
│       └── mobile/                 # 移动端 SQLite 存储、迁移和模型
├── frontend/
│   ├── bindings/                   # Wails 自动生成的 TypeScript 服务和类型绑定
│   └── src/
│       ├── api/                    # 认证、绑定调用适配和边界归一化
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

- Go 具名函数和方法使用简洁、直述型中文注释。
- 前端 `src` 业务代码使用简洁、直述型中文注释：每个文件在文件头说明职责，每个具名函数、组件和导出函数用一行注释说明作用。`frontend/bindings` 由工具生成，禁止添加注释。`components/ui` 为 shadcn 组件，只保留文件头注释。
- `appservice.Service` 是统一业务入口：服务端 Web 直接调用 `DirectBackend`，桌面端和移动端通过 API Proxy 调用企业服务端，Gin 只提供对外 HTTP API。
- 服务端由 `DirectBackend` 完成认证、错误转换并调用 Action；Gin 不定义项目内前端业务类型和主要调用契约。
- Action 直接使用 Bun，并按需调用 `common` 中的通用能力。
- `common` 只放置无数据库、无传输层和无平台依赖的通用能力；体量小的函数和错误放在 `common` 包内，完整能力使用子包。
- `domain` 只放置 Action、AppService 和不同平台共同使用的领域值，按概念拆文件，不放置数据库、传输层和平台逻辑。
- `appservice` 契约是前端业务 DTO 的唯一来源；前端不得重复声明渠道、联系人、用户、收件箱、设置等业务模型和枚举。
- `frontend/bindings` 由 Wails 自动生成，禁止手工修改；所有平台统一使用 `wails3 generate bindings -ts -i`，不得生成不同格式后覆盖现有绑定。
- 前端通过 `src/api` 调用生成绑定：`client` 注入认证与错误，`service` 绑定方法并归一化可空切片；页面不直接引用 `frontend/bindings`。
- 前端只保留表单值、组件 Props、页面状态、查询参数派生类型，以及对生成类型中可空切片的边界归一化类型。
- 数据库模型放在对应平台的 `storage` 目录中。
- 本地 PostgreSQL Docker 数据库由所有工作区共享，不为单独工作区创建容器、端口或数据卷；需要重建数据库结构时使用 Goose 回滚并重新执行迁移。
- 数据库迁移文件按 `YYYYMMDDHHMMSS_说明.sql` 格式命名，并按表拆分。
- 数据库迁移不创建外键和 `CHECK` 约束，记录关联、组织边界和业务规则由 Action 在事务中显式校验和维护。
- 项目当前不考虑历史数据和旧接口兼容；修改模型、迁移和接口时直接实现目标结构，不编写旧数据回填、缺失记录兜底或双版本兼容逻辑，除非任务明确要求。
- 项目当前处于早期开发阶段，密钥加密存储、接口响应脱敏等安全加固项暂不作为开发和审查的阻塞项，后续进入安全加固阶段时统一完善。
- 角色权限体系将在后续阶段统一建设；当前仅校验用户已登录，不要求按所有者、管理员或普通成员角色限制功能访问。
- Action 返回语言无关的错误码；AppService Backend 将其转换为结构化、本地化错误，Gin 只负责输出对应的 HTTP 状态和错误体。
- Web、桌面端和移动端统一使用 Bearer Token 认证，不使用 Cookie；API Proxy 负责把应用服务调用转换为携带 Token 的 HTTP 请求。
- 企业初始化只在 Web 端完成；桌面端和移动端将企业服务器地址持久化到本地 SQLite，连接成功后进入登录页，并允许从登录页修改地址。
- Web 与桌面端共享主要业务页面，移动端保持独立入口。
- 文件由客户端通过服务端签发的预签名请求直传对象存储；服务端不转发文件内容，Endpoint 使用客户端可访问的公开地址。
- 前端表单使用 React Hook Form、Zod 和统一错误展示组件。
- 表单输入框不使用 placeholder；字段含义由标签表达，必要的填写说明使用字段帮助文案展示。
- 代码审查（review）的结果统一使用中文输出。
- Git 提交信息以及 PR 的标题和描述统一使用中文表述。
- 修改后运行相关测试、`go vet` 静态检查和前端构建；涉及应用服务、绑定或构建流程时同时验证 `wails3 build DEV=true` 和 `wails3 task build:server DEV=true`。

### 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白；子页面使用面包屑，需要扩展的设置页使用与 URL 同步的页签，不展示空页签。
- 数据列表使用带表头的表格，字段独立成列且只展示有管理价值的信息，避免卡片式字段聚合。
- 操作按主次排序；主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。
- 列表操作列中直接展示的详情、编辑、恢复等文字操作统一使用小尺寸描边按钮。
- 操作按钮以文字为主，不添加装饰性图标；图标仅用于导航、类型、状态和三点菜单。
- 界面文案保持简洁，只保留必要的标题、标签、校验、状态和风险提示。
- 界面文案面向最终使用者，描述用户操作、结果和影响；不使用解释数据结构、实现方式、内部状态或开发逻辑的注释说明式文案。
