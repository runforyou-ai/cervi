package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestKeyConstantsMatchLocaleFiles 验证包内全部 Key 常量与各语言词条键集合双向一致：常量必须都有词条，词条不得有多余键；由此同时保证各语言文件包含相同的文案键。
func TestKeyConstantsMatchLocaleFiles(t *testing.T) {
	constants := collectKeyConstants(t)
	if len(constants) == 0 {
		t.Fatal("未在包内源文件中找到任何 Key 常量")
	}

	for _, path := range []string{"locales/en-US.json", "locales/zh-CN.json"} {
		localeKeys := readLocaleKeys(t, path)

		var missing []string
		for key := range constants {
			if _, ok := localeKeys[key]; !ok {
				missing = append(missing, key)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("%s 缺少 Key 常量对应的词条: %v", path, missing)
		}

		var extra []string
		for key := range localeKeys {
			if _, ok := constants[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Errorf("%s 存在没有 Key 常量对应的多余词条: %v", path, extra)
		}
	}
}

// TestLocalizeMatchesRequestedLanguage 验证本地化器匹配请求语言并回退到英文。
func TestLocalizeMatchesRequestedLanguage(t *testing.T) {
	tests := []struct {
		acceptLanguage string
		wantMessage    string
		wantLanguage   string
	}{
		{acceptLanguage: "zh-CN", wantMessage: "请先登录。", wantLanguage: "zh-CN"},
		{acceptLanguage: "en-US", wantMessage: "Please log in first.", wantLanguage: "en-US"},
		{acceptLanguage: "fr-FR", wantMessage: "Please log in first.", wantLanguage: "en-US"},
	}

	for _, test := range tests {
		message, matchedLanguage := Localize(test.acceptLanguage, ErrorAuthenticationRequired)
		if message != test.wantMessage || matchedLanguage != test.wantLanguage {
			t.Fatalf(
				"Localize(%q) = (%q, %q), want (%q, %q)",
				test.acceptLanguage,
				message,
				matchedLanguage,
				test.wantMessage,
				test.wantLanguage,
			)
		}
	}
}

// collectKeyConstants 解析包目录内所有非测试源文件，收集类型为 Key 的常量的字符串字面量值。
func collectKeyConstants(t *testing.T) map[string]struct{} {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	fileSet := token.NewFileSet()
	keys := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.CONST {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				ident, ok := valueSpec.Type.(*ast.Ident)
				if !ok || ident.Name != "Key" {
					continue
				}
				for _, value := range valueSpec.Values {
					lit, ok := value.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					key, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					keys[key] = struct{}{}
				}
			}
		}
	}
	return keys
}

// readLocaleKeys 读取指定语言文件并返回其中的文案键集合。
func readLocaleKeys(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	content, err := localeFiles.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	messages := make(map[string]json.RawMessage)
	if err := json.Unmarshal(content, &messages); err != nil {
		t.Fatal(err)
	}
	keys := make(map[string]struct{}, len(messages))
	for key := range messages {
		keys[key] = struct{}{}
	}
	return keys
}
