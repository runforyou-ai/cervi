# 知识库 Agent Tool 方案

## 实现状态

已完成企业知识检索、多查询融合、游标上下文读取和 Agent 知识库范围配置。当前实现以代码为准，下面保留检索编排设计及上线前待办。

本地知识库的部署、内容生命周期、实施 PR 与评测结论见 [本地知识库接入方案](knowledge-base-plan.md)。本地执行层直接接入 Haystack + Hayhooks，保留本篇已有 Tool 编排。

- Agent 创建页和详情运行配置可选择当前企业知识库；新建时绑定列表为空，配置有效知识库范围后注册 `search_knowledge`。
- `managed/v1` Revision 的 `knowledgeBaseIds` 保存明确范围，更新生成新版本；在途 Run 持续读取自身绑定的 Revision。
- 保存时在事务中校验并锁定同企业知识库；被删除的绑定保留在详情中供移除，新运行明确失败并推进消费水位。
- Tool 除 `queries` 外还支持 `cursor` 与 `before`、`after`，两种读取共用同一范围。游标中的知识库必须在绑定范围内。
- 当前 Dify 适配器读取知识库保存的检索配置并用于查询，人工检索测试与 Agent 共用同一检索服务。
- 复杂重试、输出预算和详细失败下标列为 MVP 之后的待办。

## 1. 目标

将知识库检索作为 Cervi 托管 Agent 的只读 Tool。Agent 根据当前问题自行决定是否检索、选择哪个知识库、组织多少条查询以及每条查询的表达方式；Cervi 负责授权、并发执行、结果融合和上下文裁剪。

首个实现使用现有 Dify 知识库，后续本地知识库复用同一 Agent Tool 和多查询编排，模型统一使用业务检索契约。

职责划分如下：

- LLM 提供查询或游标读取参数，检索方式与排名配置由知识库管理。
- Dify 管理并保存自身检索配置。
- 接入统一使用当前数据模型与 Dify 接口。
- 知识库写入与文档修改通过现有业务管理入口执行，Agent Tool 提供只读检索。

## 2. 核心决策

1. 每个 Run 生成一个统一知识检索 Tool，知识库范围通过闭包注入；知识库范围固定为本次 Run 的 Revision 所保存的绑定。
2. Tool 向 LLM 暴露 `queries` 或游标上下文读取参数；检索方式和排名配置由知识库提供。
3. LLM 自行决定数组长度、查询内容与表达方式。
4. 每个查询仍是一次独立的后端检索；MVP 由 Tool 层直接并发执行并负责去重和融合，上线前再补并发与输出预算。
5. Dify 检索使用知识库中保存的检索配置，由 Dify 统一管理。
6. 多查询编排位于统一检索器之上，Dify 连接器负责单查询调用，本地知识库复用相同编排。

## 3. Agent Tool 契约

### 3.1 Tool 生成

Run 开始时，服务端按 `organization_id` 和 Run 绑定 Revision 的 `knowledgeBaseIds` 加载本次可用知识库，并通过闭包注入统一 Tool。绑定列表为空时关闭知识检索；非空范围中有知识库不可用时整个 Run 失败。Tool 按 Run 动态创建并管理生命周期。

Tool 名固定为：

```text
search_knowledge
```

Tool 描述保持精简：

```text
检索当前企业知识库中的相关资料，可以提供多种不同表达的查询。
```

Tool 描述固定使用上述文案，知识库范围由 Run 注入，查询组织方式由 LLM 决定，检索配置由知识库管理。

### 3.2 输入

```json
{
  "queries": [
    "年假可以结转吗",
    "未使用年假的处理规则"
  ]
}
```

JSON Schema 只提供必要字段说明和资源边界：

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": ["queries"],
  "properties": {
    "queries": {
      "type": "array",
      "description": "用于检索的查询列表。",
      "minItems": 1,
      "maxItems": 5,
      "items": {
        "type": "string",
        "minLength": 1,
        "maxLength": 250
      }
    }
  }
}
```

单条 250 字符是 Cervi 统一检索契约的现有产品边界，Dify 与本地知识库沿用相同限制。

服务端按以下规则校验并规范化输入：

- 原始数组必须包含 1 到 5 项。
- 去除每条查询首尾空白；任一查询为空或超过 250 字符时，整个调用返回参数错误。
- 对规范化后完全相同的查询只执行一次，同时保存该执行项对应的全部原始下标。

首版保留查询的大小写、词项和表达，按规范化后的完整文本去重。

### 3.3 输出

Tool 返回融合后的分段和必要引用信息：

```json
{
  "records": [
    {
      "documentId": "document-id",
      "documentName": "休假制度",
      "segmentId": "segment-id",
      "position": 3,
      "content": "……",
      "answer": null,
      "matchedQueryIndexes": [1, 2],
      "truncated": false
    }
  ],
  "failedQueryIndexes": []
}
```

`matchedQueryIndexes` 使用原始输入中从 1 开始的下标。同一查询在原始数组中重复出现时只执行一次，但命中和失败结果仍映射到全部原始下标。同一分段被多条查询命中时只返回一次，并保留按升序排列的全部命中下标。融合分数用于内部排序，LLM 接收融合后的业务结果。

## 4. 执行分层

```text
Agent Run
  -> 按 organization_id 和 Run Revision 绑定解析可用知识库
  -> 通过闭包把知识库范围注入统一 Tool
  -> LLM 调用 Tool，传入 queries
  -> MultiQueryRetriever
       -> 并发调用 KnowledgeRetriever.Retrieve(query)
       -> 合并、去重、排序和裁剪
  -> Dify 连接器或 Haystack + Hayhooks 本地知识库接入
