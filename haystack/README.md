# 知识文档处理

当前实现自动解析、字符分段、分段查看和重新处理，不包含分段编辑、OCR、向量生成和检索召回。

## 运行

从仓库根目录执行 `wails3 task migrate`，再分别运行 `wails3 task run:haystack` 和 `wails3 task run:server`。`.env` 的 `HAYSTACK_URL` 必须指向当前工作区 `HAYSTACK_PORT` 对应的服务。Haystack 固定连接当前工作区的 PostgreSQL 数据库，HTTP 请求不接受数据库连接参数。

上传完成后，Cervi 在激活原件的同一事务中保存文档和可靠任务。知识处理使用独立 Worker Pool；Worker 流式读取本地或对象存储原件，交给 Python 解析和分段。Python 在临时目录处理文件，退出时清理。完整批次写入后，Go 核验当前任务、租约及分段数量，再发布批次。每个处理任务只执行一次，失败后直接记录失败状态与原因，不自动重试。文档列表在所有状态下均开放手动重试；先在三秒内检查服务连接，连接失败只保存失败状态，连接成功才使用知识库当前配置创建新任务。

## 数据归属与状态

- 原始文件仍由 Cervi 文件管理保存。
- `knowledge_documents` 只保存来源、处理状态、任务标识、参数快照、已发布分段批次与数量，不保存分段正文。
- 分段正文只保存在 `public.knowledge_segments.content`。Python 负责处理和查询，Cervi 迁移定义表结构；删除文档或知识库时，Go 在业务删除事务中同步删除分段。双方通过同一文档行锁协调删除和迟到写入。
- 分段表与业务表使用同一数据库的 `public` schema，由 Cervi 统一迁移；它是应用自定义存储，不由官方 `PgvectorDocumentStore` 自动建表。Haystack 负责文档处理，不要求独立 schema。
- 数据库只保存一个技术状态 `status`：`initial / queued / fetching / converting / extracting / recognizing / splitting / embedding / indexing / publishing / succeeded / failed / cancelled`。应用服务把中间处理步骤映射为产品状态 `running`，其他终态分别映射；产品展示状态不单独落库。失败原因码保存在文档中，失败阶段写入处理日志。
- `processing_id` 标识一次处理任务；人工重试生成新标识，替代此前任务。重试期间保留已发布分段，新批次完整写入后在业务事务中切换批次并清理旧正文；失败继续保留上次成功结果。`segment_batch_id` 指向完整发布的分段结果，不能用执行状态代替结果是否可用。

## 格式和分段

支持 TXT、Markdown、HTML、PDF、DOCX、PPTX、XLSX、CSV、JSON。JSON 校验结构后保留完整 JSON 文本；XLSX 保留文本单元格的前导零和工作表名称。PDF 记录真实页码；带图片且无可提取文字的页面返回需要文字识别，加密和空白原件明确失败。

长度和重叠均按 Unicode 字符计数。Haystack 递归切分提供段落边界，最终从原文连续区间统一添加一次重叠，保证长度上限及正文可还原。换行统一为 `\n`，不折叠代码缩进。分段 ID 由任务标识与文档内序号确定，重放不会生成重复记录。

## 分段查询与召回定位

对外业务接口：`GET /api/knowledge-bases/:knowledgeBaseID/documents/:documentID/segments`。

- 普通阅读传 `segmentBatchId`、`page`、`pageSize`，默认每页 20 段。
- 从召回定位传 `segmentBatchId`、`anchorSegmentId`、`pageSize`，不同时传页码。后端根据该分段之前的实际记录数量计算所在页，返回该页及 `anchorSegmentId / anchorPosition`。
- 响应包含 `segmentBatchId`、`page: { number, size, total }` 和 `segments`。每段包含稳定 `id`、文档内 `position`、`content`、`characterCount` 及真实来源信息。
- 同一轮阅读固定批次；不存在的锚点、已替换批次或删除来源明确报错，不跳到其他分段。前端弹窗接收文档、批次和分段 ID，首次定位高亮，然后双向滚动加载。页码由服务端计算，召回结果不需要缓存页码。

后续召回结果至少携带 `knowledgeBaseId / documentId / segmentBatchId / segmentId`。全文索引和向量字段应基于同一分段记录扩展，不能另建一份正文。当前表是 Python 管理的分段存储，尚未接入 `PgvectorDocumentStore` 或 Retriever；后续接入时需实现对应适配契约，并明确向量维度及索引发布状态。

## 验证

`wails3 task test:server` 重建工作区测试库并运行 Go 测试；随后执行 `wails3 task test:haystack`，验证真实文件解析、逐字符还原、批次重放、HTTP 上传处理、企业隔离、删除及锚点分页。
