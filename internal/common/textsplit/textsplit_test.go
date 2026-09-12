package textsplit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestSplitCharacterCoverage 验证分段长度上限、连续区间、前进不变量和去除重叠后的完整还原。
func TestSplitCharacterCoverage(t *testing.T) {
	sections := make([]string, 0, 10)
	for index := range 10 {
		sections = append(sections, fmt.Sprintf("第 %d 节：唯一标题\n", index)+
			strings.Repeat(fmt.Sprintf("本节序号 %d。正文保留完整。", index), 30))
	}
	texts := []string{
		strings.Repeat("合同金额 1,234.50 元，不含税。\n    code(x)\n\n🙂备注：不得退款！", 120),
		strings.Repeat("连续中文", 3000),
		strings.Join(sections, "\n\n"),
	}
	parameters := []struct{ length, overlap int }{{256, 0}, {256, 30}, {256, 200}, {512, 200}, {2048, 50}}
	for _, parameter := range parameters {
		for textIndex, text := range texts {
			runes := []rune(text)
			segments := Split(text, parameter.length, parameter.overlap)
			if len(segments) == 0 {
				t.Fatalf("Split(text[%d], %d, %d) 未返回分段", textIndex, parameter.length, parameter.overlap)
			}
			covered := 0
			for position, segment := range segments {
				content := []rune(segment.Content)
				if len(content) > parameter.length {
					t.Fatalf("分段 %d 长度 %d 超过上限 %d", position+1, len(content), parameter.length)
				}
				if segment.CharacterCount != len(content) || segment.Position != position+1 {
					t.Fatalf("分段 %d 的字符数或位置不符：%d, %d", position+1, segment.CharacterCount, segment.Position)
				}
				// 非末段长度必须大于重叠，否则下一段的起点不会前进。
				if position+1 < len(segments) && len(content) <= parameter.overlap {
					t.Fatalf("分段 %d 长度 %d 未超过重叠 %d", position+1, len(content), parameter.overlap)
				}
				// 按已覆盖长度和重叠推算本段在原文中的起点。
				start := max(0, covered-parameter.overlap)
				if !slices.Equal(runes[start:start+len(content)], content) {
					t.Fatalf("分段 %d 不是原文 %d 处的连续区间（length=%d, overlap=%d）", position+1, start, parameter.length, parameter.overlap)
				}
				covered = start + len(content)
			}
			if covered != len(runes) {
				t.Fatalf("text[%d] 覆盖 %d 字符，原文 %d 字符（length=%d, overlap=%d）", textIndex, covered, len(runes), parameter.length, parameter.overlap)
			}
		}
	}
}

// TestSplitBoundaryAndHardCut 验证收缩到范围内最靠后的边界，范围内无边界字符时按长度上限硬切。
func TestSplitBoundaryAndHardCut(t *testing.T) {
	// 长度上限内有两处边界字符，收缩必须落在靠后的第 202 字符处。
	multiple := strings.Repeat("甲", 150) + "。" + strings.Repeat("乙", 50) + "。" + strings.Repeat("丙", 400)
	single := strings.Repeat("甲", 200) + "。" + strings.Repeat("乙", 400)
	for _, parameter := range []struct {
		name     string
		text     string
		overlap  int
		expected []int
	}{
		{"最靠后边界", multiple, 0, []int{202, 256, 144}},
		{"单一边界", single, 0, []int{201, 256, 144}},
		{"带重叠", single, 50, []int{201, 256, 244}},
	} {
		lengths := make([]int, 0, 3)
		for _, segment := range Split(parameter.text, 256, parameter.overlap) {
			lengths = append(lengths, segment.CharacterCount)
		}
		if !slices.Equal(lengths, parameter.expected) {
			t.Fatalf("%s：分段长度 = %v，期望 %v", parameter.name, lengths, parameter.expected)
		}
	}
}

// TestSplitNormalizesNewlines 验证换行规整为 \n 且不折叠代码缩进。
func TestSplitNormalizesNewlines(t *testing.T) {
	segments := Split("第一行\r\n    缩进保留\r第三行", 256, 0)
	if len(segments) != 1 || segments[0].Content != "第一行\n    缩进保留\n第三行" {
		t.Fatalf("Split() = %+v", segments)
	}
}

// TestSplitEmptyResult 验证空白正文和无法前进的长度组合都返回空结果。
func TestSplitEmptyResult(t *testing.T) {
	if segments := Split(" \n\t", 256, 50); segments != nil {
		t.Fatalf("空白正文 = %+v", segments)
	}
	for _, parameter := range []struct{ length, overlap int }{{0, 0}, {50, 50}, {50, 200}} {
		if segments := Split(strings.Repeat("正文", 500), parameter.length, parameter.overlap); segments != nil {
			t.Fatalf("Split(text, %d, %d) = %d 段", parameter.length, parameter.overlap, len(segments))
		}
	}
}
