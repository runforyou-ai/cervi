# 知识来源扩展方案：在线文档与网页导入

日期：2026-09-15。状态：已实施，验证记录见第 10 节。

本文承接 [知识文档处理管线 Go 化与 Haystack 移除方案](knowledge-go-pipeline-plan.md) 第 5.3 节中「URL 来源需单独设计后再开」，以及 [知识库 Agent Tool 方案](knowledge-base-agent-tool-plan.md) 中「需要导入本地知识库时另行定义有明确范围、来源标识和完成状态的导入任务」。索引与召回链路复用 [问答知识库索引与召回方案](knowledge-qa-index-plan.md) 已建立的来源化分段体系。

## 1. 现状与目标

标准知识库的文档只能由上传原件创建：`knowledge_documents.file_id` 非空且唯一，文档名取自 `files.original_name`，处理流程固定为读取原件、markitdown 转换、切分、向量化、发布批次。企业写在网页上的产品说明、帮助中心条目，以及成员随手整理的规范，都需要先导出成文件才能入库。

目标：标准知识库支持三类文档来源，索引、召回、分段阅读和 Agent 检索对三者一致。

| 来源 | 内容获取方式 | 交付原文 |
|---|---|---|
| `file` | 上传原件，每次处理时由 markitdown 转换 | 原件 |
| `text` | 成员在页面内编写 | `knowledge_document_contents` 中的 Markdown |
| `web` | 创建与重新抓取时按 URL 抓取单个页面 | `knowledge_document_contents` 中的 Markdown 快照 |

网页文档的交付原文是快照，不是 URL：只有创建、显式重新抓取，以及尚无快照时的重试会出网，重试与知识库重建读取已有快照。远端暂时不可达或页面已改版时，已建立的检索能力不随之失效或漂移。

不在本轮范围：站点批量抓取与链接跟随、定时重抓、需要登录态或鉴权的页面、抓取时执行 JavaScript、YouTube 与音频来源、Azure 转换器。

## 2. 来源模型

`knowledge_documents` 的变更：

| 变更 | 说明 |
|---|---|
| 新增 `source_kind varchar(16) NOT NULL DEFAULT 'file'` | `file`、`text` 或 `web`，与分段表表示来源种类的 `source_type` 分属两层 |
| 新增 `title text NOT NULL DEFAULT ''` | 在线文档与网页文档的名称 |
| 新增 `source_url text NOT NULL DEFAULT ''` | 网页文档的页面地址 |
| `file_id` 改为可空 | 在线文档与网页文档没有原件 |
| `knowledge_documents_file_unique` 改为 `WHERE file_id IS NOT NULL` 的部分唯一索引 | 上传重复提交的幂等语义不变 |
| 新增 `knowledge_documents_source_url_unique` 部分唯一索引 `(knowledge_base_id, source_url) WHERE source_kind = 'web'` | 同一知识库内同一页面只保留一份文档 |

新增 `domain.KnowledgeDocumentTitleMaxLength = 120`，与知识库名称同量级；问答主问题当前没有长度上限，不作为参照。

依赖原件的三处查询按来源抽象改写，`file` 行为不变：

| 位置 | 现状 | 改后 |
|---|---|---|
| `document_query.go` 的文档查询 | `JOIN files f ON f.id = kd.file_id`，名称取 `f.original_name` | `LEFT JOIN files`，再 `LEFT JOIN knowledge_document_contents dc`；名称 `COALESCE(NULLIF(kd.title, ''), f.original_name)`，非文件来源的内容类型固定 `text/markdown`、字节数取 `octet_length(dc.content)` |
| `document_query.go` 的关键词过滤 | `f.original_name ILIKE ?` | 同时匹配 `kd.title` |
| `segment_store.go` 的 `publishedSegments` 与 `segmentColumns` | 文档路 `JOIN files`，来源名称取 `f.original_name` | `LEFT JOIN files`，来源名称同上取值 |

