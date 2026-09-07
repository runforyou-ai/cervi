# 服务端部署说明

## 交付范围

- Linux 使用 `tar.gz + systemd`，Windows Server 使用 ZIP，容器使用镜像，不单独维护 DEB、RPM 和 MSI。
- 服务端部署通过 `-config` 指定 YAML 配置文件。
- Windows SCM、`cervi install/start/stop/config` 和自动更新器暂未实现。

## 配置与诊断

YAML 中未显式配置的可选字段使用二进制内的默认值。

部署前使用以下命令校验配置：

```text
cervi-server -config <配置文件> -check-config
```

该命令校验 YAML 以及 PostgreSQL、NATS 和 TLS 配置，不连接外部服务。

部署 YAML 基础模板：

```yaml
server:
  host: 0.0.0.0
  port: 8080

database:
  host: 127.0.0.1
  port: 5432
  user: cervi
  password: 请替换密码
  name: main
  sslMode: disable

nats:
  url: nats://127.0.0.1:4222
  namespace: main

tls:
  mode: off
  acmeEmail: ""

storage:
  localDirectory: /var/lib/cervi/files
```

服务端依赖 PostgreSQL 和启用 JetStream 的 NATS。

## PostgreSQL 初始化

PostgreSQL 基线为 18，数据库镜像必须提供 `vector` 和 `pg_trgm` 扩展安装文件。仓库的 `build/docker/Dockerfile.postgres` 固定 pgvector 0.8.6、Bookworm 和多架构镜像摘要，包含两个扩展。发布工作流提供 `ghcr.io/runforyou-ai/cervi-postgres:<发行版本>`，支持 linux/amd64 和 linux/arm64。

主工作区构建并启动共享依赖：

```bash
wails3 task build:postgres
docker compose up -d --wait postgres nats
wails3 task migrate
```

每个工作区使用独立的 `POSTGRES_DB`，业务库和测试库分别初始化。Cervi 与后续 Hayhooks 共用同一实例、目标数据库和应用账号；业务表沿用原 schema，检索表由应用账号在 `haystack` schema 管理。

服务端在开放 HTTP 和后台任务前，持有当前数据库的事务级初始化锁，安装缺失扩展、创建 `haystack`，验证权限和实际向量/trigram 能力，再取得 Goose 迁移锁执行业务迁移。启动日志记录已安装扩展版本；重复启动不升级扩展、不移动 schema，也不修改账号 search_path。`pg_isready` 只表示 PostgreSQL 接受连接，应用就绪以 `/readyz` 为准。

仅准备数据库和执行业务迁移可以使用发行二进制，不需要源码或 Task：

```bash
./cervi-server -config cervi.yaml -migrate
```

`wails3 task migrate` 和服务端测试复用相同 Go 初始化入口。迁移回滚不会删除扩展和 `haystack`，检索表的生命周期独立管理。

应用账号不能安装扩展时，管理员在**每个目标数据库**执行随发行包提供的 SQL；两个 SQL 文件保持同目录：

```bash
psql -h <数据库主机> -U <管理员> -d <目标数据库> \
  -v cervi_role=<应用账号> -f database/prepare-admin.sql
```

该脚本安装两个扩展并授权 `public` 的 USAGE、`haystack` 的 USAGE/CREATE。应用账号还需原有业务 schema 的建表和业务表访问权限；建议由该账号拥有业务数据库及其业务表。正常运行时 Cervi 与 Hayhooks 均使用应用账号。管理员 SQL 同时包含在服务端容器的 `/database`，可通过 `docker cp <容器>:/database ./database` 取出。

缺少扩展安装文件时需更换镜像或在 PostgreSQL 主机安装扩展包；缺少权限时按启动错误执行管理员 SQL。已有扩展不在 `public` 时由管理员明确处理。初始化按实际能力校验，不以扩展版本字符串强制拒绝其他兼容版本。真实云数据库的扩展支持与权限取决于提供商，本次未进行真实云实例验收。

### 已有数据库更换镜像

从 Alpine 切换到 Bookworm 时，使用逻辑备份恢复到新卷，验证编码、排序规则、数据、序列和索引。不要直接将未经验证的旧数据目录挂载到不同发行版镜像。

先在独立容器和端口完成空库部署、备份恢复和业务测试。正式切换前阻止源库继续写入、终止原连接，完成最终备份后恢复至新卷；核验通过后再向客户端开放原端口。保留旧镜像、旧卷及备份用于回退。新实例重新接受写入后，回退需要另行迁移新增数据。

共享实例数据卷由主工作区 `.env` 的 `POSTGRES_VOLUME` 指定，端口由 `POSTGRES_PORT` 指定。其他工作区继续使用共享实例地址，不自行启动 PostgreSQL。数据库备份不包含本地或对象存储中的原始文件，文件存储路径应保持可用。

## Linux systemd

文件路径：

- 二进制：`/usr/local/bin/cervi-server`
- YAML：`/etc/cervi/cervi.yaml`
- 数据目录：`/var/lib/cervi`
- 服务定义：`/etc/systemd/system/cervi.service`

基础安装命令：