```

后端执行契约保持单查询：

```go
type KnowledgeRetriever interface {
	Retrieve(
		ctx context.Context,
		knowledgeBaseID string,
		query string,
	) ([]RetrievalRecord, error)
}
```

连接器使用类型化错误向编排层提供稳定分类：`canceled`、`unauthorized`、`forbidden`、`not_found`、`invalid_config`、`rate_limited`、`timeout`、`network`、`tls`、`unavailable` 和 `protocol`。Dify 与本地实现都必须映射到这些分类，编排层按类型化分类处理错误。

`MultiQueryRetriever` 是与 Agent SDK、Dify 和本地索引实现无关的编排层。Eino 适配器负责把动态 Tool 接入本次 Run，知识库授权和检索业务由服务端检索层处理。

Tool 的执行身份是本次 Agent Run，按 `AgentExecutionContext` 中的 `organization_id`、Agent、Revision 和 Run 信息授权；执行身份与触发用户的登录身份及权限分离。

## 5. Dify 检索语义

Dify 是其知识库检索配置的唯一事实来源。当前适配器在每次检索闭包首次查询时读取知识库详情，再把保存的检索配置交给 Dify 检索接口；检索方式、Top K、阈值和 Rerank 统一使用 Dify 保存的配置。

人工检索测试与 Agent 实际执行使用同一检索服务。游标读取通过文档和分段接口读取命中片段周边内容，知识库范围仍由闭包约束。

## 6. 多查询编排

### 6.1 并发

每个唯一执行项携带规范化查询、首次出现下标和全部原始下标，按首次出现顺序排列。MVP 为每个“知识库 + 查询”直接并发调用一次单查询 `KnowledgeRetriever`，结果按原始查询顺序保存到对应执行槽位，排名与下标随槽位确定。并发限制在上线前专项补充。

调用共享父 `context`；单个知识库检索失败时记录警告并保留其他知识库的成功结果，全部失败时返回错误。

### 6.2 分段去重

候选分段使用以下键去重：

```text
knowledge_base_id + segment_id
```

同一查询内重复出现的分段只保留最靠前的名次。同一分段被不同查询命中时合并 `matchedQueryIndexes`；分段字段采用最佳名次对应的记录，最佳名次相同时采用首次出现下标较小的记录，保证内容和文档信息稳定。

本地问答接入时，先在来源适配中按问答条目折叠，把条目编号作为逻辑 `segment_id`，返回主问题和完整答案，再进入上述融合。Haystack + Hayhooks 的多个命中片段按同一问答条目合并输出。

### 6.3 融合排序

Dify 分数在各查询内解释，关键词检索的分数可能为空或无效。多查询结果使用 Reciprocal Rank Fusion，只使用每个查询结果中的相对名次：

```text
rrf(record) = Σ 1 / (60 + rank)
```

其中 `rank` 从 1 开始，只对实际命中该分段的唯一执行项累加；原始数组中的重复查询共享一次融合权重。Dify 的 Score Threshold 和 Rerank 已在单次检索中生效，融合阶段按名次计算。

RRF 分数相同时依次比较：

1. 命中查询数量，多者优先。
2. 最佳单查询名次，小者优先。
3. 首次命中的查询下标和名次，小者优先。
4. 知识库编号和分段编号，保证输出稳定。

只有一个唯一执行项时保持后端原始结果顺序。

### 6.4 上线前输出预算

MVP 直接返回融合结果。正式上线前再增加进入模型上下文的结果条数和序列化字节预算。

Agent Tool 在创建编排器时显式提供分段条数上限和序列化后 UTF-8 字节预算，这些值由服务端维护。候选完成融合排序后，按顺序追加完整分段；达到任一上限时停止。预算包含 JSON 结构、文档信息、正文和查询下标。

如果最高排名的首个分段本身超过字节预算，只裁剪该记录的文本字段，并返回 `truncated: true`；该结果仍返回，裁剪后的文本保持合法 UTF-8。现有人工检索测试完整展示单查询后端结果。

## 7. 失败语义

- 参数非法：在输入校验阶段结束调用并返回错误。
- 单条查询召回为空：成功返回空列表。
- 父调用取消，或单条查询遇到鉴权、权限、知识库不存在、配置、TLS 或协议错误：取消其他查询，整个 Tool 调用失败。
- 单条查询遇到超时、限流、网络或服务端临时错误：最多重试一次；仍失败时记录对应下标，并使用其他成功查询继续融合。
- 所有查询均失败：整个 Tool 调用失败。

部分成功时通过按升序排列的 `failedQueryIndexes` 告知 LLM 哪些查询未完成；重复查询失败时包含其全部原始下标。编排层根据类型化错误分类决定重试和终止。同时记录 `WARN` 日志，字段限定为 `organization_id`、Run、知识库、查询数量、成功数量、失败数量、候选数量、输出数量和耗时。

## 8. 本地知识库统一方式

本地知识库直接对接 Haystack + Hayhooks 的专用管线，接入现有检索服务和多查询编排。首版使用向量召回与按知识库配置启用的重排。独立检索模式切换在对应路径验收后提供。Agent Tool 保留 `queries` 和受限范围的游标读取语义；本地 cursor 必须携带 `sourceVersion` 并验证索引版本，Dify 响应省略该字段，读取远端当前文档与分段。具体规则按 [本地知识库接入方案](knowledge-base-plan.md) 落地。

```text
DifyRetriever
  -> Dify 保存检索配置
  -> POST /datasets/{id}/retrieve，只传 query

