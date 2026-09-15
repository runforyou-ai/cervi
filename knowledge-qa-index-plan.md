# 问答知识库索引与召回方案

日期：2026-09-14。状态：已实施，验证记录见第 9 节。

本文承接 [知识文档处理管线 Go 化与 Haystack 移除方案](knowledge-go-pipeline-plan.md) 第 10 节中「本地问答路在进入融合前按条目折叠」的未完成项，把问答知识库接入已有的分段、向量、词法与混合召回链路。Agent 运行期检索见 [知识库 Agent Tool 方案](knowledge-base-agent-tool-plan.md)。

## 1. 现状与目标

文档知识库已经完成上传、转换、分段、向量化、词法词元写入、混合召回与检索测试。问答知识库只保存主问题、相似问题和答案，保存后不产生任何分段，知识库上配置的向量模型与召回参数对问答库尚未生效。

目标：问答条目保存后自动进入索引，检索测试与后续 Agent 检索在问答库中命中任何问题或答案片段时，都返回该条目的主问题与完整答案。

## 2. 索引单元

一条问答是一个逻辑来源，索引时拆成多段：

| 分段来源 | 数量 | 正文 |
|---|---|---|
| 主问题 | 1 | 问题原文 |
| 相似问题 | 每条 1 | 问题原文 |
| 答案 | 按字符分段后的段数 | 答案原文的连续切片 |

- 问题与答案分开建向量。用户提问与录入问题形态一致，问题对问题的相似度是最可靠的信号；答案单独切段是为了兜住「问法不像，但答案里恰有该细节」的情况。问题与答案不拼接编码。
- 答案使用 `textsplit.Split` 切段，长度与重叠使用固定常量 `domain.KnowledgeQAChunkLength = 512`、`domain.KnowledgeQAChunkOverlap = 50`。问答库表单继续不展示分段参数。
- 分段位置在批次内连续：主问题为 1，相似问题按 `sort_order` 依次递增，答案分段随后。
- 每段同时写入向量和 `search_vector`，词法路对问题原文的精确词元同样生效。
- 答案整段保存在 `knowledge_qa_contents`，是唯一交付来源；分段正文只用于匹配。召回时整段返回答案，不截断。答案长度暂不设产品上限。

## 3. 数据模型

### 3.1 `knowledge_segments` 来源化

分段表当前以 `document_id` 关联文档。问答分段写入同一张表，来源字段改为通用语义：

| 变更 | 说明 |
|---|---|
| `document_id` 重命名为 `source_id` | 文档编号或问答条目编号 |
| 新增 `source_type text NOT NULL` | `document` 或 `qa_entry`，现有记录回填 `document` |
| 唯一索引改为 `(source_id, segment_batch_id, position)` | 语义不变 |

向量列、部分索引、`search_vector` 和 GIN 索引保持不变。`segment_store.go`、`document_segments.go` 与删除函数按 `source_id` 改写。

### 3.2 `knowledge_qa_entries` 索引状态

新增与 `knowledge_documents` 同义的索引列：`status`、`processing_id`、`segment_batch_id`、`segment_count`、`failure_code`、`embedding_provider_id`、`embedding_model_identifier`、`embedding_dimension`。问答不保存分段参数，由常量固定。

### 3.3 索引状态类型共享

`domain.KnowledgeDocumentStatus` 及 `IsProcessing` 改名为 `domain.KnowledgeIndexStatus`，值不变；`appservice` 中的 `KnowledgeDocumentStatus`、`KnowledgeDocumentProcessingStatus` 相应改为 `KnowledgeIndexStatus`、`KnowledgeIndexProcessingStatus`。前端把 `documents.status.*` 词条提升为 `indexStatus.*`，`KnowledgeDocumentStatus` 组件泛化为按状态与失败信息展示的 `KnowledgeIndexStatus`。该改名作为第一个提交单独审阅。

问答任务经历的阶段为 `queued → splitting → embedding → publishing → succeeded | failed`，不经过 `fetching` 与 `converting`。

## 4. 问答索引任务

- 任务名 `knowledge.qa_entry.process`，队列 `QueueKnowledge`，`MaxAttempts` 为 1，幂等键为 `processing_id`。
- 载荷 `ProcessQAInput`：企业、知识库、条目编号、`processing_id` 与向量模型快照。内容在执行时读取，不放入载荷；条目再次编辑会生成新的 `processing_id` 并投递新任务，旧任务在 `setStage` 时因 `processing_id` 不匹配而退出。
- 执行顺序：置 `splitting` 读取当前主问题、相似问题与答案并组段；置 `embedding` 调用 `embedding.Embed`；置 `publishing` 在一个事务内持条目行锁校验 `processing_id` 与执行状态、删除该来源全部分段、写入本批次分段、更新 `segment_batch_id`、`segment_count` 与状态，最后 `servertask.LockExecution`。
- 失败由 `FinalizeFailure` 记录 `failed` 与原因码，错误码沿用 `embedding_model_unavailable`、`embedding_failed`、`embedding_dimension_mismatch`、`empty_content`、`service_failed`。
- 共享实现：`insertSegments` 按来源类型与编号写入；供应商入口解析当前在 `process_document.go` 与 `retrieve.go` 各写一份，提取为 `resolveEmbeddingCredential` 供文档处理、问答处理与检索三处使用；阶段更新与失败落库按表参数化后复用。

## 5. 触发与界面

