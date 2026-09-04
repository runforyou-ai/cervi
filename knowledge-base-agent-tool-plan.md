# 知识库 Agent Tool 方案

## 1. 目标

将知识库检索作为 Cervi 托管 Agent 的只读 Tool。Agent 根据当前问题自行决定是否检索、选择哪个知识库、组织多少条查询以及每条查询的表达方式；Cervi 负责授权、并发执行、结果融合和上下文裁剪。

首个实现使用现有 Dify 知识库，后续本地知识库复用同一 Agent Tool 和多查询编排，不向模型暴露后端类型。

本方案不包含：

- 让 LLM 选择检索方式、Top K、Score Threshold、Rerank 或混合检索权重。
- 在 Cervi 中复制 Dify 的检索配置。
- 历史数据和旧版 Dify 兼容逻辑。
- 写入知识库、修改文档或其他有副作用的知识库 Tool。

## 2. 核心决策

1. 每个 Run 生成一个统一知识检索 Tool，知识库范围通过闭包注入；MVP 默认使用当前企业全部知识库，后续由 Agent 配置收窄范围。
2. Tool 只向 LLM 暴露 `queries` 数组，不暴露检索参数。
3. LLM 自行决定数组长度和查询内容，Tool 描述不提供“简单问题一条、复杂问题多条”等生成规则。
4. 每个查询仍是一次独立的后端检索；MVP 由 Tool 层直接并发执行并负责去重和融合，上线前再补并发与输出预算。
5. Dify 检索请求只传 `query`，由 Dify 使用知识库中保存的检索配置。
6. 多查询编排位于统一检索器之上，不进入 Dify 连接器，因而可以直接复用于本地知识库。

## 3. Agent Tool 契约

### 3.1 Tool 生成

Run 开始时，服务端按 `organization_id` 加载本次可用知识库，并通过闭包注入统一 Tool。MVP 使用当前企业全部知识库；后续 Agent Revision 保存允许使用的知识库编号后，只调整范围加载参数，不修改 Tool 契约和检索实现。Tool 是本次 Run 的动态依赖，不加入进程级全局 Tool 列表。

Tool 名固定为：

```text
search_knowledge
```

Tool 描述保持精简：

```text
检索当前企业知识库中的相关资料，可以提供多种不同表达的查询。
```

描述中不展开知识库清单、查询改写规则或检索参数。

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

单条 250 字符是 Cervi 统一检索契约的现有产品边界，不是 Dify 专属参数，本地知识库沿用相同限制。

服务端不依赖模型遵守 JSON Schema，并再次执行以下校验和规范化：

- 原始数组必须包含 1 到 5 项。
- 去除每条查询首尾空白；任一查询为空或超过 250 字符时，整个调用返回参数错误。
- 对规范化后完全相同的查询只执行一次，同时保存该执行项对应的全部原始下标。

首版不做大小写折叠、分词去重或语义去重，避免 Cervi 改变 LLM 的检索意图。

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

`matchedQueryIndexes` 使用原始输入中从 1 开始的下标。同一查询在原始数组中重复出现时只执行一次，但命中和失败结果仍映射到全部原始下标。同一分段被多条查询命中时只返回一次，并保留按升序排列的全部命中下标。融合分数属于内部排序细节，不返回给 LLM。

## 4. 执行分层

