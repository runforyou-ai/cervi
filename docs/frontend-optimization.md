# 前端优化计划

日期：2026-09-27。

本文记录前端代码审查发现的待办优化事项，按 PR 分组，某项完成后直接删除对应内容。会话时间线的窗口裁剪与虚拟化已列入 [路线图](roadmap.md)「实时同步」的「会话列表虚拟化与窗口裁剪」，按其启动条件开启，本文不重复。

## 首屏与包体积

构建前后记录首屏与访客端产物体积，逐项验证收益。

- `api/index.ts` 以 `export *` 聚合全部绑定模型，首屏预加载约 212KB 的 API chunk。为 `src/api` 与 `bindings` 声明无副作用，使打包按实际使用裁剪；验证生成的模型无副作用。
- 语言包按命名空间懒加载，首屏只加载 common、auth、setup、connection、account，其余命名空间随各 feature 路由加载。
- 网站访客端 `markdown.js` 约 719KB，其中 parse5（rehype-raw）、shiki 语言表与 mermaid 文案不会用到。在 messenger 构建中移除这些依赖，或换用更轻的渲染组合。
- 知识库表格预览改用 `xlsx` 的 mini 构建。
- 字体只引入实际使用的字符子集。
- `@types/hast` 加入 devDependencies。
- 知识库文档预览的 docx、xlsx 解析移到 Web Worker，工作表按当前页签生成 HTML。
