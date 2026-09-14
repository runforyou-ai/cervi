package searchtext

import (
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-ego/gse"
)

var (
	knowledgeOnce      sync.Once
	knowledgeSegmenter gse.Segmenter
	knowledgeLoadError error
)

// knowledgeWord 表示分词得到的一个多字汉语词及其首字的字符序号。
type knowledgeWord struct {
	lexeme     string
	start, end int
}

// LoadKnowledgeDictionary 加载知识库分词使用的简体词典与停用词，进程内只执行一次。
func LoadKnowledgeDictionary() error {
	knowledgeOnce.Do(func() {
		started := time.Now()
		knowledgeSegmenter.SkipLog = true
		if knowledgeLoadError = knowledgeSegmenter.LoadDictEmbed("zh_s"); knowledgeLoadError != nil {
			return
		}
		if knowledgeLoadError = knowledgeSegmenter.LoadStopEmbed(); knowledgeLoadError != nil {
			return
		}
		slog.Info("知识库分词词典加载完成", "duration_ms", time.Since(started).Milliseconds())
	})
	return knowledgeLoadError
}

// knowledgeWords 切分原文中两个字以上的纯汉字词；三字以上的词同时给出词典中存在的子词，与搜索模式一致。
func knowledgeWords(text string) []knowledgeWord {
	if LoadKnowledgeDictionary() != nil {
		return nil
	}
	// 分词结果给出字节偏移，按字符序号对齐单字词元位置。
	runeIndex := make([]int, len(text)+1)
	count := 0
	for offset := range text {
		runeIndex[offset] = count
		count++
	}
	runeIndex[len(text)] = count
	var words []knowledgeWord
	for _, segment := range knowledgeSegmenter.Segment([]byte(text)) {
		word := text[segment.Start():segment.End()]
		if utf8.RuneCountInString(word) < 2 || strings.ContainsFunc(word, func(r rune) bool { return !unicode.Is(unicode.Han, r) }) {
			continue
		}
		start := runeIndex[segment.Start()]
		words = append(words, knowledgeWord{lexeme: word, start: start, end: runeIndex[segment.End()]})
		for _, sub := range knowledgeSegmenter.CutSearch(word, true) {
			if sub == word || utf8.RuneCountInString(sub) < 2 {
				continue
			}
			for offset := 0; ; {
				index := strings.Index(word[offset:], sub)
				if index < 0 {
					break
				}
				subStart := start + utf8.RuneCountInString(word[:offset+index])
				words = append(words, knowledgeWord{lexeme: sub, start: subStart, end: subStart + utf8.RuneCountInString(sub)})
				offset += index + len(sub)
			}
		}
	}
	return words
}

// KnowledgeVector 返回知识分段的 tsvector 字面量，包含单字、字母数字片段和搜索模式分词词元。
func KnowledgeVector(text string) string {
	positions := map[string][]int{}
	startPosition := map[int]int{}
	for position, item := range tokenize(text, false) {
		startPosition[item.start] = position + 1
		addPosition(positions, item.lexemes[0], position+1)
	}
	for _, word := range knowledgeWords(text) {
		if knowledgeSegmenter.IsStop(word.lexeme) {
			continue
		}
		if position, ok := startPosition[word.start]; ok {
			addPosition(positions, word.lexeme, position)
		}
	}
	return vectorLiteral(positions)
}

// KnowledgeQuery 把自然语言问句转成词元任一匹配的 tsquery 文本；去掉停用词后没有可检索内容时返回 false。
func KnowledgeQuery(input string) (string, bool) {
	slots := tokenize(input, false)
	words := knowledgeWords(input)
	covered := make([]bool, utf8.RuneCountInString(input))
	for _, word := range words {
		for index := word.start; index < word.end; index++ {
			covered[index] = true
		}
	}
	var terms []string
	seen := map[string]bool{}
	appendTerm := func(term string) {
		if !seen[term] {
			seen[term] = true
			terms = append(terms, term)
		}
	}
	// 连续的字母数字片段按相邻短语匹配，未被分词覆盖且不是停用词的单字按单字匹配。
	for index := 0; index < len(slots); {
		if !slots[index].word {
			if !covered[slots[index].start] && !knowledgeSegmenter.IsStop(slots[index].lexemes[0]) {
				appendTerm("'" + slots[index].lexemes[0] + "'")
			}
			index++
			continue
		}
		next := index
		for next < len(slots) && slots[next].word {
			next++
		}
		phrase := make([]string, 0, next-index)
		for _, item := range slots[index:next] {
			phrase = append(phrase, "'"+item.lexemes[0]+"'")
		}
		if len(phrase) == 1 {
			appendTerm(phrase[0])
		} else {
			appendTerm("(" + strings.Join(phrase, " <-> ") + ")")
		}
		index = next
	}
	for _, word := range words {
		if !knowledgeSegmenter.IsStop(word.lexeme) {
			appendTerm("'" + word.lexeme + "'")
		}
	}
	return strings.Join(terms, " | "), len(terms) > 0
}
