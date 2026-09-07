/** 验证原始文档格式解析和不可信 HTML 的阅读隔离。 */
import assert from "node:assert/strict"
import { test } from "node:test"
import { JSDOM } from "jsdom"
import JSZip from "jszip"
import * as XLSX from "xlsx"

const dom = new JSDOM("")
Object.assign(globalThis, { window: dom.window, document: dom.window.document, DOMParser: dom.window.DOMParser, Node: dom.window.Node })
const { parseDocumentPreview, decodeDocumentText } = await import("../src/features/knowledge-base/document-preview-parser.ts")
const encode = new TextEncoder()

test("纯文本与 Markdown 扩展名保留原文，MDX 不执行组件", async () => {
  for (const extension of ["TXT", "PROPERTIES", "VTT", "MD", "MARKDOWN", "MDX"]) {
    const content = "# 阅读标题\n\n<Component />\n正文内容"
    const preview = await parseDocumentPreview(`sample.${extension}`, encode.encode(content))
    assert.equal(preview.kind, ["MD", "MARKDOWN", "MDX"].includes(extension) ? "markdown" : "text")
    assert.equal(preview.content, content)
  }
  assert.equal(decodeDocumentText(new Uint8Array([255, 254, 0x2d, 0x4e])), "中")
})

test("HTML/HTM 保留标题表格，移除脚本、事件和活动控件", async () => {
  for (const extension of ["HTML", "HTM"]) {
    const preview = await parseDocumentPreview(`sample.${extension}`, encode.encode('<h1>阅读标题</h1><table><tr><td>表格正文</td></tr></table><script>window.pwned=true</script><img src="x" onerror="alert(1)"><a href="javascript:alert(1)">链接</a><form><input value="秘密"></form>'))
    assert.equal(preview.kind, "html")
    assert.match(preview.content, /阅读标题/)
    assert.match(preview.content, /表格正文/)
    assert.doesNotMatch(preview.content, /<script|onerror|javascript:|<input/)
    assert.match(preview.content, /Content-Security-Policy/)
  }
})

test("XLS/XLSX 保留多个工作表和显示值，CSV 保留文本单元格", async () => {
  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([["名称", "金额"], ["年假", 12]]), "汇总")
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet([["第二个工作表"]]), "明细")
  for (const extension of ["xls", "xlsx"]) {
    const bytes = new Uint8Array(XLSX.write(workbook, { bookType: extension, type: "array" }))
    const preview = await parseDocumentPreview(`sample.${extension}`, bytes)
    assert.equal(preview.kind, "sheets")
    assert.deepEqual(preview.sheets.map((sheet) => sheet.name), ["汇总", "明细"])
    assert.match(preview.sheets[0].content, /年假/)
    assert.match(preview.sheets[1].content, /第二个工作表/)
  }
  const csv = await parseDocumentPreview("sample.csv", encode.encode('编号,内容\n001,"含逗号,的正文"'))
  assert.equal(csv.kind, "sheets")
  assert.match(csv.sheets[0].content, /001/)
  assert.match(csv.sheets[0].content, /含逗号,的正文/)
})

test("DOCX/ODT 保留段落和表格，ODT 保留图片", async () => {
  const docx = new JSZip()
  docx.file("[Content_Types].xml", '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>')
  docx.file("word/document.xml", '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>DOCX 正文</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>表格内容</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>')
  const result = await parseDocumentPreview("sample.docx", await docx.generateAsync({ type: "uint8array" }))
  assert.equal(result.kind, "html")
  assert.match(result.content, /DOCX 正文/)
  assert.match(result.content, /<table>/)
  const odt = new JSZip()
  odt.file("content.xml", '<office:document-content xmlns:office="urn:o" xmlns:text="urn:t" xmlns:table="urn:ta" xmlns:draw="urn:d" xmlns:xlink="urn:x"><office:body><office:text><text:h text:outline-level="1">ODT 标题</text:h><text:p>连续正文</text:p><table:table><table:table-row><table:table-cell><text:p>表格内容</text:p></table:table-cell></table:table-row></table:table><draw:frame><draw:image xlink:href="Pictures/pixel.png"/></draw:frame></office:text></office:body></office:document-content>')
  odt.file("Pictures/pixel.png", new Uint8Array([137, 80, 78, 71]))
  const preview = await parseDocumentPreview("sample.odt", await odt.generateAsync({ type: "uint8array" }))
  assert.equal(preview.kind, "html")
  assert.match(preview.content, /<h1>ODT 标题<\/h1>/)
  assert.match(preview.content, /<table>/)
  assert.match(preview.content, /data:image\/png;base64/)
})

test("PDF 传递原始数据；未知格式不伪造预览；损坏文件返回解析错误", async () => {
  const bytes = encode.encode("%PDF-1.7")
  const preview = await parseDocumentPreview("sample.PDF", bytes)
  assert.equal(preview.kind, "pdf")
  assert.deepEqual(preview.content, bytes)
  assert.deepEqual(await parseDocumentPreview("sample.unknown", bytes), { kind: "unsupported" })
  await assert.rejects(parseDocumentPreview("broken.docx", bytes))
})
