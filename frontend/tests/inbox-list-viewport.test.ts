/** 在真实 React 提交周期中验证列表唯一滚动补偿与原邻居优先级。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import { JSDOM } from "jsdom"
import { act, createElement } from "react"
import { createRoot } from "react-dom/client"
import { useInboxListViewport } from "../src/features/inbox/use-inbox-list-viewport.ts"

test("上方插入、锚点移走、后继消失时仅补偿一次，并保留原像素偏移", async () => {
  const dom = new JSDOM('<div id="root"></div>')
  const oldWindow = globalThis.window
  const oldDocument = globalThis.document
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  let viewport: ReturnType<typeof useInboxListViewport>
  /** 挂载与页面一致的视口适配器，使用浏览器布局测量夹具。 */
  function Harness() {
    viewport = useInboxListViewport()
    return createElement("div", { ref: viewport.root }, createElement("div", { "data-slot": "scroll-area-viewport" }))
  }
  const root = createRoot(document.getElementById("root")!)
  try {
    await act(async () => root.render(createElement(Harness)))
    const container = document.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')!
    Object.defineProperty(container, "clientHeight", { configurable: true, value: 100 })
    container.getBoundingClientRect = () => ({ top: 100, bottom: 200 } as DOMRect)
    let ids = ["79", "80", "81", "82"]
    // 以行序和 scrollTop 模拟布局，检查的是最终滚动偏移。
    for (const id of ["new", ...ids]) {
      const row = document.createElement("button")
      row.dataset.inboxId = id
      row.getBoundingClientRect = () => ({ top: 100 + ids.indexOf(id) * 68 - container.scrollTop, bottom: 168 + ids.indexOf(id) * 68 - container.scrollTop } as DOMRect)
      if (id !== "new") container.append(row)
    }
    viewport!.positions.current = ids.map((id) => ({ id, positionCursor: `p${id}`, lastActivityAt: null }))
    container.scrollTop = 78
    const anchor = viewport!.capture()!
    assert.equal(anchor.id, "80")
    assert.deepEqual(anchor.neighbors.slice(0, 2), [{ id: "80", offset: -10 }, { id: "81", offset: 58 }])
    ids = ["new", ...ids]
    viewport!.restore(anchor, new Set(), false)
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 146)
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 146)
    ids = ["80", "new", "79", "81", "82"]
    viewport!.restore(anchor, new Set(["80"]), false)
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 146)
    container.querySelector('[data-inbox-id="81"]')!.remove()
    container.querySelector('[data-inbox-id="82"]')!.remove()
    ids = ["80", "new", "79"]
    viewport!.restore(anchor, new Set(["80"]), false)
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 214)
    Object.defineProperty(container, "clientHeight", { configurable: true, value: 0 })
    viewport!.restore(null, new Set(), true)
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 214)
    Object.defineProperty(container, "clientHeight", { configurable: true, value: 100 })
    await act(async () => root.render(createElement(Harness)))
    assert.equal(container.scrollTop, 0)
  } finally {
    await act(async () => root.unmount())
    Object.assign(globalThis, { window: oldWindow, document: oldDocument })
    dom.window.close()
  }
})
