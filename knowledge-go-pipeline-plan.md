# 知识文档处理管线 Go 化与 Haystack 移除方案

日期：2026-09-11。状态：待实施的开发方案。

本文取代 [知识库接入方案](knowledge-base-plan.md) 第 3 节和 [知识库最小 MVP 开发顺序](knowledge-base-mvp-plan.md) PR 3 中"本地执行层直接采用 Haystack + Hayhooks"的结论。

调整后的职责边界：**原件转换交给 markitdown 独立容器，其余全部在 Go 服务端进程内**。字符分段、向量化、分段写入、批次发布、分段阅读和召回编排都由 Go 完成，`haystack/` 目录及其 HTTP 服务整体删除。

## 1. 依赖形态

markitdown 与 Haystack 的区别不在于"是否是 Python"，而在于组件形态：

| | Haystack（移除） | markitdown（引入） |
|---|---|---|
| 状态 | 持有管线状态和执行阶段 | 无状态 |
| 数据库 | 持有数据库凭据，直接 `UPDATE knowledge_documents`、写 `knowledge_segments` | 不接触数据库 |
| 业务语义 | 在 HTTP 边界后重复一套业务契约 | 只做字节进、Markdown 出 |
| 工作区隔离 | 连接当前工作区数据库，需要每工作区独立端口 | 无状态，所有工作区共享一份实例 |
| 可替换性 | 业务逻辑与框架耦合 | 换成任意转换器只改一个客户端 |

markitdown 为 MIT 许可，无 CGO 和 AGPL 约束，同时解决了纯 Go PDF 文本抽取的质量问题。

## 2. 范围

本次交付：

- 引入 markitdown 独立容器，Go 通过 HTTP 提交原件字节、取回 Markdown 正文。
- 在 Go 内实现字符分段与重叠。
- 在 Go 内调用 OpenAI 兼容 `/v1/embeddings` 生成分段向量。
- 分段写入、批次发布和分段阅读全部由 Go 在业务事务内完成。
- 删除 Haystack 服务、Python 管线、相关 Task、环境变量和配置项。

本次不做：

- **OCR 与扫描版 PDF**。PDF 一律按纯文本解析，无可提取文字的原件按 `empty_content` 失败。
- **页级引用**。见第 3 节，`page_number` 与 `source_label` 两个 meta 字段一并删除。
- **引入 Qdrant**。向量继续存放在同库的 `public.knowledge_segments`，沿用现有 halfvec HNSW 部分索引。
- **向量召回接入**。`knowledgeretrieval.Source` 的实际实现见第 9 节，另行排期。

## 3. 已确认的能力变化

以下三项已用中文样本在 markitdown 0.1.7 容器上实测确认，是本次接入的既定代价：

**PDF 页码不可靠。** `_pdf_converter.py` 有两条输出路径：所有页都未被识别为表单式版面时（`form_page_count == 0`）走 `pdfminer.high_level.extract_text`，页与页之间保留换页符 `\f`；只要有任一页被识别为表单式版面，就改为 `"\n\n".join(markdown_chunks)`，且正文为空的页直接跳过。

实测中文纯文本 PDF 和标签取值分列的表单式 PDF 均走 pdfminer 路径：三页 PDF（含一页空白）按 `\f` 切分得到正好三段，空白页对应空段。未能构造出触发表单式分支的样本，该分支行为按源码判定。

因此按 `\f` 计算页码对多数 PDF 成立，但响应中没有任何字段标明本次走的是哪条路径：一旦命中表单式分支，分隔符和页序对应关系同时失效，页码会静默错位而非缺失。`knowledge_segments.meta.page_number` 不按此实现，roadmap 中"页级引用使用解析器提供的真实页码"推后到能够稳定判定页边界时。中文正文本身无乱码。

