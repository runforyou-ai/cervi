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
  dataDirectory: /var/lib/cervi/tls
  acmeEmail: ""

storage:
  localDirectory: /var/lib/cervi/files
```

服务端依赖 PostgreSQL 和启用 JetStream 的 NATS。

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

将基础模板中的 `tls.mode` 改为 `auto`。`tls.dataDirectory` 用于持久化证书，`tls.acmeEmail` 可填写 ACME 联系邮箱。

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

公网域名的 80/443 需转发到当前服务器。停用 `auto` 后删除 drop-in 并重新加载 systemd。

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

持久化 `/data/files` 和 `/data/tls`。`auto` 模式映射 80/443，外部终止 HTTPS 时使用 `external` 模式。

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