```text
Agent Run
  -> 按 organization_id 解析可用知识库
  -> 通过闭包把知识库范围注入统一 Tool
  -> LLM 调用 Tool，传入 queries
  -> MultiQueryRetriever
       -> 并发调用 KnowledgeRetriever.Retrieve(query)
       -> 合并、去重、排序和裁剪
  -> DifyRetriever 或 LocalRetriever
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

连接器使用类型化错误向编排层提供稳定分类：`canceled`、`unauthorized`、`forbidden`、`not_found`、`invalid_config`、`rate_limited`、`timeout`、`network`、`tls`、`unavailable` 和 `protocol`。Dify 与本地实现都必须映射到这些分类，编排层不解析错误文本。

`MultiQueryRetriever` 是与 Agent SDK、Dify 和本地索引实现无关的编排层。Eino 适配器只负责把动态 Tool 接入本次 Run，不承载知识库授权和检索业务。

Tool 使用 `AgentExecutionContext` 中的 `organization_id`、Agent、Revision 和 Run 信息授权，不构造用户登录 Identity，也不继承触发用户的权限。

## 5. Dify 检索语义

Dify 是其知识库检索配置的唯一事实来源。Cervi 对每条查询发送：

```json
{
  "query": "年假可以结转吗"
}
```

Cervi 不读取后再回传 `retrieval_model_dict`，也不发送或覆盖以下参数：

- `search_method`
- `top_k`
- `reranking_enable`
- `reranking_model`
- `weights`
- `score_threshold_enabled`
- `score_threshold`

知识库在 Dify 中配置为关键词检索时，每个查询都由 Dify 执行关键词检索；配置的 Top K 为 3 时，每个查询各自最多召回 3 个候选。Dify 配置修改后，下一次检索直接使用新配置。

Cervi 不在 Dify 返回后再次应用 Score Threshold 或结果重排。目标 Dify 不接受只传 `query` 的调用或无法执行已保存配置时，将其作为配置或协议错误返回。

Embedding、Rerank、鉴权或知识库配置错误由 Dify 返回，Cervi 将其映射为结构化 Tool 错误，不静默切换检索方式。

现有知识库“检索测试”也应收敛为只输入查询并使用知识库配置，避免人工测试和 Agent 实际执行采用两套检索语义。

## 6. 多查询编排

### 6.1 并发

每个唯一执行项携带规范化查询、首次出现下标和全部原始下标，按首次出现顺序排列。MVP 为每个“知识库 + 查询”直接并发调用一次单查询 `KnowledgeRetriever`，结果保存到对应执行槽位，不能按请求完成顺序改变查询排名或下标。并发限制在上线前专项补充。

调用共享父 `context`；单个知识库检索失败时记录警告并保留其他知识库的成功结果，全部失败时返回错误。

### 6.2 分段去重

候选分段使用以下键去重：

```text
knowledge_base_id + segment_id
```

同一查询内重复出现的分段只保留最靠前的名次。同一分段被不同查询命中时合并 `matchedQueryIndexes`；分段字段采用最佳名次对应的记录，最佳名次相同时采用首次出现下标较小的记录，保证内容和文档信息稳定。

### 6.3 融合排序

不同查询的 Dify 分数不可直接横向比较，关键词检索也可能没有有效分数。多查询结果使用 Reciprocal Rank Fusion，只使用每个查询结果中的相对名次：

```text
rrf(record) = Σ 1 / (60 + rank)
```

其中 `rank` 从 1 开始，只对实际命中该分段的唯一执行项累加；原始数组中的重复查询不会重复增加融合权重。Dify 的 Score Threshold 和 Rerank 已在单次检索中生效，融合阶段不再应用统一分数阈值。

RRF 分数相同时依次比较：

1. 命中查询数量，多者优先。
2. 最佳单查询名次，小者优先。
3. 首次命中的查询下标和名次，小者优先。
4. 知识库编号和分段编号，保证输出稳定。

只有一个唯一执行项时保持后端原始结果顺序。

### 6.4 上线前输出预算

MVP 直接返回融合结果。正式上线前再增加进入模型上下文的结果条数和序列化字节预算。

Agent Tool 在创建编排器时显式提供分段条数上限和序列化后 UTF-8 字节预算，这些值不加入 Tool 参数和描述。候选完成融合排序后，按顺序追加完整分段；达到任一上限时停止。预算包含 JSON 结构、文档信息、正文和查询下标。

如果最高排名的首个分段本身超过字节预算，只裁剪该记录的文本字段，并返回 `truncated: true`，不能用空结果伪装成未命中；裁剪必须保留合法 UTF-8。现有人工检索测试直接展示单查询后端结果，不套用 Agent Tool 的输出预算。

## 7. 失败语义

- 参数非法：整个 Tool 调用失败，不发起检索。
- 单条查询没有召回结果：视为空列表，不是错误。
- 父调用取消，或单条查询遇到鉴权、权限、知识库不存在、配置、TLS 或协议错误：取消其他查询，整个 Tool 调用失败。
- 单条查询遇到超时、限流、网络或服务端临时错误：最多重试一次；仍失败时记录对应下标，并使用其他成功查询继续融合。
- 所有查询均失败：整个 Tool 调用失败。

部分成功时通过按升序排列的 `failedQueryIndexes` 告知 LLM 哪些查询未完成；重复查询失败时包含其全部原始下标。编排层只根据类型化错误分类决定重试和终止，不解析供应商错误文本。同时记录 `WARN` 日志，包含 `organization_id`、Run、知识库、查询数量、成功数量、失败数量、候选数量、输出数量和耗时，不记录 API Key。

## 8. 本地知识库统一方式

本地知识库实现相同的单查询 `KnowledgeRetriever`，从 Cervi 保存的知识库配置中选择关键词、全文、向量或混合检索。Agent Tool、`queries` 契约、并发、RRF、输出预算和失败语义保持不变。

```text
DifyRetriever
  -> Dify 保存检索配置
  -> POST /datasets/{id}/retrieve，只传 query

