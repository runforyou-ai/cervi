# Cervi（鹿行）

## 项目背景

Cervi 是一款开源、以私有化部署为主的 AI 原生企业协作产品。项目使用 Go、Wails v3 和 React 开发，同一套代码支持服务端、Web、桌面端和移动端。

## 约定作用域

- 本文件只定义整个仓库共同遵循的项目级约定。
- 修改 `frontend/` 下的代码时，同时遵循[前端开发约定](frontend/AGENTS.md)。
- 修改 `internal/` 下的代码时，同时遵循[后端开发约定](internal/AGENTS.md)。
- 子目录中的 `AGENTS.md` 只补充该目录的专属约定；未覆盖的规则继续继承本文件。

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

- 输出方案和提交 PR 前，必须调用 Pi CLI 审核并处理反馈。
- 审核先判断当前方案或实现是否足够简洁、符合职责边界和现有约定，以及能否用更小、更直接的做法完成目标。有更小做法时给出可落地建议；已经足够简洁则明确说无需再拆。方案审核看职责边界、依赖方向和抽象层级；实现审核结合本次差异检查重复逻辑、过度抽象、无必要的防御性分支、历史兼容层、反向依赖、冗余注释与命名、控制流与实现是否一致。
- 正确性、回归风险和安全检查必须同时给出，但不得只做缺陷扫描。调用时说明任务目标、范围、约束和当前方案或差异，要求先评质量与替代做法，再列正确性问题；意见区分必须修改和可选建议，有具体位置时标明位置。
- Pi CLI 已配置默认模型，无需指定；运行可能较慢，必须耐心等待完整结果。
