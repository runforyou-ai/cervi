// Package textsplit 按字符长度和重叠切分正文。
package textsplit

import "strings"

// Segment 表示一段按文档顺序编号的正文。
type Segment struct {
	Position       int
	Content        string
	CharacterCount int
}

// Split 按长度上限切分正文，长度和重叠均按 Unicode 字符计数。
// 分段正文是规整换行后原文的连续区间，去除重叠后可逐字符还原。
// 正文为空白，或长度上限不大于重叠导致分段无法前进时，返回空结果。
func Split(text string, length, overlap int) []Segment {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if length <= overlap || strings.TrimSpace(normalized) == "" {
		return nil
	}
	runes := []rune(normalized)
	// 收缩后的分段长度至少为 max(overlap+1, length/2)。
	minimum := max(overlap+1, length/2)
	var segments []Segment
	for start := 0; start < len(runes); {
		end := min(start+length, len(runes))
		if end < len(runes) {
			// 从长度上限向前查找最靠后的边界字符。
		boundary:
			for index := end; index >= start+minimum; index-- {
				switch runes[index-1] {
				case '\n', '。', '！', '？', '；', ' ':
					end = index
					break boundary
				}
			}
		}
		segments = append(segments, Segment{
			Position:       len(segments) + 1,
			Content:        string(runes[start:end]),
			CharacterCount: end - start,
		})
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return segments
}
