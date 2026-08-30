# 后端开发约定

本文件适用于 `internal/` 目录，并继承根目录 `AGENTS.md` 的项目级约定。

## 运行与验证

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
go test ./...
go test -tags server ./...

# 服务端与容器构建
wails3 task build:server
wails3 task build:docker CGO_ENABLED=0
```

## 代码组织

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
├── integration/
│   ├── connectiontest/             # 外部连接探测的通用执行语义
│   ├── connector/                  # 外部系统连接器只读探测
│   └── modelprovider/              # 模型服务供应商连接探测适配器
├── publicweb/                      # 网站渠道公开嵌入脚本和访客聊天页
├── storage/
│   ├── server/                     # PostgreSQL 连接、迁移和服务端模型
│   ├── desktop/                    # 桌面端 SQLite 存储、迁移和模型
│   └── mobile/                     # 移动端 SQLite 存储、迁移和模型
├── task/                           # 可靠任务能力
│   ├── task.go                     # 跨平台共享的最小执行语义
│   ├── client/                     # 客户端 SQLite 可靠任务方案与实现
│   └── server/                     # 服务端 PostgreSQL、NATS 与 Cron 实现
└── tools/
    └── appservicegen/              # 从 Backend 接口生成三层适配样板
```

## 分层

- `appservice.Service` 是统一业务入口。Gin 只做对外 HTTP API 适配，输出 Backend 给出的状态和错误体，不定义前端业务类型和主要调用契约。
- `appservice/backend.go` 中的 `Backend` 接口是业务调用的唯一契约源：每个方法必须携带 `cervi:route` 指令；`Service` 委托、Gin 路由与 Handler、API Proxy 转发由 `go generate ./internal/appservice` 统一生成到各包的 `*_gen.go`，禁止手改生成文件。
- 新增业务方法的步骤：在 `Backend` 接口补方法与指令（GET 的查询结构体在 `types.go` 为每个字段显式加 `query` 标签，不传输的字段使用 `query:"-"`），运行生成器，然后只手写 `DirectBackend` 实现和 Action。无法按统一模式生成的层用 `manual=service,api,proxy` 标记并在对应包手写；API Proxy 的响应归一化在 `normalizeOutput` 中按类型补分支。
- `DirectBackend` 负责认证，并把 Action 返回的语言无关错误码转成结构化、本地化错误，再调用 Action。
- Action 直接使用 Bun，按需调用 `common`；记录关联、组织边界和业务规则在事务中显式校验和维护。
- `common` 只放无数据库、无传输层、无平台依赖的通用能力。小函数和错误放在包内，完整能力使用子包。
- `domain` 只放各层共用的领域值，按概念拆文件，不放数据库、传输层和平台逻辑。
- 服务端 PostgreSQL 模型放在 `storage/server`，桌面端 SQLite 模型放在 `storage/desktop`，移动端 SQLite 模型放在 `storage/mobile`；桌面端和移动端的 SQLite 迁移保持独立。
- `task` 根包只放各平台共享的 Action 执行语义；客户端和服务端分别定义自己的投递参数、存储与运行机制，不为形式统一互相复用平台实现。

## 数据与迁移

- 本地不运行 S3 兼容服务；对象存储由管理页面配置，可使用任意客户端可访问的临时 S3 兼容服务。
- 重建库结构使用 `wails3 task migrate:reset`，或先回滚再 `migrate`；回滚和重建前先停止服务端。
- 已合入 `main` 的迁移不可修改、重命名或重排；后续结构变化必须新增时间戳更晚的增量迁移，并提供对应的 Down 迁移。
- 同一未合并 PR 内调整数据结构时，直接修改或合并该结构对应且尚未合入 `main` 的迁移，只保留目标最终结构；不得为 PR 内已放弃的中间方案追加过渡、修正或清理迁移。本地已执行旧版本时，手动调整开发库或迁移记录并重新验证，禁止把这种一次性修正写入产品迁移。
- 服务端新迁移文件的 `YYYYMMDDHHMMSS` 必须取创建文件时的本地实际时间并精确到秒，统一通过 `wails3 task make:migration NAME=<name>` 生成；禁止手工编造整点、整分、日期或序号占位时间戳，即使分钟或秒为 `00` 也必须来自命令运行时的真实时间。
- 建表迁移按 `YYYYMMDDHHMMSS_create_<table>_table.sql` 命名，每个文件只创建一张表，不创建外键和 `CHECK` 约束。
- 迁移中使用简洁中文 `COMMENT ON` 说明表和业务字段。

## 当前阶段

- 不考虑历史数据和旧接口兼容。改模型、迁移和接口时直接实现目标结构，不写旧数据回填、缺失记录兜底或双版本逻辑，除非任务明确要求。
- 密钥加密存储、接口响应脱敏等安全加固暂不阻塞开发和审查。
- 角色权限后续统一建设；当前只校验已登录，不按管理员或普通成员限制功能。

## 注释

- Go 具名函数和方法使用简洁、直述型中文注释。