```bash
sudo useradd --system --home-dir /var/lib/cervi --create-home --shell /usr/sbin/nologin cervi
sudo install -o root -g root -m 0755 cervi-server /usr/local/bin/cervi-server
sudo install -d -o root -g cervi -m 0750 /etc/cervi
sudo install -o root -g cervi -m 0640 cervi.yaml /etc/cervi/cervi.yaml
sudo install -o root -g root -m 0644 cervi.service /etc/systemd/system/cervi.service
sudo systemctl daemon-reload
sudo systemctl enable --now cervi
curl --retry 60 --retry-delay 1 --retry-connrefused --fail http://127.0.0.1:8080/readyz
```

systemd 模板：

```systemd
[Unit]
Description=Cervi 企业协作服务端
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=cervi
Group=cervi
ExecStartPre=/usr/local/bin/cervi-server -config /etc/cervi/cervi.yaml -check-config
ExecStart=/usr/local/bin/cervi-server -config /etc/cervi/cervi.yaml
WorkingDirectory=/var/lib/cervi
Restart=on-failure
RestartSec=5s
TimeoutStopSec=30s

StateDirectory=cervi
StateDirectoryMode=0750

[Install]
WantedBy=multi-user.target
```

查看日志：

```bash
journalctl -u cervi -f
```

### Linux 自动 HTTPS

将基础模板中的 `tls.mode` 改为 `auto`。自动 HTTPS 的 ACME 账户、证书和临时验证数据统一保存在 PostgreSQL，`tls.acmeEmail` 可填写 ACME 联系邮箱。

`auto` 模式需要监听 80/443。将以下 drop-in 保存为 `/etc/systemd/system/cervi.service.d/auto-https.conf`：

```systemd
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

重新加载并重启：

```bash
sudo systemctl daemon-reload
sudo systemctl restart cervi
```

公网域名的 80/443 需转发到当前服务器。只有公网域名会申请证书；IP、`localhost`、无点主机名以及 `.localhost`、`.local`、`.internal`、`.home.arpa` 地址继续使用 HTTP。停用 `auto` 后删除 drop-in 并重新加载 systemd。

从 `off` 或 `external` 改为 `auto` 后需重启服务。已绑定企业访问地址的公网域名可在首次 HTTPS 访问时直接签发证书；未绑定的新域名必须先访问 HTTP 入口。每个服务进程在任意 3 小时内最多放行 40 个新证书签发尝试窗口，同一域名 1 分钟内的并发请求合并计数；有效期内的缓存证书和后台续期不占用该额度。

## Windows Server

文件路径：

- 二进制：`C:\Program Files\Cervi\cervi-server.exe`
- 配置：`C:\ProgramData\Cervi\cervi.yaml`
- 数据：`C:\ProgramData\Cervi\data\files`

存储目录配置：

```yaml
storage:
  localDirectory: 'C:\ProgramData\Cervi\data\files'
```

管理员 PowerShell 示例：

```powershell
New-Item -ItemType Directory -Force 'C:\Program Files\Cervi', 'C:\ProgramData\Cervi\data\files'
Copy-Item .\cervi-server.exe 'C:\Program Files\Cervi\cervi-server.exe'
Copy-Item .\cervi.yaml 'C:\ProgramData\Cervi\cervi.yaml'
& 'C:\Program Files\Cervi\cervi-server.exe' -config 'C:\ProgramData\Cervi\cervi.yaml' -check-config
& 'C:\Program Files\Cervi\cervi-server.exe' -config 'C:\ProgramData\Cervi\cervi.yaml'
```

另一终端运行：

```powershell
Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:8080/readyz'
```

Windows 默认使用 `off` TLS 模式，并以前台进程或服务包装器运行。

## 容器

只需持久化 `/data/files`；自动 TLS 数据保存在 PostgreSQL。`auto` 模式映射 80/443，外部终止 HTTPS 时使用 `external` 模式。

## 升级

手动升级流程：

1. 校验发布制品的签名或校验和；
2. 备份 PostgreSQL 和 YAML；
3. 更新 YAML 配置；
4. 使用待升级二进制执行 `-check-config`；
5. 停止服务，原子替换二进制并保留上一版本；
6. 启动服务并检查 `/readyz`。

数据库迁移随服务启动执行并允许乱序迁移。二进制回退不回退数据库结构。

`/readyz` 表示 PostgreSQL 可以响应请求。NATS 在服务启动时连接，运行期间断线由客户端自动重连，状态通过服务日志和 NATS 监控检查。

自动更新器负责下载并校验发布制品，再调用平台服务管理器替换、重启和回退。服务进程不自行更新。

## 2026-08-23 临时服务器验证

- 主机：`ecs-user@47.239.49.135`
- 域名：`test-https.runforyou.app`
- 环境：Ubuntu 26.04、x86_64、systemd、PostgreSQL 18、NATS Server 2.10.27 JetStream
- 已验证：Linux AMD64 静态构建、配置校验、数据库迁移、NATS 任务运行时、systemd 启停、自动 HTTPS、证书缓存和两个探针。
- 当前服务以 `cervi` 用户运行，8080 绑定回环地址，80/443 提供自动 HTTPS。
