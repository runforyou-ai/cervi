package localworkspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
)

// pngHeader 是最小 PNG 文件头，按内容识别为 image/png。
var pngHeader = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R'}

// newTestWorkspace 创建包含若干文件的默认文件夹与其外的目录，返回后端、默认文件夹与外部目录。
func newTestWorkspace(t *testing.T) (*Backend, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	files := map[string]string{
		"workspace/README.md":        "# Cervi\n介绍\n",
		"workspace/src/main.go":      "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
		"workspace/src/util/text.go": "package util\n\n// Hello 返回问候。\nfunc Hello() string { return \"Hello\" }\n",
		"workspace/web/app.ts":       "export const hello = 'hello'\n",
		"workspace/.git/config":      "hello from git\n",
		"outside/notes.txt":          "outside hello\n",
	}
	for name, content := range files {
		path := filepath.Join(base, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "logo.png"), pngHeader, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.bin"), []byte{'a', 0, 'b'}, 0o644); err != nil {
		t.Fatal(err)
	}
	return New(root), root, outside
}

// requireSymlink 创建符号链接，当前平台不支持时跳过测试。
func requireSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink unavailable: %v", err)
		}
		t.Fatal(err)
	}
}

// TestResolve 验证相对路径以默认文件夹为起点，绝对路径与 ~ 开头的路径访问本机任意位置，符号链接按目标读取，以目录链接为起点时 glob 与 grep 遍历其目标。
func TestResolve(t *testing.T) {
	ctx := context.Background()
	backend, root, outside := newTestWorkspace(t)
	if content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "README.md"}); err != nil || content.Content != "# Cervi\n介绍" {
		t.Fatalf("relative read=%+v %v", content, err)
	}
	if content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: filepath.Join(outside, "notes.txt")}); err != nil || content.Content != "outside hello" {
		t.Fatalf("absolute read=%+v %v", content, err)
	}
	if content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "../outside/notes.txt"}); err != nil || content.Content != "outside hello" {
		t.Fatalf("parent read=%+v %v", content, err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := backend.resolve("~/Documents"); err != nil || got != filepath.Join(home, "Documents") {
		t.Fatalf("home path=%q %v", got, err)
	}
	requireSymlink(t, outside, filepath.Join(root, "linked"))
	if content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "linked/notes.txt"}); err != nil || content.Content != "outside hello" {
		t.Fatalf("symlink read=%+v %v", content, err)
	}
	if infos, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: "*.txt", Path: "linked"}); err != nil || len(infos) != 1 || infos[0].Path != filepath.Join(root, "linked", "notes.txt") {
		t.Fatalf("glob in linked dir=%+v %v", infos, err)
	}
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "outside", Path: "linked"}); err != nil || len(matches) != 1 || matches[0].Path != filepath.Join(root, "linked", "notes.txt") {
		t.Fatalf("grep in linked dir=%+v %v", matches, err)
	}
	if _, err := New(filepath.Join(t.TempDir(), "missing")).LsInfo(ctx, &filesystem.LsInfoRequest{}); err == nil {
		t.Fatal("listed missing default folder")
	}
}

