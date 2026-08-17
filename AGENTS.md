# Cervi（鹿行）

## 项目背景

Cervi 是一款开源、以私有化部署为主的 AI 原生企业协作产品。项目使用 Go、Wails v3 和 React 开发，同一套代码支持服务端、Web、桌面端和移动端。

## 运行方式

主要开发命令：

```bash
# 启动本地 PostgreSQL
docker compose up -d postgres

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
```

前端要求 Node.js 22.22.0 或更高版本，项目构建使用 Wails v3 和 Task。

## 代码组织

```text
cervi/
├── main.go                         # 应用入口和 Wails 配置
├── application_service_apiproxy.go # 组装原生端共用的 API 代理
├── application_services_*.go      # 按服务端、桌面端和移动端注册服务
├── internal/
│   ├── actions/                    # 按领域组织的 Action 与 Query
│   ├── api/                        # 企业服务端 HTTP API
│   ├── apiproxy/                   # 原生端到企业服务端的 API 代理
│   ├── common/                     # 按能力组织的通用代码
│   ├── i18n/                       # 后端本地化能力和翻译词条
│   └── storage/
│       ├── server/                 # PostgreSQL 连接、迁移和服务端模型
│       ├── desktop/                # 桌面端 SQLite 存储和模型
│       └── mobile/                 # 移动端 SQLite 存储和模型
├── frontend/src/
│   ├── api/                        # 按领域组织的接口请求和请求客户端
│   ├── apps/
│   │   ├── web/                    # Web 应用入口和路由
│   │   ├── desktop/                # 桌面端应用入口和路由
│   │   └── mobile/                 # 移动端独立页面
│   ├── components/form/            # 通用表单展示组件
│   ├── features/                   # Web 与桌面端共用的业务功能
│   ├── i18n/                       # 国际化资源
│   └── platform/                   # 运行平台识别
└── build/                          # Wails 多平台构建配置
```

## 开发约定

- Go 具名函数和方法使用简洁、直述型中文注释。
- HTTP 请求进入 Action，Action 直接使用 Bun，并按需调用 `common` 中的通用能力。
- `common` 只放置无数据库、无传输层和无平台依赖的通用能力。
- 数据库模型放在对应平台的 `storage` 目录中。
- 数据库迁移不创建外键约束，记录关联和组织边界由 Action 在事务中显式校验和维护。
- 项目当前不考虑历史数据和旧接口兼容；修改模型、迁移和接口时直接实现目标结构，不编写旧数据回填、缺失记录兜底或双版本兼容逻辑，除非任务明确要求。
- Action 返回语言无关的错误码，API 层负责生成本地化文案。
- Web 与桌面端共享主要业务页面，移动端保持独立入口。
- 前端表单使用 React Hook Form、Zod 和统一错误展示组件。
- 代码审查（review）的结果统一使用中文输出。
- Git 提交信息以及 PR 的标题和描述统一使用中文表述。
- 修改后运行相关测试、静态检查和前端构建。

### 管理界面设计

- 管理页面采用左对齐的可用宽度布局并保持统一留白；子页面使用面包屑，需要扩展的设置页使用与 URL 同步的页签，不展示空页签。
- 数据列表使用带表头的表格，字段独立成列且只展示有管理价值的信息，避免卡片式字段聚合。
- 操作按主次排序；主要操作直接展示，低频或危险操作收进三点菜单，危险操作执行前确认。
- 操作按钮以文字为主，不添加装饰性图标；图标仅用于导航、类型、状态和三点菜单。
- 界面文案保持简洁，只保留必要的标题、标签、校验、状态和风险提示。
