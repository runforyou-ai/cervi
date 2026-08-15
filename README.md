# Cervi

Cervi 是一个面向 Web、桌面端和移动端的开源 AI 客服协作平台。项目使用 Wails 3、Go、React、Vite 和 shadcn/ui。

## 本地开发

环境要求：Go 1.25.7、Node.js 22.22 或更高版本、Wails 3 CLI。

```bash
wails3 dev
```

前端也可以独立运行：

```bash
cd frontend
npm install
npm run dev
```

## 构建与检查

```bash
wails3 build
npm --prefix frontend run build
go test ./...
go test -tags server ./...
```

Server 模式使用 PostgreSQL。本地数据库和服务可通过以下命令启动：

```bash
docker compose up -d postgres
wails3 task run:server
```
