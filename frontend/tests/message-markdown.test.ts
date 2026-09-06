/** 用实际发布给 Messenger 的共享组件验证正文和流式更新。 */
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { test } from "node:test"
import { JSDOM } from "jsdom"

const bundle = readFileSync(new URL("../../internal/publicweb/dist/markdown.js", import.meta.url), "utf8")

/** 创建只执行本地构建产物的 DOM 测试宿主。 */
function host(t: { after: (callback: () => void) => void }) {
  const dom = new JSDOM('<html lang="zh-CN"><body><div id="message"></div></body></html>', { runScripts: "outside-only", url: "https://cervi.test" })
  dom.window.eval(bundle)
  const api = dom.window.CerviMarkdown
  const container = dom.window.document.getElementById("message")!
  t.after(() => { api.unmount(container); dom.window.close() })
  return { api, container, window: dom.window }
}

/** 等待 React 完成异步提交后检查实际 DOM。 */
async function rendered(check: () => void) {
  const deadline = Date.now() + 2000
  while (true) {
    try { check(); return } catch (error) {
      if (Date.now() >= deadline) throw error
      await new Promise((resolve) => setTimeout(resolve, 5))
    }
  }
}

test("共享正文支持中文、GFM 表格、任务列表和代码原文复制", async (t) => {
  const { api, container, window } = host(t)
  let copied = ""
  Object.defineProperty(window.navigator, "clipboard", { value: { writeText: async (value: string) => { copied = value } } })
  api.render(container, '# 标题\n\n**重点**\n第二行\n\n- [x] 完成\n- [ ] 待办\n\n| 名称 | 结果 |\n| --- | --- |\n| 测试 | 成功 |\n\n```js\nconst a = "<安全>";\n```', "agent")
  await rendered(() => {
    assert.equal(container.querySelector("h1")?.textContent, "标题")
    assert.equal(container.querySelector("strong")?.textContent, "重点")
    assert.equal(container.querySelectorAll('input[type="checkbox"]').length, 2)
    assert.equal(container.querySelector("td")?.textContent, "测试")
    assert.equal(container.querySelector("pre code")?.textContent, 'const a = "<安全>";\n')
  })
  container.querySelector("button")!.click()
  await rendered(() => assert.equal(copied, 'const a = "<安全>";\n'))
})

test("流式输入修复未闭合语法并保留已完成块，结束后与历史正文一致", async (t) => {
  const { api, container } = host(t)
  const prefix = "已经完成的段落。\n\n"
  api.render(container, prefix + "**正在", "agent", true)
  await rendered(() => assert.equal(container.querySelector("strong")?.textContent, "正在"))
  const firstParagraph = container.querySelector("p")
  const body = prefix + '**正在生成**\n\n```ts\nconst answer = 42;\n```\n\n| 项目 | 值 |\n| --- | --- |\n| 答案 | 42 |'
  for (let length = prefix.length + 5; length <= body.length; length += 7) {
    api.render(container, body.slice(0, length), "agent", true)
    await new Promise((resolve) => setTimeout(resolve, 5))
    assert.equal(container.querySelector("p"), firstParagraph)
  }
  api.render(container, body, "agent", false)
  await rendered(() => {
    assert.equal(container.querySelector("strong")?.textContent, "正在生成")
    assert.equal(container.querySelector("pre code")?.textContent, "const answer = 42;\n")
    assert.equal(container.querySelector("td")?.textContent, "答案")
  })
  assert.equal(container.querySelector("p"), firstParagraph)
  const history = container.ownerDocument.createElement("div")
  container.appendChild(history)
  api.render(history, body, "agent")
  await rendered(() => assert.equal(history.querySelector(".message-markdown")?.innerHTML, container.querySelector(".message-markdown")?.innerHTML))
})

test("正文不执行 HTML 或危险 URL，安全链接保留浏览器语义", async (t) => {
  const { api, container } = host(t)
  api.render(container, '<script>alert(1)</script>\n\n<img src="x" onerror="alert(1)">\n\n[危险](javascript:alert%281%29) [本地](file:///etc/passwd) [网页](https://example.com)\n\n```html\n<script>example</script>\n```', "agent")
  await rendered(() => assert.equal(container.querySelectorAll("a").length, 3))
  assert.equal(container.querySelector("script, [onerror], img"), null)
  assert.equal(container.querySelector('a[href^="javascript:"], a[href^="file:"]'), null)
  const link = container.querySelector('a[href="https://example.com"]')!
  assert.equal(link.getAttribute("target"), "_blank")
  assert.equal(link.getAttribute("rel"), "noopener noreferrer")
  assert.equal(container.querySelector("code")?.textContent, "<script>example</script>\n")
})

test("人工和访客纯文本保留星号与换行，卸载后可重新使用正文节点", async (t) => {
  const { api, container } = host(t)
  api.render(container, "**AI**", "agent")
  await rendered(() => assert.equal(container.querySelector("strong")?.textContent, "AI"))
  api.render(container, "**原样**\n@成员 <标签>", "user")
  assert.equal(container.textContent, "**原样**\n@成员 <标签>")
  assert.equal(container.childElementCount, 0)
  api.render(container, "# 访客原文", null)
  assert.equal(container.textContent, "# 访客原文")
  assert.equal(container.childElementCount, 0)
  api.unmount(container)
  api.render(container, "**新回复**", "agent")
  await rendered(() => assert.equal(container.querySelector("strong")?.textContent, "新回复"))
})

test("列表与引用摘要从语法树提取文字，纯文本保持原意", (t) => {
  const { api } = host(t)
  assert.equal(api.preview("**原样**", "user"), "**原样**")
  assert.equal(api.preview("# 访客原文", null), "# 访客原文")
  assert.equal(api.preview("# 标题\n\n[链接](https://example.com) 和 `代码`", "agent"), "标题 链接 和 代码")
  assert.equal(api.preview("- 第一项\n- 第二项", "agent"), "第一项 第二项")
})

test("未完成链接在生成中不可跳转，脚注在多条消息之间保持独立", async (t) => {
  const { api, container } = host(t)
  api.render(container, "[阅读文档](https://exa", "agent", true)
  await rendered(() => assert.ok(container.textContent?.includes("阅读文档")))
  assert.equal(container.querySelector("a[href]"), null)
  const body = "脚注[^1]\n\n[^1]: 说明"
  api.render(container, body, "agent")
  const other = container.ownerDocument.createElement("div")
  container.append(other)
  api.render(other, body, "agent")
  await rendered(() => assert.equal(container.querySelectorAll('a[data-footnote-ref]').length, 2))
  const links = [...container.querySelectorAll('a[data-footnote-ref]')]
  assert.notEqual(links[0].getAttribute("href"), links[1].getAttribute("href"))
  for (const link of links) assert.ok(container.ownerDocument.getElementById(link.getAttribute("href")!.slice(1)))
})