`publishedSegments` 是内连接，不改写时 `text` 与 `web` 即使成功发布分段，也会被向量召回、词法召回和游标阅读整行过滤掉，因此与来源抽象放在同一提交。

`knowledgeDocumentFromAction` 当前按 `filepath.Ext(record.Name)` 推导 `Format`，在线文档与网页文档的名称没有扩展名，改为按 `source_kind` 取值：`file` 保持按扩展名推导，`text` 与 `web` 固定 `.md`。

## 3. 正文存储

新增 `knowledge_document_contents`，以 `document_id` 为主键，保存 `content` 与 `updated_at`，参照 `knowledge_qa_contents` 的职责边界：

- `text` 来源保存成员编辑的 Markdown。
- `web` 来源保存最近一次抓取转换得到的 Markdown 快照。
- `file` 来源不写入该表，正文每次处理时从原件转换。

分段正文仍只写入 `knowledge_segments`，不由该表派生。`DeleteDocumentAction` 与 `DeleteKnowledgeBaseAction` 在删除文档行之前先删除对应正文，与知识库删除中先删问答内容再删条目的顺序一致。

## 4. 处理流程

`ProcessInput` 增加 `SourceKind` 与 `FetchPage`，`ProcessDocumentAction` 在读取原件之前按来源分支：

| 来源与触发 | `fetching` | `converting` | `splitting` 输入 |
|---|---|---|---|
| `file` | 打开原件 | 送 markitdown 转换 | 转换结果 |
| `text` | 跳过 | 跳过 | `knowledge_document_contents` 正文 |
| `web`，创建、重新抓取或尚无快照 | HTTP GET 抓取页面字节 | 送 markitdown 转换并写入快照 | 转换结果 |
| `web`，重试或知识库重建且已有快照 | 跳过 | 跳过 | `knowledge_document_contents` 快照 |

`text` 的状态流转为 `queued → splitting → embedding → publishing → succeeded | failed`，与问答条目一致。正文为空按 `empty_content` 失败。切分、向量化、发布事务、`processing_id` 校验与批次替换语义不变。

快照在转换成功后立即写入，写入事务持文档行锁并核验 `processing_id` 与执行状态，条件不满足时放弃写入，语义与发布事务相同：重新抓取与旧任务并发时，失效任务既不覆盖新快照也不写回已删除文档的孤立正文。转换成功而向量化失败时，文档详情显示新快照、召回仍是上一批次，两者在下一次发布前可以不一致。

触发规则：

- 创建在线文档：保存事务内写入正文、置 `queued` 并投递任务。
- 编辑在线文档正文：替换正文并投递新任务；正文未变化且当前状态为 `succeeded` 时不投递。
- 只改名称：不投递任务，不影响已发布批次。
- 创建网页文档与重新抓取：投递带 `FetchPage` 的任务。修改网页文档的 URL 按重新抓取处理。
- 重试：不出网，按当前快照重新索引；`web` 尚无快照时按创建处理。
- 知识库向量模型或分段参数变更触发的重建：三类来源一并重建，`web` 读快照，`reindex` 不出网。

`DocumentProcessing.Retry` 当前无条件调用 markitdown 连接检查，改为按来源判定：`file` 与出网的 `web` 保留检查，`text` 与读快照的 `web` 直接投递。

新增失败原因码，同时进入 `knowledgeIndexPresentation` 的映射与 `internal/i18n` 的中英文词条：

| 原因码 | 触发条件 |
|---|---|
| `url_unreachable` | 解析失败、连接失败、超时或响应状态非 2xx |
| `url_content_unsupported` | 响应内容类型不是 HTML 或纯文本 |
| `url_content_too_large` | 响应体超过上限 |

`url_unreachable` 覆盖多种原因，文案按「页面读取失败」表达，不写成单一网络原因。

## 5. 网页抓取边界

新增 `internal/integration/webfetch`，提供 `Fetch(ctx context.Context, target string) (Page, error)`，返回送转换服务使用的文件名与响应体：