**XLSX 数值被类型推断改写。** `_xlsx_converter.py` 使用 `pd.read_excel(..., engine="openpyxl")`，pandas 按数值读取单元格。实测编号 `0012` 输出为 `12`，金额 `1234.50` 输出为 `1234.5`：前导零和小数尾零都会丢失。现行实现中 `dtype=str, keep_default_na=False` 的防护不再存在。金额与编号类字段的检索和引用按改写后的字面值进行。

**来源标记只保留在正文内。** XLSX 输出 `## 工作表名` 标题，PPTX 每页输出 `<!-- Slide number: N -->` 注释，PDF 在 pdfminer 路径下输出 `\f`，均经实测确认。这些信息进入分段正文，不再单独成列。将来若要恢复页级引用，PPTX 和 XLSX 的序号可从正文标记还原，PDF 需要第 9 节之外的单独方案。

若后续认定 XLSX 的数值改写不可接受，把该格式的分派改回 Go 侧的 excelize 只影响 `ProcessDocumentAction` 中一处按扩展名的分派，不动其余链路。

据此删除 `knowledge_segments.meta` 的 `page_number` 和 `source_label`，以及 `knowledgeprocessing.Segment` 的 `PageNumber`、`SourceLabel` 字段和前端对应展示。不为单个格式保留只在该格式生效的兜底字段。

## 4. 当前替换点

现有分层已经把跨进程边界收敛成两个接口，替换成本集中在实现侧：

| 位置 | 现状 | 目标 |
|---|---|---|
| `knowledgebase.documentProcessor` | `knowledgeprocessing.Client.Process` 经 multipart HTTP 提交原件，Python 完成全部处理 | Action 内调用 markitdown 转换，随后在 Go 完成分段、向量化和写入 |
| `knowledgebase.segmentReader` | `knowledgeprocessing.Client.List` 经 HTTP 查询分段 | `DocumentQuery` 直接用 Bun 查库 |
| 处理阶段状态 | Python 直接 `UPDATE knowledge_documents` | Action 更新，与业务写入同源 |
| 批次发布 | Python 先写分段，Go 再核验数量与租约后发布 | 分段写入与发布在同一事务内完成 |

`internal/integration/knowledgeprocessing` 整包删除：`ProcessInput` 和 `Error` 迁入 `actions/knowledgebase` 作为任务载荷和语言无关错误码，`Segment`、`ListInput`、`SegmentPage` 迁入同包作为查询契约。

## 5. 目标结构

```text
markitdown/                        # 转换服务镜像定义与最小 HTTP 包装
internal/
├── common/textsplit/              # 字符分段与重叠（纯计算）
├── integration/
│   ├── documentconvert/           # markitdown HTTP 客户端与连接检查
│   └── embedding/                 # OpenAI 兼容向量接口的批量调用
└── actions/knowledgebase/
    ├── process_document.go        # 转换、分段、向量化、写入并发布批次
    └── document_segments.go       # 固定批次的分页与锚点查询
```

### 5.1 为什么自己包一层 HTTP

markitdown 官方不提供 HTTP 服务，README 明确列为 out of scope；可用的只有 CLI、Python API 和 `markitdown-mcp`。`markitdown-mcp` 的工具签名是 `convert_to_markdown(uri)`，只接受 URI：走 `file:` 需要两个容器共享卷，在对象存储模式下原件不落盘因而不成立；走 `data:` 需要把整份原件 base64 塞进 JSON-RPC 消息，几十 MB 的原件内存开销不可接受。官方同时警告其 HTTP 传输无鉴权且会跟随任意 `http(s)` URI 取数。

因此 `markitdown/` 下提供一个最小 FastAPI 包装，约 50 行：

- `POST /convert`，multipart 接收原件字节，返回 `{"markdown": "..."}`。
- `GET /status`，供连接检查使用。
- 在临时目录落盘、转换、退出时清理；不接受任何 URI，不持有数据库凭据，不读环境中的业务配置。

Go 侧原件读取路径（本地目录或对象存储流式读）已经存在，multipart 流式转发不需要整份文件驻留内存。

### 5.2 部署