- 新建问答：保存事务内置 `queued`、生成 `processing_id` 并投递任务。
- 编辑问答：`saveQAContents` 返回内容是否变化；主问题、相似问题或答案任一变化，或条目当前状态不是 `succeeded`，则投递新任务；仅移动分组保留已有分段与批次。
- 删除问答：同事务删除该来源全部分段。删除知识库已按 `knowledge_base_id` 清理分段，无需改动。
- 重试：新增 `POST /knowledge-bases/:knowledgeBaseID/qa-entries/:entryID/retry`，按当前知识库模型配置重新投递；问答不依赖 markitdown，无连接检查。
- 问答列表：`QASummary` 增加状态与失败信息；表格列为主问题、相似问题、答案、状态、创建时间、操作；排队或处理中时每 2 秒轮询，与文档列表一致；三点菜单增加「重试」。
- 知识库向量模型或分段参数变更后，文档与问答条目一并重建索引，规则见 [知识文档处理管线 Go 化方案](knowledge-go-pipeline-plan.md) 第 10 节。

## 6. 问答召回

### 6.1 检索服务按知识库类别分支

- `Retrieve` 的就绪检查按类别：文档库查 `knowledge_documents`，问答库查 `knowledge_qa_entries` 是否存在已发布批次。
- `publishedSegments` 接收知识库记录：文档库联结 `knowledge_documents` 与 `files`，名称为文件名；问答库联结 `knowledge_qa_entries` 与主问题内容，名称为主问题。两条路径都以来源当前 `segment_batch_id` 限定已发布分段。
- 向量路、词法路、RRF 融合与重排打分保持不变。问答库在重排之后按 `source_id` 折叠，每个条目保留最高重排得分的分段，再截取知识库的召回数量；随后一次查询读取这些条目的答案。

### 6.2 结果契约

- `RetrievalRecord` 与 `knowledgeretrieval.Record` 增加 `Answer`，文档记录为空。问答记录的 `DocumentID` 与 `SegmentID` 均为条目编号，`Position` 固定为 1，`Content` 为命中的问题或答案分段原文，`DocumentName` 为主问题，`Answer` 为完整答案。跨知识库融合按 `knowledge_base_id + segment_id` 去重，相似问题与答案分段的命中因此合并为一条。
- `Source.Read` 对问答库忽略 `before` 与 `after`，校验条目存在且当前已发布批次与游标批次一致后返回该条目一条记录，否则返回 `ErrSegmentStale`；条目修改并重新发布后旧游标失效。
- `appservice.KnowledgeRetrievalRecord` 增加 `answer`；检索测试结果对问答记录展示主问题、命中原文与完整答案，不展示分段序号与「查看上下文」。
- 问答列表页标题栏增加「检索测试」入口，复用现有侧栏。

## 7. 交付范围

索引与召回在同一个 PR 内交付，提交顺序为：状态类型共享改名、分段表来源化与条目索引列、问答处理任务与触发、问答召回与结果契约、前端列表状态与检索测试展示。文档库的行为在整个 PR 内保持不变。Agent 运行期检索工具通过 `Record.Answer` 输出问答的完整答案。

## 8. 验收

`wails3 task test:server`：

- 新建问答后任务投递、分段数量等于问题数加答案段数、位置连续、向量维度与词法词元落库。
- 编辑问题或答案后批次替换且旧分段清除；仅移动分组不投递；旧 `processing_id` 的任务在阶段更新时退出。
- 删除问答后分段清除；删除知识库后分段清除；企业隔离；重试重新投递；向量接口失败时状态为 `failed` 并保留上一批次。
- 主问题、相似问题、答案分段三种命中都折叠为一条并返回完整答案；同一条目多段命中只出现一次。
- 问答库无已发布批次时返回未就绪；条目删除后旧游标读取返回失效。
- 文档库检索结果与本轮改动前一致。

前端构建与测试通过；界面验证问答列表状态流转、重试，以及问答库检索测试展示与文档库展示互不影响。

验证结束后清理本次启动的进程。

## 9. 验证记录

- `wails3 task test:server` 通过。新增 `TestKnowledgeQAIndexLifecycle` 覆盖保存投递、分段构成与位置、词元与向量落库、移动分组不投递、内容变更替换批次、旧任务退出、向量失败保留旧批次、重新保存未完成条目再投递、企业隔离、条目与知识库删除清理；新增 `TestKnowledgeQARetrieval` 覆盖未就绪、问题与答案片段折叠为条目、完整答案、跨库融合去重、游标读取与删除后失效。现有文档处理与混合召回测试在分段表来源化后保持通过。
- 前端 `common:build:frontend` 与 `test:frontend` 通过。
- 服务端实际运行验证（通义千问 `qwen3.7-text-embedding` 1536 维与 `qwen3-rerank`）：通过 HTTP API 创建含两条相似问题和长答案的问答，状态经排队、处理中到已完成，落库 5 段（3 段问题、2 段答案）且均带向量与词元；检索「怎么退钱」折叠为 1 条记录，返回主问题、命中片段与完整答案；重试后重新发布批次；删除后分段清空。文档库检索结果不受影响。
- 2026-09-15 浏览器界面验证（通义千问 `qwen3.7-text-embedding` 与 `qwen3-rerank`）：问答列表展示状态列，三点菜单「重试」执行后回到已完成，排队与处理中的中间状态因处理较快未截取到；问答库检索测试展示主问题、命中内容与完整答案，不展示分段序号与「查看上下文」；文档库检索测试可查看上下文；知识库编辑页展示绑定的 AI 员工。`qwen3-rerank` 对问答短文本的重排得分约 0.55 至 0.68，问题原文与自身的得分为 0.648，相关性阈值为 0.7 时问答库检索无结果，调整为 0.5 后正常命中。
- AI 员工运行期：绑定文档库与问答库后，系统指令未要求检索时模型直接作答且未调用 `search_knowledge`；系统指令要求先检索后，模型调用 `search_knowledge` 并依据问答答案与文档原文作答。