- 只接受 `http` 与 `https` 的绝对地址，其余在保存 URL 时即拒绝；保存时去掉 fragment，保留 query，不改写路径尾斜杠与参数顺序。
- 超时 30 秒，最多跟随 5 次重定向，响应体上限 10 MB。
- 按响应内容类型选择转换文件名：`text/html` 与 `application/xhtml+xml` 用 `.html`，`text/plain` 用 `.txt`，其余返回 `url_content_unsupported`。内容类型缺省时按 `url_content_unsupported` 处理，不做内容嗅探。PDF 与 Office 文件仍按上传原件入库。
- 固定 User-Agent 标识来自 Cervi 知识库导入。
- 不执行 JavaScript。依赖前端渲染的帮助中心会得到空壳并以 `empty_content` 失败，属于本轮已知限制。浏览器渲染服务暂不引入：触发条件是出现必须导入的客户端渲染站点，在此之前多拉一个 Chromium 量级容器的部署成本高于收益，静态抓取已覆盖服务端渲染与预渲染的文档站点，其余内容可以另存为 HTML 上传或用在线文档录入。引入时只替换 `webfetch.Client.Fetch` 的实现并增加对应配置，快照语义与处理分支不变。
- HTML 页面在送转换前先按 `golang.org/x/net/html/charset` 的判定顺序（BOM、页面内声明、响应头 charset）解码为 UTF-8，`text/plain` 同样解码；判定失败时保留原字节。
- 解码后由 `github.com/go-shiori/go-readability` 提取正文主体，导航、侧栏、页脚不进入正文，正文内的标题层级、表格和代码块保留，页面内的相对链接改写为绝对地址。提取结果包成完整 HTML 文档后送转换服务。解析失败或提取不到正文时送整页，前端渲染的空壳页据此仍按 `empty_content` 失败。
- 提取按整页正文结构判定，不保证逐站点准确：结构异常的站点可能保留部分页面结构或丢失次要区块。判定规则由 readability 提供，本轮不在其之上叠加站点规则。

不做内网地址与私有网段拦截：私有化部署的企业内网文档站点是本能力的实际来源之一。SSRF 与抓取频率控制列入上线前的安全与容量专项。

## 6. 接口

`Backend` 新增方法，各自携带 `cervi:route` 指令：

```text
POST   /knowledge-bases/:knowledgeBaseID/text-documents            { groupId, title, content }   status=201
POST   /knowledge-bases/:knowledgeBaseID/web-documents             { groupId, title, sourceUrl } status=201
GET    /knowledge-bases/:knowledgeBaseID/documents/:documentID/content
PUT    /knowledge-bases/:knowledgeBaseID/documents/:documentID     { title }
PUT    /knowledge-bases/:knowledgeBaseID/documents/:documentID/content { title, content }
POST   /knowledge-bases/:knowledgeBaseID/documents/:documentID/refetch  { sourceUrl }
```

来源限制与原件预览只对 `file` 有效的规则对齐：正文读取与名称更新只对 `text` 与 `web` 有效，正文更新只对 `text` 有效，重新抓取只对 `web` 有效，其余组合返回不支持。

保存期校验：

- 标题去空白后非空且不超过 `KnowledgeDocumentTitleMaxLength`。
- 在线文档正文去空白后非空，保存期即拒绝，不先建文档再以 `empty_content` 失败。
- URL 按第 5 节规则校验；同一知识库内重复的网页地址按已存在拒绝。

`KnowledgeDocument` 增加 `sourceKind` 与 `sourceUrl`。文档列表、移动分组、删除、重试与分段查询保持当前契约。改后执行 `wails3 generate bindings -clean=true -ts -i`。

## 7. 前端

编辑器使用 Tiptap v3 与官方 `@tiptap/markdown`（3.7.0 起提供双向 Markdown 支持），按前端约定锁定精确版本。编辑器组件当前只被知识库使用，放在 `features/knowledge-base`。

