package agentruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/integration/localskill"
)

const (
	// skillToolName 是加载技能说明的工具名称。
	skillToolName = "skill"
	// installSkillToolName 是安装技能的工具名称。
	installSkillToolName = "install_skill"
	// removeSkillToolName 是删除技能的工具名称。
	removeSkillToolName = "remove_skill"
	// maxSkillResources 是加载技能时列出的附带文件数量上限。
	maxSkillResources = 100
)

// skillToolNames 是技能工具，按注册顺序排列。
var skillToolNames = []string{skillToolName, installSkillToolName, removeSkillToolName}

const skillToolDesc = `加载一个技能的完整说明，按说明完成任务。
- skill 填写可用技能的名称，args 是可选的附加说明；本次运行中新安装的技能同样可以加载。
- 同一个技能在本次运行中加载过时不重复加载，直接按上文说明执行。

可用技能：
%s`

const installSkillToolDesc = `把技能安装到这台电脑，这台电脑上主人的所有助理共用；安装后当前运行即可用 skill 加载。
- source 可以是 GitHub 简写 owner/repo，可跟仓库内路径（如 anthropics/skills/skills/xlsx）；GitHub 仓库或目录地址；zip、tar.gz 压缩包或 SKILL.md 的地址；本机的绝对路径。
- 来源包含多个技能时用 skill 指定要安装的技能名称；同名技能会被替换。`

const removeSkillToolDesc = `删除助理安装在这台电脑上的技能。当前可删除：%s。其他 AI 工具安装的技能不能删除。`

// installSkillArgs 是安装技能的参数。
type installSkillArgs struct {
	Source string `json:"source" jsonschema:"required" jsonschema_description:"技能来源"`
	Skill  string `json:"skill,omitempty" jsonschema_description:"来源包含多个技能时要安装的技能名称"`
}

// removeSkillArgs 是删除技能的参数。
type removeSkillArgs struct {
	Name string `json:"name" jsonschema:"required" jsonschema_description:"技能名称"`
}

// skillBackend 把执行设备的技能目录提供给技能中间件，并按运行记录已加载的技能说明。
type skillBackend struct {
	skills  LocalSkills
	managed bool // 执行设备提供托管运行环境，技能说明后补充依赖安装方式。
	mu      sync.Mutex
	loaded map[string][32]byte // 技能名称到本次运行已加载说明的摘要。
}

// List 返回可用技能的名称与简介，fork 与模型切换不生效。
func (b *skillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	skills, err := b.skills.List(ctx)
	if err != nil {
		return nil, err
	}
	matters := make([]skill.FrontMatter, len(skills))
	for i, item := range skills {
		matters[i] = skill.FrontMatter{Name: item.Name, Description: item.Description}
	}
	return matters, nil
}

// Get 读取技能的名称、简介、正文与所在目录。
func (b *skillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	item, body, err := b.skills.Load(ctx, name)
	if err != nil {
		// 名称不存在时列出可用技能供模型改正。
		available := "无"
		if skills, listErr := b.skills.List(ctx); listErr == nil && len(skills) > 0 {
			names := make([]string, len(skills))
			for i, item := range skills {
				names[i] = item.Name
			}
			available = strings.Join(names, "、")
		}
		return skill.Skill{}, fmt.Errorf("没有名为 %s 的技能，可用技能：%s", name, available)
	}
	return skill.Skill{FrontMatter: skill.FrontMatter{Name: item.Name, Description: item.Description}, Content: body, BaseDirectory: item.Dir}, nil
}

// content 返回技能加载结果：正文、技能目录与附带文件清单以 skill_content 包裹；本次运行已加载过同一份说明时返回提示。
func (b *skillBackend) content(_ context.Context, loaded skill.Skill, rawArguments string) (string, error) {
	digest := sha256.Sum256([]byte(loaded.BaseDirectory + "\x00" + loaded.Content))
	b.mu.Lock()
	repeated := b.loaded[loaded.Name] == digest
	b.loaded[loaded.Name] = digest
	b.mu.Unlock()
	if repeated {
		return fmt.Sprintf("技能 %s 的说明已在上文加载，直接按其执行。", loaded.Name), nil
	}
	var text strings.Builder
	fmt.Fprintf(&text, "<skill_content name=%q>\n%s\n\n技能目录：%s\n技能中的相对路径以技能目录为起点，读取文件与运行脚本时使用绝对路径。\n", loaded.Name, loaded.Content, loaded.BaseDirectory)
	// 列出技能附带的文件，内容由模型按需读取。
	var resources []string
	complete := true
	_ = filepath.WalkDir(loaded.BaseDirectory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == loaded.BaseDirectory {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "__pycache__" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Name() == localskill.FileName && filepath.Dir(path) == loaded.BaseDirectory {
			return nil
		}
		if len(resources) == maxSkillResources {
			complete = false
			return filepath.SkipAll
		}
		relative, _ := filepath.Rel(loaded.BaseDirectory, path)
		resources = append(resources, filepath.ToSlash(relative))
		return nil
	})
	if len(resources) > 0 {
		text.WriteString("<skill_resources>\n")
		for _, resource := range resources {
			fmt.Fprintf(&text, "<file>%s</file>\n", resource)
		}
		if !complete {
			fmt.Fprintf(&text, "<!-- 只列出前 %d 个文件 -->\n", maxSkillResources)
		}
		text.WriteString("</skill_resources>\n")
	}
	if b.managed {
		text.WriteString("技能说明中安装 Python 依赖或运行脚本的方式按 execute 工具说明中的托管运行环境用法执行，例如用 uv run --with 包名 代替 pip install。\n")
	}
	text.WriteString("</skill_content>")
	var arguments struct {
		Args string `json:"args"`
	}
	if err := json.Unmarshal([]byte(rawArguments), &arguments); err == nil && strings.TrimSpace(arguments.Args) != "" {
		text.WriteString("\n\n附加说明：" + arguments.Args)
	}
	return text.String(), nil
}