markitdown 无状态，与 PostgreSQL、NATS 一样在主工作区共享启动，不再需要每工作区独立端口：

```bash
docker compose up -d postgres nats markitdown
```

`docker-compose.yml` 增加 `markitdown` 服务，镜像由 `markitdown/Dockerfile` 构建。**镜像与 markitdown 版本必须固定到精确版本**：转换器行为变化会改变分段正文，未固定版本会让重新处理产生与历史批次不一致的结果。

### 5.3 镜像依赖

按 `markitdown[all]` 安装全部可选依赖，为后续扩展知识库格式留出空间。镜像参照官方 `Dockerfile`：基于 `python:3.13-slim-trixie`，apt 安装 `ffmpeg` 与 `libimage-exiftool-perl`，再叠加包装所需的 `fastapi`、`uvicorn`、`python-multipart`，`ENTRYPOINT` 由官方的 `markitdown` CLI 改为 uvicorn 启动包装服务。

`[all]` 覆盖 `pptx`、`docx`、`xlsx`、`xls`、`pdf`、`outlook`、`audio-transcription`、`youtube-transcription`、`az-doc-intel`、`az-content-understanding` 十个组，带入 pandas、lxml、pdfplumber、pydub、SpeechRecognition 和 azure-ai-* 等包，镜像体积明显大于按需安装。私有化部署需拉取该镜像，发布时在部署文档中说明。

装全依赖后，**实际可处理范围由 Go 侧的扩展名白名单 `domain.KnowledgeDocumentFormat` 决定**，不由容器决定。后续扩展格式只改 Go 白名单、前端接受类型和文案，不重建镜像。

三类能力装了依赖也不会自动可用，需要单独设计后再开：

- **URL 与 YouTube 来源**：对应转换器只接受 URI，而包装服务只接受上传的字节。若要支持，需在包装中单开接口并明确出网边界。
- **Azure Document Intelligence 与 Content Understanding**：需要 Azure endpoint 与凭据，且会把原件送往 Azure，与私有化部署定位冲突。
- **音频转写**：`_transcribe_audio.py` 调用的是 `recognizer.recognize_google(audio)`，音频会发送到 Google 的识别服务。包装服务按音频转换器声明的 `.wav`、`.mp3`、`.m4a`、`.mp4` 四个后缀直接返回 `unsupported_file`，使该路径在装有全部依赖的镜像上不可达。扩展音频能力时改为自托管 ASR，与 roadmap 第 6 节"先确定本地或外部 ASR"一致。

## 6. PR 1：Go 字符分段

只交付纯计算能力和单元测试，不接线。

`textsplit.Split(text string, length, overlap int) []Segment`，直译现有实现并保持三项性质：

- 按段落边界（`\n\n`、`\n`、`。`、`！`、`？`、`；`、空格）就近收缩，边界必须覆盖半段且长度大于重叠长度，否则按长度硬切。
- 重叠从原文连续区间统一添加一次，长度和重叠均按 Unicode 字符计数（`[]rune`）。
- 分段正文是输入文本的连续切片，拼接后可逐字符还原。转换后的 Markdown 是这条性质的参照物。

现行 Python 依赖 `RecursiveDocumentSplitter` 产出边界后用 `bisect` 重算，Go 版本直接在 rune 切片上求边界，逻辑更短。

**验收**：`wails3 task test:server` 中新增单元测试，`haystack/tests/test_processing.py` 的分段断言逐条迁移；中文长文本分段后拼接与原文逐字符相等，边界收缩与硬切两条路径分别覆盖。

## 7. PR 2：分段阅读查询下沉到 Go

`DocumentQuery.Segments` 改为直接用 Bun 查询，删除 `segmentReader` 接口和对 Haystack `/knowledge/segments` 的调用。合并后 Haystack 仍在运行，但只承担转换与处理职责。

查询语义保持不变：