- 扩展集：StarterKit 3.31.3 已包含标题、列表、引用、代码块和链接，另外单独引入表格扩展（含表头、行、单元格）。
- Tiptap 官方文档说明 Markdown 表格每个单元格只允许一个子节点，编辑器按此限制单元格内容，粘贴内容按同一规则规范化。
- 不注册图片节点，粘贴与拖入的图片因此不会进入文档；本轮在线文档不支持图片。
- 正文必填校验同步到隐藏原生控件，由浏览器把校验显示在控件上，与 `shouldUseNativeValidation` 的既有做法一致，不使用 Toast 或 `FieldError`。
- 文档列表页的「上传文档」改为下拉菜单：上传文件、编写文档、导入网页。
- 新增路由 `.../documents/new` 与 `.../documents/:documentId/edit`，两条都排在 `.../documents/:documentId` 之前，与问答的 `/qa/new` 顺序一致。页面结构对齐问答表单页：名称字段加必填标记，底部操作区使用 `space-y-9`。
- 网页导入用 Dialog 收集地址与名称，保存后回到列表并按现有轮询显示状态流转。
- 文档表格增加「来源」列；类型列对 `text` 与 `web` 显示 MD，大小列显示正文字节数。`text` 行把「编辑」作为直接展示的描边按钮，与问答列表一致；`web` 行的三点菜单提供「重新抓取」；三类来源都保留「重试」。处理中允许覆盖投递，与现有重试一致，不额外禁用。
- 文档详情页对 `text` 与 `web` 展示正文，使用现有 Markdown 渲染；`file` 保持原件预览。网页文档在标题下展示来源地址；正文尚未写入时按索引状态说明抓取中或失败原因，不呈现空白正文，处理期间正文按轮询刷新。
- 编辑页在表单未编辑时跟随最新读取到的名称与正文。网页文档的名称修改接口已提供，本轮界面不提供改名入口。
- 编辑器把 Markdown 解析成文档时会产生结构规范化，正文只在用户实际编辑后回写表单，未改动的文档保存时不重新索引。
- 文档名称在客户端按 `KnowledgeDocumentTitleMaxLength` 限制长度，与知识库名称的校验方式一致。
- 正文读取在 `resourceKeys` 中单列一个 key，详情页与编辑页共用。

## 8. 交付顺序

| 提交 | 内容 |
|---|---|
| 1 | 来源抽象：迁移与正文表、模型、文档查询与 `publishedSegments` 改写、`Format` 取值、删除链路，`file` 行为不变 |
| 2 | 在线文档：创建与编辑接口、处理分支、前端编辑器、路由与来源列 |
| 3 | 网页导入：`webfetch`、创建与重新抓取、快照写入、失败原因码与前端入口 |
| 4 | 文档更新：本文与 `knowledge-go-pipeline-plan.md` 第 10 节 |

## 9. 验收

`wails3 task test:server`：

- 三类来源创建后投递任务、分段数量与位置正确、向量与词法词元落库。
- 在线文档编辑正文后替换批次、清除旧分段；只改名称不投递；旧 `processing_id` 的任务在阶段更新时退出。
- 网页抓取成功后写入快照并发布分段；重新抓取替换快照与批次；抓取失败保留上一批次与上一快照。
- 重试与知识库重建读取快照且不出网；`web` 尚无快照时重试出网。
- 旧任务在新任务完成重新抓取后返回时不覆盖新快照；文档删除后在途任务不写回正文。
- `text/plain` 响应按 `.txt` 转换，正文换行保留且字面 `<tag>` 不被当作标记；非 HTML 内容类型返回 `url_content_unsupported`。
- 删除文档与删除知识库清除分段与正文；企业隔离；同一知识库重复 URL 被拒绝。
- 检索测试与 `search_knowledge` 对三类来源返回一致的分段正文、上下文和游标读取结果，`text` 与 `web` 的来源名称为文档标题。

