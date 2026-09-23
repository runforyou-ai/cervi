package agentruntime

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	fsmiddleware "github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const lsToolDesc = `列出工作区中目录的直接子项，目录以 / 结尾。
- path 使用以 / 开头的工作区路径，/ 表示工作区根目录。`

const readFileToolDesc = `读取工作区中的文本文件，结果以 cat -n 格式带行号返回，行号从 1 开始，默认最多读取 2000 行。
- file_path 使用以 / 开头的工作区路径。
- 已知需要的范围时用 offset 与 limit 只读取该部分。
- 二进制文件与超过 10 MB 的文件无法读取。`

const readImageToolDesc = `
- PNG、JPEG、GIF、WebP 图片以图片形式返回。`

const globToolDesc = `按 glob 模式查找工作区中的文件，返回按修改时间从新到旧排列的工作区路径。
- 支持 *、**、?、[abc] 与 {a,b}，如 "**/*.go"、"src/**/*.ts"。
- path 是搜索起点目录，省略时从工作区根目录开始；模式相对起点目录匹配，以 / 开头的模式相对工作区根目录匹配。
- 跳过 .git 目录。`

const grepToolDesc = `在工作区文件内容中按正则表达式搜索。
- 正则使用 Go RE2 语法，不支持反向引用和环视；字面的特殊字符需要转义，如 interface\{\}。
- path 是搜索的文件或目录，省略时搜索整个工作区。
- glob 按路径过滤文件，不含 / 的模式匹配文件名，如 "*.go"、"*.{ts,tsx}"；type 按扩展名过滤，如 "go"、"py"。
- output_mode：files_with_matches 只列出文件（默认），content 列出匹配行并可用 -A、-B、-C 显示上下文，count 列出每个文件的匹配数。
- multiline 为 true 时模式可以跨行匹配，. 匹配换行。
- 跳过 .git 目录、二进制文件和超过 2 MB 的文件。`

// newWorkspaceMiddleware 按有效配置中的本机工具创建工作区工具中间件；模型支持图片输入且工作区可读取图片时 read_file 以图片返回图片文件，images 为 false 时改按文本读取。
func newWorkspaceMiddleware(ctx context.Context, request RunRequest, images *atomic.Bool) (adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage], []string, error) {
	names := make([]string, 0, len(localTools))
	for _, name := range request.Assignment.Tools {
		if IsLocalTool(name) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, nil, nil
	}
	if request.Workspace == nil {
		return nil, nil, fmt.Errorf("agent run assignment requires workspace tools %v without a workspace", names)
	}
	// 按有效配置启用工具，未列出的工具不注册。
	config := func(name, desc string) *fsmiddleware.ToolConfig {
		return &fsmiddleware.ToolConfig{Name: name, Desc: &desc, Disable: !slices.Contains(names, name)}
	}
	readDesc := readFileToolDesc
	backend := request.Workspace
	_, multimodal := backend.(filesystem.MultiModalReader)
	multimodal = multimodal && slices.Contains(request.Assignment.Model.InputModalities, domain.AIModelInputModalityImage)
	if multimodal {
		readDesc += readImageToolDesc
		backend = &imageGatedBackend{Backend: backend, images: images}
	}
	middleware, err := fsmiddleware.NewTyped[*schema.AgenticMessage](ctx, &fsmiddleware.MiddlewareConfig{
		Backend:             backend,
		UseMultiModalRead:   multimodal,
		LsToolConfig:        config("ls", lsToolDesc),
		ReadFileToolConfig:  config("read_file", readDesc),
		GlobToolConfig:      config("glob", globToolDesc),
		GrepToolConfig:      config("grep", grepToolDesc),
		WriteFileToolConfig: &fsmiddleware.ToolConfig{Disable: true},
		EditFileToolConfig:  &fsmiddleware.ToolConfig{Disable: true},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create workspace tools middleware: %w", err)
	}
	return middleware, names, nil
}

// imageGatedBackend 在图片读取被关闭后把全部文件按文本读取，图片文件因此按二进制文件拒绝读取。
type imageGatedBackend struct {
	filesystem.Backend
	images *atomic.Bool
}

// MultiModalRead 图片读取开启时交给工作区按图片读取，关闭后按文本读取。
func (b *imageGatedBackend) MultiModalRead(ctx context.Context, req *filesystem.MultiModalReadRequest) (*filesystem.MultiFileContent, error) {
	if b.images.Load() {
		return b.Backend.(filesystem.MultiModalReader).MultiModalRead(ctx, req)
	}
	content, err := b.Backend.Read(ctx, &req.ReadRequest)
	if err != nil {
		return nil, err
	}
	return &filesystem.MultiFileContent{FileContent: content}, nil
}
