/** 将 ODT 的正文、表格和内嵌图片转换为语义化 HTML。 */
import JSZip from "jszip"

/** 读取 ODT 内容与样式并保留内容阅读所需的文档结构。 */
export async function convertODT(bytes: Uint8Array): Promise<string> {
  const zip = await JSZip.loadAsync(bytes)
  const content = await zip.file("content.xml")?.async("string")
  if (!content) throw new Error("ODT content.xml is missing")
  const parser = new DOMParser()
  const xml = parser.parseFromString(content, "application/xml")
  if (xml.querySelector("parsererror")) throw new Error("Invalid ODT XML")
  const stylesXML = await zip.file("styles.xml")?.async("string")
  const styles = new Map<string, { bold: boolean; italic: boolean }>()
  for (const source of [stylesXML ? parser.parseFromString(stylesXML, "application/xml") : null, xml]) {
    for (const style of Array.from(source?.getElementsByTagName("style:style") ?? [])) {
      const properties = style.getElementsByTagName("style:text-properties")[0]
      const parent = styles.get(style.getAttribute("style:parent-style-name") ?? "")
      styles.set(style.getAttribute("style:name") ?? "", {
        bold: properties?.getAttribute("fo:font-weight") === "bold" || Boolean(parent?.bold),
        italic: properties?.getAttribute("fo:font-style") === "italic" || Boolean(parent?.italic),
      })
    }
  }
  const body = xml.getElementsByTagName("office:text")[0]
  if (!body) throw new Error("ODT text body is missing")
  const html = document.implementation.createHTMLDocument("")

  /** 递归转换正文节点，通过 DOM 写入文本避免拼接未转义内容。 */
  async function append(node: Node, parent: HTMLElement): Promise<void> {
    if (node.nodeType === Node.TEXT_NODE) { parent.append(html.createTextNode(node.textContent ?? "")); return }
    if (node.nodeType !== Node.ELEMENT_NODE) return
    const element = node as Element
    const tag = element.tagName
    if (["text:tracked-changes", "office:annotation", "text:sequence-decls", "text:soft-page-break"].includes(tag)) return
    if (tag === "draw:image") {
      const path = element.getAttribute("xlink:href")?.replace(/^\.\//, "") ?? ""
      const image = zip.file(path)
      if (image) {
        const extension = path.split(".").pop()?.toLowerCase()
        const mime = extension === "jpg" || extension === "jpeg" ? "image/jpeg" : extension === "gif" ? "image/gif" : "image/png"
        const img = html.createElement("img")
        img.src = `data:${mime};base64,${await image.async("base64")}`
        img.alt = element.parentElement?.getAttribute("draw:name") ?? ""
        parent.append(img)
      }
      return
    }
    if (tag === "text:s") { parent.append(html.createTextNode(" ".repeat(Number(element.getAttribute("text:c") ?? 1)))); return }
    if (tag === "text:tab") { parent.append(html.createTextNode("\t")); return }
    const tags: Record<string, string> = { "text:p": "p", "text:span": "span", "text:list": "ul", "text:list-item": "li", "text:a": "a", "text:line-break": "br", "table:table": "table", "table:table-row": "tr", "table:table-cell": "td", "table:table-header-rows": "thead" }
    const targetTag = tag === "text:h" ? `h${Math.min(6, Math.max(1, Number(element.getAttribute("text:outline-level") ?? 1)))}` : tags[tag]
    const target = targetTag ? html.createElement(targetTag) : parent
    if (target !== parent) parent.append(target)
    if (tag === "text:a") target.setAttribute("href", element.getAttribute("xlink:href") ?? "")
    if (tag === "table:table-cell") {
      for (const [source, attribute] of [["table:number-columns-spanned", "colspan"], ["table:number-rows-spanned", "rowspan"]]) {
        const value = element.getAttribute(source)
        if (value) target.setAttribute(attribute, value)
      }
    }
    let textTarget = target
    const style = styles.get(element.getAttribute("text:style-name") ?? "")
    for (const [enabled, name] of [[style?.bold, "strong"], [style?.italic, "em"]] as const) {
      if (enabled) { const wrapper = html.createElement(name); textTarget.append(wrapper); textTarget = wrapper }
    }
    for (const child of Array.from(element.childNodes)) await append(child, textTarget)
    const repetitions = Number(element.getAttribute(tag === "table:table-row" ? "table:number-rows-repeated" : "table:number-columns-repeated") ?? 1)
    if (target !== parent) for (let i = 1; i < repetitions; i++) parent.append(target.cloneNode(true))
  }
  for (const child of Array.from(body.childNodes)) await append(child, html.body)
  return html.body.innerHTML
}
