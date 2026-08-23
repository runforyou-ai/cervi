# 服务端部署临时说明

> 本文记录服务端安装和升级方案，安装器稳定后再拆分为用户文档与发布资源。

## 当前决策

- 不为业务代码分别维护 DEB、RPM 和 MSI；基础交付物为服务端二进制、配置模板和平台服务定义。
- Linux 首期使用 `tar.gz + systemd`，Windows Server 首期使用 ZIP 前台运行，容器环境使用官方镜像。
- 本地开发继续由 Task 自动加载当前 worktree 的 `.env`，不受生产部署方式影响。
- 生产二进制不自动查找 `.env`，也不回退到开发数据库或相对数据目录。
- Windows Service Control Manager、`cervi install/start/stop/config` 管理命令和无人值守自动升级属于后续阶段。

## 配置与诊断

运行配置按以下优先级加载，后者覆盖前者：

1. 二进制内的默认值；
2. `-config` 指定的 YAML；
3. 环境变量。

部署前使用以下命令校验配置：

```text
cervi-server -config <配置文件> -check-config
```

生产 YAML 基础模板：

```yaml
server:
  host: 127.0.0.1
  port: 8080

database:
  maxOpenConnections: 25
  maxIdleConnections: 5
  connectionMaxLifetime: 30m
  connectionMaxIdleTime: 5m
  connectTimeout: 1m
  migrationTimeout: 10m

# 推荐由反向代理、负载均衡或 Tunnel 管理公网 HTTPS。
https:
  mode: external

storage:
  localDirectory: /var/lib/cervi/files
```

数据库连接和 NATS 身份通过受限环境文件或平台密钥管理能力注入，不写入通用 YAML：

```dotenv
POSTGRES_HOST=127.0.0.1
POSTGRES_PORT=5432
POSTGRES_USER=cervi
POSTGRES_PASSWORD=请替换密码
POSTGRES_DB=cervi
POSTGRES_SSLMODE=require
NATS_URL=nats://127.0.0.1:4222
NATS_NAMESPACE=cervi
```

## Linux systemd

安装路径：

- 二进制：`/usr/local/bin/cervi-server`
- YAML：`/etc/cervi/cervi.yaml`
- 环境文件：`/etc/cervi/cervi.env`，权限 `root:cervi 0640`
- 数据目录：`/var/lib/cervi`
- 服务定义：`/etc/systemd/system/cervi.service`

基础安装命令：

```bash
sudo useradd --system --home-dir /var/lib/cervi --create-home --shell /usr/sbin/nologin cervi
sudo install -o root -g root -m 0755 cervi-server /usr/local/bin/cervi-server
sudo install -d -o root -g cervi -m 0750 /etc/cervi
sudo install -o root -g cervi -m 0640 cervi.yaml /etc/cervi/cervi.yaml
sudo install -o root -g cervi -m 0640 cervi.env /etc/cervi/cervi.env
sudo install -o root -g root -m 0644 cervi.service /etc/systemd/system/cervi.service
sudo systemctl daemon-reload
sudo systemctl enable --now cervi
curl --fail http://127.0.0.1:8080/readyz
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
EnvironmentFile=/etc/cervi/cervi.env
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

日志由 journald 管理：

```bash
journalctl -u cervi -f
```

### Linux 自动 HTTPS

由 Cervi 自动签发证书时使用：

```yaml
https:
  mode: auto
  tlsDataDirectory: /var/lib/cervi/tls
```

自动 HTTPS 需要监听 80/443。服务仍以 `cervi` 用户运行，仅在启用 `auto` 时安装以下 systemd drop-in：

```systemd
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

drop-in 保存为 `/etc/systemd/system/cervi.service.d/auto-https.conf`，然后执行：

```bash
sudo systemctl daemon-reload
sudo systemctl restart cervi
```

公网域名的 80/443 必须转发到当前服务器。切回 `external` 或 `off` 后删除 drop-in 并重新加载 systemd。

## Windows Server

首期使用 ZIP，不要求 MSI。建议路径：

- 二进制：`C:\Program Files\Cervi\cervi-server.exe`
- 配置：`C:\ProgramData\Cervi\cervi.yaml`
- 数据：`C:\ProgramData\Cervi\data\files`

Windows YAML 与基础模板相同，只需把存储路径改为：

```yaml
storage:
  localDirectory: 'C:\ProgramData\Cervi\data\files'
```

管理员 PowerShell 示例：

```powershell
New-Item -ItemType Directory -Force 'C:\Program Files\Cervi', 'C:\ProgramData\Cervi\data\files'
Copy-Item .\cervi-server.exe 'C:\Program Files\Cervi\cervi-server.exe'
Copy-Item .\cervi.yaml 'C:\ProgramData\Cervi\cervi.yaml'
$env:POSTGRES_HOST = '127.0.0.1'
$env:POSTGRES_PORT = '5432'
$env:POSTGRES_USER = 'cervi'
$env:POSTGRES_PASSWORD = '请替换密码'
$env:POSTGRES_DB = 'cervi'
$env:POSTGRES_SSLMODE = 'require'
$env:NATS_URL = 'nats://127.0.0.1:4222'
$env:NATS_NAMESPACE = 'cervi'
& 'C:\Program Files\Cervi\cervi-server.exe' -config 'C:\ProgramData\Cervi\cervi.yaml' -check-config
& 'C:\Program Files\Cervi\cervi-server.exe' -config 'C:\ProgramData\Cervi\cervi.yaml'
```

另一终端使用 `Invoke-WebRequest -UseBasicParsing 'http://127.0.0.1:8080/readyz'` 检查就绪状态。服务端尚未接入 Windows SCM，首期前台运行或使用服务包装器托管。

## 容器

镜像持久化 `/data/files` 和 `/data/tls`。使用 `auto` HTTPS 时映射 80/443；由宿主机、负载均衡或反向代理终止 HTTPS 时使用 `external` 模式。

## 升级边界

手动升级流程：

1. 校验发布制品的签名或校验和；
2. 备份 PostgreSQL；
3. 停止服务；
4. 原子替换二进制并保留上一版本；
5. 启动服务并检查 `/readyz`。

数据库迁移随服务启动执行，生产和开发环境始终允许乱序迁移，不提供关闭开关。回退二进制不等于回退数据库结构，数据库回退需要单独决策。

后续自动更新由独立更新器完成：下载带签名的发布清单和制品，校验版本、平台、架构及摘要，再调用平台服务管理器替换并重启，保留旧二进制用于程序回退。服务进程本身不负责自更新。

## 2026-08-23 临时服务器验证

- 主机：`ecs-user@47.239.49.135`
- 域名：`test-https.runforyou.app`
- 环境：Ubuntu 26.04、x86_64、systemd、PostgreSQL 18
- 验证结果：生产构建、配置校验、数据库连接和迁移、systemd 启停与重启、HTTP 到 HTTPS 跳转、证书缓存恢复、`/healthz` 和 `/readyz` 均通过。
- 实测确认：非 root 的 systemd 服务在默认低端口策略下无法绑定 80/443；安装上述 `CAP_NET_BIND_SERVICE` drop-in 后通过。
- 当前临时服务以 `cervi` 用户运行，8080 仅绑定回环地址，80/443 提供自动 HTTPS。