- 在 `REPEATABLE READ READ ONLY` 事务中先核验文档归属、企业边界和已发布批次，快照内完成锚点定位与取页。
- 传 `anchorSegmentId` 时按该分段之前的实际记录数量计算页码，返回 `anchorSegmentId` 与 `anchorPosition`；与 `page` 互斥。
- 批次被替换、锚点不存在或来源已删除返回 `segment_stale`，不跳到其他分段。
- 排序固定为 `(meta->>'position')::int, id`。

`knowledge_processing_integration_test.go` 的 `segmentReaderProbe` 替换为写入真实分段记录后查询断言。

**验收**：`wails3 task test:server` 覆盖锚点定位、跨企业越权、批次失效、分页边界；界面确认分段查看弹窗首次定位高亮与双向滚动加载正常。

## 8. PR 3：markitdown 容器与转换客户端

不改变现有处理路径，只交付可独立验证的转换能力。

- `markitdown/Dockerfile` 与 FastAPI 包装，`docker-compose.yml` 增加服务定义。
- `internal/integration/documentconvert`：`Convert(ctx, name string, source io.Reader) (string, error)` 与 `CheckConnection(ctx) error`，超时与错误码沿用现有语义（`unavailable`、`connection_timeout`、`request_timeout`）。
- `.env.example` 与 `internal/config/server/config.go` 增加 `MARKITDOWN_URL`。
- `internal/server-deployment.md` 增加 markitdown 部署说明。

**验收**：`wails3 task test:server` 中客户端单元测试覆盖成功、非 200、超时和连接拒绝四条路径；手工对中文 PDF、DOCX、PPTX、XLSX 各一份调用 `POST /convert`，确认正文非空且无乱码。

## 9. PR 4：流水线切换并移除 Haystack

### 9.1 处理流程

`ProcessDocumentAction.Execute` 改为：

1. 置 `fetching`，按任务快照读取原件记录和向量模型供应商，解析 OpenAI 兼容入口。
2. 置 `converting`，流式转发原件到 markitdown，取回 Markdown 正文。
3. 置 `splitting`，调用 `textsplit.Split`；结果为空按 `empty_content` 失败。
4. 置 `embedding`，按每批 20 条调用 `embedding.Embed`。返回维度与任务快照不一致整批失败，不写入部分向量。
5. 置 `publishing`，在一个事务内完成：持文档行锁核验 `processing_id` 与执行状态、删除本文档的全部分段、写入本批次分段、更新 `segment_batch_id` 与 `segment_count`、`servertask.LockExecution`。

分段写入进入发布事务后，"远端完整写入后 Go 再核验分段数量"这段补偿逻辑随之删除——数量核验的存在理由是跨进程写入不可信，现在写入方与发布方是同一个事务。

分段 ID 保持 `uuid5(processingID, position)` 的确定性构造，重放不产生重复记录。Go 1.27 标准库的 `uuid` 包没有 v5，在 `internal/common/uuid.go` 补一个按 RFC 9562 §5.5 实现的 `NewUUIDv5(namespace UUID, name string) UUID`，约 20 行。

`DocumentProcessing.Retry` 的连接检查保留，检查对象从 Haystack 改为 markitdown：转换仍在独立进程，连接失败直接保存失败状态而不投递任务的语义不变。

### 9.2 向量化

新增 `internal/integration/embedding`，提供 `Embed(ctx, credential, model string, dimension int, inputs []string) ([][]float32, error)`：

- 请求 `POST {baseURL}/embeddings`，body 为 `{model, input, dimensions}`，Bearer 鉴权。
- 每批 20 条，按供应商兼容接口允许的最小批量。
- 供应商入口解析失败返回 `embedding_model_unavailable`，调用失败返回 `embedding_failed`，维度不符返回 `embedding_dimension_mismatch`。

凭据仍在执行任务时由 Action 从供应商记录解析，不写入任务载荷。

### 9.3 状态与错误码清理

不再有生产者的状态和错误码直接删除，不保留占位：

