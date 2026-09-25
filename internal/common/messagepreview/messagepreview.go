// Package messagepreview 生成会话列表和消息提醒使用的单行纯文本摘要。
package messagepreview

import (
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// MaxRunes 是摘要保留的最大字符数。
const MaxRunes = 200

// markdownParser 按 GFM 语法解析 AI 回复正文。
var markdownParser = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()

// Text 返回单行摘要：Markdown 正文提取块级文字并以空格分隔，其余正文保留原文；空白折叠后截取前 MaxRunes 个字符。
func Text(body string, markdown bool) string {
	if markdown {
		body = markdownText([]byte(body))
	}
	body = strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(body) <= MaxRunes {
		return body
	}
	return string([]rune(body)[:MaxRunes])
}

// markdownText 展开列表、引用和表格等容器块，逐块提取文字，原始 HTML 不计入。
func markdownText(source []byte) string {
	var parts []string
	pending := children(markdownParser.Parse(text.NewReader(source)))
	for len(pending) > 0 {
		node := pending[0]
		pending = pending[1:]
		switch node.Kind() {
		case ast.KindList, ast.KindListItem, ast.KindBlockquote, extast.KindTable, extast.KindTableHeader, extast.KindTableRow:
			pending = append(children(node), pending...)
		case ast.KindHTMLBlock, ast.KindThematicBreak:
		case ast.KindCodeBlock, ast.KindFencedCodeBlock:
			var code strings.Builder
			lines := node.Lines()
			for index := range lines.Len() {
				segment := lines.At(index)
				code.Write(segment.Value(source))
			}
			parts = append(parts, code.String())
		default:
			var block strings.Builder
			inlineText(&block, node, source)
			parts = append(parts, block.String())
		}
	}
	return strings.Join(parts, " ")
}

// children 返回节点的直接子节点。
func children(node ast.Node) []ast.Node {
	var nodes []ast.Node
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		nodes = append(nodes, child)
	}
	return nodes
}

// inlineText 按文档顺序写入行内文字，链接保留文字、图片保留替代文本、原始 HTML 不计入。
func inlineText(out *strings.Builder, node ast.Node, source []byte) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch current := child.(type) {
		case *ast.Text:
			out.Write(current.Segment.Value(source))
			if current.SoftLineBreak() || current.HardLineBreak() {
				out.WriteByte(' ')
			}
		case *ast.String:
			out.Write(current.Value)
		case *ast.AutoLink:
			out.Write(current.Label(source))
		case *ast.RawHTML:
		default:
			inlineText(out, child, source)
		}
	}
}