// TestReadAndList 验证目录列表、按行读取、二进制文件、图片读取与写入拒绝。
func TestReadAndList(t *testing.T) {
	ctx := context.Background()
	backend, root, _ := newTestWorkspace(t)
	infos, err := backend.LsInfo(ctx, &filesystem.LsInfoRequest{Path: ""})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, info := range infos {
		rel, _ := filepath.Rel(root, info.Path)
		if info.IsDir {
			rel += "/"
		}
		paths = append(paths, filepath.ToSlash(rel))
	}
	if strings.Join(paths, ",") != ".git/,README.md,app.bin,logo.png,src/,web/" {
		t.Fatalf("ls=%v", paths)
	}
	content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "src/main.go", Offset: 3, Limit: 2})
	if err != nil || content.Content != "func main() {\n\tprintln(\"hello\")" {
		t.Fatalf("read range=%+v %v", content, err)
	}
	if content, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "src/main.go", Offset: 100}); err != nil || content.Content != "" {
		t.Fatalf("read past end=%+v %v", content, err)
	}
	if _, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "app.bin"}); err == nil {
		t.Fatal("read binary file as text")
	}
	if _, err := backend.Read(ctx, &filesystem.ReadRequest{FilePath: "src"}); err == nil {
		t.Fatal("read directory as file")
	}
	image, err := backend.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{ReadRequest: filesystem.ReadRequest{FilePath: "logo.png"}})
	if err != nil || len(image.Parts) != 1 || image.Parts[0].MIMEType != "image/png" {
		t.Fatalf("read image=%+v %v", image, err)
	}
	text, err := backend.MultiModalRead(ctx, &filesystem.MultiModalReadRequest{ReadRequest: filesystem.ReadRequest{FilePath: "README.md"}})
	if err != nil || text.FileContent == nil || !strings.HasPrefix(text.Content, "# Cervi") {
		t.Fatalf("multimodal read text=%+v %v", text, err)
	}
	if err := backend.Write(ctx, &filesystem.WriteRequest{FilePath: "new.txt", Content: "x"}); err == nil {
		t.Fatal("write accepted")
	}
	if err := backend.Edit(ctx, &filesystem.EditRequest{FilePath: "README.md", OldString: "Cervi", NewString: "x"}); err == nil {
		t.Fatal("edit accepted")
	}
}

// TestSearch 验证 glob 与 grep 的匹配、过滤、上下文与跳过版本库目录。
func TestSearch(t *testing.T) {
	ctx := context.Background()
	backend, root, _ := newTestWorkspace(t)
	infos, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: "**/*.go"})
	if err != nil || len(infos) != 2 {
		t.Fatalf("glob=%+v %v", infos, err)
	}
	mainGo, appTS := filepath.Join(root, "src", "main.go"), filepath.Join(root, "web", "app.ts")
	if infos, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: "*.go", Path: "src"}); err != nil || len(infos) != 1 || infos[0].Path != mainGo {
		t.Fatalf("glob in dir=%+v %v", infos, err)
	}
	if infos, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: filepath.Join(root, "web", "*.ts"), Path: "src"}); err != nil || len(infos) != 1 || infos[0].Path != appTS {
		t.Fatalf("absolute glob=%+v %v", infos, err)
	}
	if _, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Pattern: "["}); err == nil {
		t.Fatal("invalid glob accepted")
	}

	matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "hello", CaseInsensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, match := range matches {
		files = append(files, match.Path)
	}
	joined := strings.Join(files, ",")
	if strings.Contains(joined, ".git") || !strings.Contains(joined, filepath.Join(root, "src", "util", "text.go")) || !strings.Contains(joined, appTS) {
		t.Fatalf("grep files=%v", files)
	}
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "hello", Glob: "*.go"}); err != nil || len(matches) != 1 || matches[0].Path != mainGo || matches[0].Line != 4 {
		t.Fatalf("grep glob=%+v %v", matches, err)
	}
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "hello", FileType: "ts"}); err != nil || len(matches) != 1 || matches[0].Path != appTS {
		t.Fatalf("grep type=%+v %v", matches, err)
	}
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "println", Path: "src/main.go", BeforeLines: 1, AfterLines: 1}); err != nil || len(matches) != 3 || matches[0].Line != 3 || matches[2].Line != 5 {
		t.Fatalf("grep context=%+v %v", matches, err)
	}
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: `main\(\) \{.*println`, EnableMultiline: true}); err != nil || len(matches) != 2 || matches[0].Line != 3 || matches[1].Line != 4 {
		t.Fatalf("grep multiline=%+v %v", matches, err)
	}
	// 搜索单个文件时过滤条件按文件名匹配。
	if matches, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "println", Path: "src/main.go", Glob: "*.go", FileType: "go"}); err != nil || len(matches) != 1 {
		t.Fatalf("grep single file with filters=%+v %v", matches, err)
	}
	if _, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Pattern: "(?<=a)b"}); err == nil {
		t.Fatal("unsupported regexp accepted")
	}
}
