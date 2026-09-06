/** 为网站 Messenger 消息正文提供共享 React 渲染和生命周期接口。 */
import { createRoot, type Root } from "react-dom/client"
import { MessageMarkdown } from "../components/message-markdown"
import { messagePreview } from "../lib/message-preview"
import type { ConversationMessage } from "../api"

const roots = new Map<HTMLElement, Root>()

/** 在稳定的正文节点上更新完整消息或流式原文。 */
export function render(container: HTMLElement, message: Pick<ConversationMessage, "body" | "bodyFormat">, streaming = false) {
  if (message.bodyFormat !== "markdown") {
    unmount(container)
    container.textContent = message.body
    return
  }
  let root = roots.get(container)
  if (!root) {
    root = createRoot(container)
    roots.set(container, root)
  }
  root.render(<MessageMarkdown locale={document.documentElement.lang} streaming={streaming}>{message.body}</MessageMarkdown>)
}

/** 在消息节点被移除前释放它和后代正文的 React 资源。 */
export function unmount(container: Node) {
  for (const [element, root] of roots) {
    if (container === element || container.contains(element)) {
      root.unmount()
      roots.delete(element)
    }
  }
}

export { messagePreview as preview }