LocalRetriever
  -> Cervi 保存检索配置
  -> 调用本地索引，只接收 query
```

统一的是执行契约和检索结果，不统一 Dify 与本地知识库的底层配置字段。

## 9. 实现 PR 拆分

### PR 1：支持 Agent 检索企业知识

- 增加统一的单查询 `KnowledgeRetriever` 和多查询编排器。
- Dify 检索请求改为只传 `query`，移除 Cervi 对 Dify 检索参数的覆盖。
- Run 开始时加载当前企业全部知识库，通过闭包注入统一 `search_knowledge` Tool。
- 并发执行知识库与查询的组合，按知识库和分段去重并使用 RRF 融合。
- 保留 calculator 作为上线前的长期测试 Tool。
- MVP 只实现贯通流程所需的参数校验和错误返回，不提前增加配额、复杂重试、降级和输出预算。
- 覆盖多查询、多知识库、重复分段、稳定排序和组织隔离测试。

### PR 2：配置 Agent 知识库范围

- Agent Revision 增加知识库绑定配置，管理界面支持选择同一 `organization_id` 下的知识库。
- Run 开始时把默认企业范围替换为 Revision 绑定范围。
- Tool 闭包、输入契约、多查询执行和 RRF 融合保持不变。
- 覆盖多知识库选择、`organization_id` 隔离和失效绑定测试。

### 后续：本地知识库

- 增加本地文档、分段、索引和检索配置。
- 实现 `LocalRetriever`，不修改 Agent Tool 契约和多查询编排。

## 10. 验收标准

- LLM 只能看到知识库用途和 `queries`，查询数量与表达方式由模型自行决定。
- Tool 描述不包含固定查询数量、查询改写模板或检索参数建议。
- Dify 请求体只包含 `query`，实际检索方式和 Top K 与 Dify 知识库配置一致。
- 多条查询并发执行，重复分段只返回一次，输出顺序稳定。
- 单条查询时结果顺序与后端一致，多条查询时按 RRF 融合。
- 上线前专项补充检索并发与模型上下文输出预算。
- Dify 与本地知识库可以替换实现而不改变 Agent Tool Schema。
- MVP Agent 只能检索当前 `organization_id` 的知识库；范围配置完成后只能检索 Revision 已绑定的知识库。