前端构建、类型检查与测试通过。界面验证：工具栏可插入的块结构在序列化后能被切分规则识别，编辑、保存、重新打开后结构保持（覆盖表格、列表、代码块组合）；网页导入的状态流转与失败提示；来源列与三点菜单在三类来源下的展示。验证结束后清理本次启动的进程。

## 10. 验证记录

- `wails3 task test:server` 通过。新增 `TestKnowledgeDocumentSourceWithoutFile` 覆盖无原件文档在列表、详情、关键词过滤和混合召回中按标题呈现；`TestKnowledgeTextDocumentLifecycle` 覆盖在线文档的创建投递、索引发布、只改名称不投递、正文未变化不投递、正文变化替换批次并清除旧分段、来源限制与删除清理；`TestKnowledgeWebDocumentLifecycle` 覆盖地址校验、重复导入拒绝、首次抓取写快照、重试读快照不出网、重新抓取替换快照与批次、更新页面地址、抓取失败保留上一批次与上一快照、在线文档不支持重新抓取。`internal/integration/webfetch` 的单元测试覆盖地址规范化、HTML 与纯文本的文件名选择、非 2xx、内容类型不支持、体积上限与连接失败。
- 前端 `common:build:frontend` 与 `test:frontend` 通过。
- 2026-09-16 网页正文提取与编码：`internal/integration/webfetch` 新增单元测试，覆盖带导航、侧栏、页脚的帮助中心页面提取后保留标题、表格与代码块并去掉页面结构，空壳页回退整页，GBK 页面按响应头与页面内声明两种判定解码，以及 GBK 页面经 `Fetch` 全程解码。真实页面对照 markitdown 转换结果：`https://zh.wikipedia.org/wiki/Markdown` 由 37844 字符降到 21942 字符，跳转到内容、分类索引、互助客栈、隐私政策、维基媒体基金会等页面结构不再出现，正文与 CommonMark 等内容保留；`https://docs.python.org/zh-cn/3/tutorial/introduction.html` 由 14253 字符降到 11505 字符，导航、上一页、页脚版权不再出现，`## `、`### ` 标题与围栏代码块保留。`wails3 task test:server` 通过。
- 2026-09-15 浏览器界面验证（通义千问 `qwen3.7-text-embedding` 1536 维与 `qwen3-rerank`）：「添加文档」下拉包含上传文件、编写文档、导入网页三项；在线编写「在线编写验证」经工具栏插入标题、正文、无序列表和 3×3 表格后保存，约 9 秒内完成索引，列表来源列显示「在线编写」、类型列显示 MD、主操作为「编辑」，重新打开后标题、列表与表格结构保持一致；名称与正文的必填提示均由浏览器显示在对应控件上；导入 `https://zh.wikipedia.org/wiki/Markdown` 后来源列显示「网页导入」，约 30 秒内完成索引并落库 97 段，详情页正文按 Markdown 渲染；网页文档的「重新抓取」可用，在线文档的同一菜单项禁用。
- 界面验证期间发现并修复：承载正文必填的隐藏控件带 `readonly` 时不参与约束校验，且 `onFocus` 转移焦点会取消浏览器气泡；改为透明、覆盖编辑区且接收字段 ref 的控件后提示正常显示。
- 代码审查后的修复与复验：编辑页在表单未编辑时同步最新名称与正文并重建编辑器；网页详情页在处理期间轮询正文、无正文时按索引状态显示占位并展示来源地址；重试与重新抓取一并失效正文缓存；「编写文档」入口保留列表筛选与页码；导入弹窗每次打开重置；尚无快照的网页重试按出网处理并先检查转换服务连接；回滚迁移先清除非上传来源的文档与分段，在含在线文档和网页文档的开发库上验证了回滚与重新前滚。
- 文档名称的客户端长度限制与编辑器回写时机的复验：名称超过 120 个字符时浏览器在输入框上提示并阻止提交；打开已发布文档的编辑页不做改动直接保存，`processing_id` 与保存前一致，未投递新的索引任务。
- 验证结束后已停止本次启动的服务端，确认端口释放。
