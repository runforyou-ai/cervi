# 外部连接测试架构

## 目标

连接测试用于验证一份尚未保存的外部服务配置是否可用。对象存储、模型服务、Dify、n8n、翻译供应商和 MCP 共用执行语义，但保留各自的强类型业务契约和探测方式。

## 分层

```text
appservice 强类型输入
        │
        ▼
领域 Action：校验、规范化、选择适配器
        │
        ▼
integration 适配器：执行供应商或协议专属探测
        │
        ▼
connectiontest Runner：超时、取消、错误分类和安全日志
```

- `appservice` 对每类集成分别提供输入 DTO，例如 `AIProviderConnectionInput` 和 `S3Setting`。不提供包含任意 JSON 配置的通用 `TestConnection` 接口。
- `internal/integration/connectiontest` 只定义 `Probe`、`Runner`、安全的 `Target` 以及稳定错误分类，不依赖 Eino、HTTP、S3 或具体业务配置。
- 每个集成适配器负责选择无副作用的最小探测请求、设置鉴权、校验响应契约，并把供应商错误转换成通用分类。
- 同类集成使用注册表选择适配器；模型供应商工厂可在应用组装阶段注册或替换，便于后续桥接 Eino 组件或 Cervi 自定义实现。
- Action 负责字段校验和适配器选择。连接测试不读取或写入集成记录，因此创建页和编辑页都直接测试当前草稿。

## 通用执行语义

- 探测由配置所在位置执行。当前 S3 和模型服务使用 `server`；未来访问设备本地 MCP 时可使用 `device`。
- Runner 统一设置超时，默认不重试。重试可能放大限流、延长表单等待时间或重复执行有副作用的供应商接口。
- 探测目标日志只包含 `category`、`adapter`、`location` 和耗时，不包含地址、请求体、响应体或凭据。
- 失败阶段固定为 `connect`、`authenticate`、`authorize`、`capability`。
- 失败类别固定为 `invalid_config`、`unauthorized`、`forbidden`、`not_found`、`rate_limited`、`timeout`、`network`、`tls`、`protocol`、`unavailable`。
- 应用服务把内部分类转换为本地化业务错误；不得把供应商原始错误或响应体直接返回前端。

## 与 Agent Runtime 的边界

- Eino 和 `eino-ext` 用于 Cervi 托管的 `managed` runtime。后续引入 `v0.10.x` beta 时，只在 runtime 和集成适配层依赖其类型，不把 Eino 类型放入 `domain` 或 `appservice` 契约。
- 如果 Eino 组件能提供只读模型查询，可在模型供应商适配器内部复用；连接测试仍以 Cervi 的 `Probe` 契约对外。若 SDK 不支持某家供应商，则直接实现适配器，不影响上层。
- Dify、n8n 等外部平台属于 `connected` runtime。它们各自拥有连接配置和连接测试 Action；AI 员工运行配置只引用已经保存的连接及平台资源标识，不复制 API Key。
- MCP 的协议握手、能力发现和工具列表校验由 MCP 适配器实现。服务端 MCP 与设备本地 MCP 使用同一错误分类，但通过 `location` 保留执行位置差异。

## 新增连接测试的步骤

1. 在 `appservice` 增加该集成的强类型草稿输入和测试方法。
2. 在对应 Action 中复用保存配置时的字段校验。
3. 实现一个无副作用的 `connectiontest.Probe`，并规范化外部错误。
4. 使用共享 Runner 执行，按业务语境映射本地化错误。
5. 贯通 Gin、API Proxy、Wails 绑定和表单交互。

这样扩展新平台时，只增加业务契约和协议适配器，通用执行语义、错误模型及前端调用路径无需重构。