| 删除项 | 位置 |
|---|---|
| `KnowledgeDocumentExtracting`、`KnowledgeDocumentRecognizing`、`KnowledgeDocumentIndexing` | `internal/domain/knowledge_document.go`、`internal/appservice/types_knowledge_document.go` |
| `recognition_required`、`encrypted_file` 分支 | `internal/appservice/direct_backend_knowledge_document.go` |
| `error.knowledge_recognition_required`、`error.knowledge_file_encrypted` | `internal/i18n/i18n.go`、`locales/zh-CN.json`、`locales/en-US.json` |
| `PageNumber`、`SourceLabel` | 分段查询契约、`knowledge_segments.meta` 写入、前端分段展示 |

保留的执行状态为 `initial / queued / fetching / converting / splitting / embedding / publishing / succeeded / failed / cancelled`。markitdown 不返回结构化失败原因，加密原件与解析异常统一归入 `parse_failed`；扩展名白名单校验仍在 Go 侧前置，不在范围内返回 `unsupported_file`。改后执行 `wails3 generate bindings -clean=true -ts -i`。

### 9.4 删除清单

| 对象 | 位置 |
|---|---|
| Haystack 服务与测试 | `haystack/` 整个目录 |
| 内部传输契约包 | `internal/integration/knowledgeprocessing/`（类型迁入 `actions/knowledgebase`） |
| Task | `Taskfile.yml` 的 `run:haystack`、`test:haystack` |
| 环境变量 | `.env.example` 的 `HAYSTACK_PORT`、`HAYSTACK_URL` |
| 配置项 | `internal/config/server/config.go` 的 `HaystackURL` 字段、环境覆盖、地址校验及 `config_test.go` 对应用例 |
| 文档 | `internal/server-deployment.md` 中 Haystack 相关部署说明和 `haystack` schema 描述（`db:ensure` 实际未创建该 schema，说明本身已过时） |

### 9.5 验收

- `wails3 task test:server`：重试幂等与任务标识替换、全状态重试、参数快照与落库维度、原件缺失失败、失败保留上次成功分段、删除文档与知识库时分段同步清除、企业隔离。
- `wails3 task build:server` 与 `wails3 task build:docker CGO_ENABLED=0`。
- 界面：上传中文 DOCX、PDF、XLSX、PPTX 各一份，确认状态流转、分段数量和分段正文正确；人工重试一次确认批次替换；停止 markitdown 容器后重试，确认保存连接失败状态且不投递任务。
- 确认 `run:server` 加共享 markitdown 容器即可完成全流程，不再需要 `run:haystack`。

## 10. 后续

向量召回尚未接线：`internal/integration/knowledgeretrieval` 已提供多查询 RRF 融合和游标读取，但 `Source{Retrieve, Read}` 没有任何实现，应用层也未注入 `KnowledgeSearch`。后续 PR 按下述边界接入 pgvector：

- 把分段存储收敛为一个接口（批次写入、按文档删除、向量检索、按位置读取），pgvector 实现先落地。`knowledgeretrieval.Source` 与该接口共同构成将来切换向量存储的唯一改动面。
- 检索 SQL 必须带 `embedding_dimension` 谓词才能命中 halfvec 部分索引。
- 本地问答路在进入融合前按条目折叠，`documentId` 与 `segmentId` 都使用问答条目编号，位置固定为 1。

Qdrant 暂不引入。触发条件是单企业分段规模进入千万级、或确认需要 sparse 与 dense 原生混合检索；在此之前跨库双写会破坏删除文档时的事务一致性，并把企业隔离从 SQL 条件降级为 payload 过滤。

## 11. 工期

| PR | 内容 | 估算 |
|---|---|---|
| PR 1 | 字符分段 | 1 天 |
| PR 2 | 分段阅读下沉 | 1 天 |
| PR 3 | markitdown 容器与转换客户端 | 1 天 |
| PR 4 | 流水线切换与删除 Haystack | 2 到 3 天 |

净增 Go 约 1200 到 1500 行（含测试），新增 Python 约 50 行，删除 Python 655 行和 Go 传输层 356 行。