// newSkillTools 按有效配置创建技能中间件与安装、删除工具，有效配置不含技能工具时返回空值。
func newSkillTools(ctx context.Context, request RunRequest) (adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage], []tool.BaseTool, error) {
	if !slices.Contains(request.Assignment.Tools, skillToolName) {
		return nil, nil, nil
	}
	if request.Skills == nil {
		return nil, nil, fmt.Errorf("agent run assignment requires skill tools without local skills")
	}
	backend := &skillBackend{skills: request.Skills, managed: request.ManagedToolchain, loaded: make(map[string][32]byte)}
	middleware, err := skill.NewTyped(ctx, &skill.TypedConfig[*schema.AgenticMessage]{
		Backend:    backend,
		UseChinese: true,
		// 技能用法由场景规则中的工具说明提供。
		CustomSystemPrompt: func(context.Context, string) string { return "" },
		// 可用技能随工具说明提供，每次运行开始时按技能目录刷新。
		CustomToolDescription: func(_ context.Context, skills []skill.FrontMatter) string {
			if len(skills) == 0 {
				return fmt.Sprintf(skillToolDesc, "（暂无，可用 install_skill 安装）")
			}
			lines := make([]string, len(skills))
			for i, item := range skills {
				lines[i] = "- " + item.Name + "：" + item.Description
			}
			return fmt.Sprintf(skillToolDesc, strings.Join(lines, "\n"))
		},
		CustomFormatReminder: func(context.Context, *skill.FormatReminderInput) (*skill.FormatReminderOutput, error) {
			return &skill.FormatReminderOutput{}, nil
		},
		// 技能名称不限定枚举，运行中新安装的技能同样可以加载，名称不存在时由工具返回可用技能。
		CustomToolParams: func(_ context.Context, defaults map[string]*schema.ParameterInfo) (map[string]*schema.ParameterInfo, error) {
			defaults["skill"].Desc = "技能名称"
			defaults["args"].Desc = "传给技能的附加说明"
			return defaults, nil
		},
		BuildContent: backend.content,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create skill middleware: %w", err)
	}
	install, err := utils.InferTool(installSkillToolName, installSkillToolDesc, func(ctx context.Context, input installSkillArgs) (string, error) {
		installed, err := request.Skills.Install(ctx, input.Source, input.Skill)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("已安装技能 %s：%s\n现在可以用 skill 加载它。", installed.Name, installed.Description), nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create install skill tool: %w", err)
	}
	skills, err := request.Skills.List(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list local skills: %w", err)
	}
	// 删除工具的说明列出助理安装的技能。
	var removable []string
	for _, item := range skills {
		if item.Source == localskill.SourceCervi {
			removable = append(removable, item.Name)
		}
	}
	current := "无"
	if len(removable) > 0 {
		current = strings.Join(removable, "、")
	}
	remove, err := utils.InferTool(removeSkillToolName, fmt.Sprintf(removeSkillToolDesc, current), func(ctx context.Context, input removeSkillArgs) (string, error) {
		removed, err := request.Skills.Remove(ctx, input.Name)
		if err != nil {
			return "", err
		}
		if !removed {
			return "", fmt.Errorf("没有可删除的名为 %s 的技能", input.Name)
		}
		return "已删除技能 " + input.Name + "。", nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create remove skill tool: %w", err)
	}
	return middleware, []tool.BaseTool{install, remove}, nil
}

// skillCallIDs 返回上下文中加载技能的工具调用编号，这些调用与结果在摘要压缩时原样保留。
func skillCallIDs(messages []*schema.AgenticMessage) map[string]bool {
	ids := make(map[string]bool)
	for _, message := range messages {
		for _, call := range toolCalls(message) {
			if call.Name == skillToolName {
				ids[call.CallID] = true
			}
		}
	}
	return ids
}