Haystack + Hayhooks 本地知识库接入
  -> Cervi 服务端从本进程配置注入工作区数据库
  -> 根据身份和 Run Revision 解析知识库、当前索引代次与有效来源
  -> Haystack + Hayhooks 召回，再校验来源修订号并返回原文或完整问答
```

复用的是现有工具入口与结果编排；Dify 保留自身检索配置，本地锁定 Haystack + Hayhooks 框架版本，embedding 与重排选择由 Cervi 后台管理。各路原始分数按来源分别解释，跨来源按名次融合。

## 9. 实现 PR 拆分

### PR 1：支持 Agent 检索企业知识（已完成）

- 增加统一的单查询 `KnowledgeRetriever` 和多查询编排器。
- Dify 检索使用知识库保存的检索配置。
- 通过闭包注入统一 `search_knowledge` Tool；知识库范围现已由 PR 2 的 Revision 绑定提供。
- 并发执行知识库与查询的组合，按知识库和分段去重并使用 RRF 融合。
- 保留 calculator 作为上线前的长期测试 Tool。
- MVP 实现贯通流程所需的参数校验和错误返回；配额、复杂重试、降级和输出预算列入上线前专项。
- 覆盖多查询、多知识库、重复分段、稳定排序和组织隔离测试。

### PR 2：配置 Agent 知识库范围（已完成）

- Agent Revision 增加知识库绑定配置，管理界面支持选择同一 `organization_id` 下的知识库。
- 单聊和网站客服 Run 使用自身 Revision 的绑定，空列表关闭检索；已有 Run 保持自身绑定范围。
- 删除知识库后保留失效绑定供用户移除，新运行明确失败并推进消费水位。
- 沿用 Tool 闭包、输入契约、多查询执行和 RRF 融合。
- 覆盖多知识库选择、`organization_id` 隔离和失效绑定测试。

### 后续：本地知识库

- 按 [本地知识库接入方案](knowledge-base-plan.md) 分 PR 完成 pgvector 基础环境、Haystack + Hayhooks、后台模型配置与索引版本、本地问答检索和文档导入。
- Cervi 管理业务来源、文件和处理状态，Haystack + Hayhooks 执行分段与索引，Cervi 管理模型配置与索引代次；继续复用 Agent Tool 和多查询编排。

## 10. 验收标准

- LLM 使用 `queries` 查询或使用 `cursor` 与 `before`、`after` 读取上下文，查询数量与表达方式由模型自行决定。
- Tool 描述说明检索目的与输入能力，查询数量和表达方式由模型决定。
- Dify 实际检索方式和 Top K 使用其知识库保存的配置。
- 多条查询并发执行，重复分段只返回一次，输出顺序稳定。
- 单条查询时结果顺序与后端一致，多条查询时按 RRF 融合。
- 上线前专项补充检索并发与模型上下文输出预算。
- Dify 与本地知识库共用 Agent Tool 的查询和范围语义；本地游标约束来源索引版本，Dify 校验并读取当前远端分段；模型使用业务查询与游标参数，底层索引表和执行引擎由服务端解析。
- Agent 查询与游标读取均限定在当前 `organization_id` 且 Run Revision 已绑定的知识库范围内。
