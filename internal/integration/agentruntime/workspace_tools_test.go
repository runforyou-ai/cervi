package agentruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestResolveAssignmentLocalTools 验证本机工具进入工具清单并在指令中说明，没有本机工具时不出现说明。
func TestResolveAssignmentLocalTools(t *testing.T) {
	facts := AssignmentFacts{OrganizationName: "测试企业", AgentName: "小码", Scene: SceneContext{Scene: SceneAgentChat}}
	assignment := ResolveAssignment(facts, Capabilities{LocalTools: []string{"ls", "read_file"}})
	if !slices.Contains(assignment.Tools, "ls") || !slices.Contains(assignment.Tools, "read_file") || !strings.Contains(assignment.Instruction, "- ls、read_file：查阅这台电脑上的文件") {
		t.Fatalf("assignment=%+v", assignment)
	}
	plain := ResolveAssignment(facts, Capabilities{})
	if slices.ContainsFunc(plain.Tools, IsLocalTool) || strings.Contains(plain.Instruction, "这台电脑上的文件") {
		t.Fatalf("assignment without workspace=%+v", plain)
	}
}

// imageWorkspace 是返回固定文本与图片的测试本机文件后端，按图片读取时图片文件返回图片内容。
type imageWorkspace struct {
	filesystem.Backend
}

// Read 以文本返回任意文件。
func (imageWorkspace) Read(context.Context, *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	return &filesystem.FileContent{Content: "text view"}, nil
}

// MultiModalRead 以图片返回任意文件。
func (imageWorkspace) MultiModalRead(context.Context, *filesystem.MultiModalReadRequest) (*filesystem.MultiFileContent, error) {
	return &filesystem.MultiFileContent{Parts: []filesystem.FileContentPart{{Type: filesystem.FileContentPartTypeImage, MIMEType: "image/png", Data: []byte("png")}}}, nil
}

// workspaceChatModel 第一次调用读取本机文件，拿到工具结果后给出回答；rejectImages 为 true 时拒绝携带工具图片的请求。
type workspaceChatModel struct {
	mu           sync.Mutex
	rejectImages bool
	tools        [][]string
	inputs       [][]*schema.AgenticMessage
}

// Generate 按输入中是否已有工具结果决定调用工具或回答。
func (m *workspaceChatModel) Generate(_ context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.AgenticMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for _, info := range model.GetCommonOptions(nil, opts...).Tools {
		names = append(names, info.Name)
	}
	m.tools = append(m.tools, names)
	m.inputs = append(m.inputs, input)
	if m.rejectImages && carriesMedia(input) {
		return nil, errors.New("tool result images are not supported")
	}
	if toolResult(input[len(input)-1]) == nil {
		return assistantReply("", &schema.FunctionToolCall{CallID: "read-1", Name: "read_file", Arguments: `{"file_path":"/logo.png"}`}), nil
	}
	return assistantReply("已读取"), nil
}

// Stream 以单个分片返回模型输出。
func (m *workspaceChatModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	return singleChunkStream(m.Generate(ctx, input, opts...))
}

// runWorkspace 以只含 ls 与 read_file 的有效配置执行一次运行。
func runWorkspace(t *testing.T, chatModel *workspaceChatModel, modalities []domain.AIModelInputModality) RunResult {
	t.Helper()
	runtime := &EinoRuntime{newModel: func(context.Context, ModelConfig) (model.AgenticModel, error) { return chatModel, nil }}
	feed := &testInputFeed{}
	feed.appendUser("看看 logo")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID:      "workspace-run",
		Assignment: Assignment{AgentName: "小码", Tools: []string{"ls", "read_file"}, Model: AssignmentModel{InputModalities: modalities}},
		Workspace:  imageWorkspace{Backend: filesystem.NewInMemoryBackend()},
	}, feed)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestWorkspaceToolsFollowAssignment 验证只注册有效配置列出的本机工具，模型不支持图片时按文本读取。
func TestWorkspaceToolsFollowAssignment(t *testing.T) {
	chatModel := &workspaceChatModel{}
	result := runWorkspace(t, chatModel, nil)
	if result.Content != "已读取" {
		t.Fatalf("result=%+v", result)
	}
	tools := chatModel.tools[0]
	if !slices.Contains(tools, "ls") || !slices.Contains(tools, "read_file") || slices.Contains(tools, "grep") || slices.Contains(tools, "glob") || slices.Contains(tools, "write_file") {
		t.Fatalf("registered tools=%v", tools)
	}
	if text := messageText(chatModel.inputs[1][len(chatModel.inputs[1])-1]); !strings.Contains(text, "text view") {
		t.Fatalf("tool result=%q", text)
	}
	if _, _, err := newWorkspaceMiddleware(context.Background(), RunRequest{Assignment: Assignment{Tools: []string{"ls"}}}, nil); err == nil {
		t.Fatal("workspace tools created without workspace")
	}
}

// TestWorkspaceImageRead 验证模型支持图片时本机图片随工具结果交给模型并以类型记入过程；模型拒绝后改按文本读取重新执行。
func TestWorkspaceImageRead(t *testing.T) {
	images := []domain.AIModelInputModality{domain.AIModelInputModalityText, domain.AIModelInputModalityImage}
	chatModel := &workspaceChatModel{}
	result := runWorkspace(t, chatModel, images)
	if !carriesMedia(chatModel.inputs[1]) {
		t.Fatal("image result was not passed to model")
	}
	recorded := ""
	for _, block := range result.Blocks {
		if call := block.Payload.ToolCall; call != nil && call.Result != nil {
			recorded = *call.Result
		}
	}
	if recorded != "[image image/png]" {
		t.Fatalf("recorded result=%q", recorded)
	}

	rejecting := &workspaceChatModel{rejectImages: true}
	if result := runWorkspace(t, rejecting, images); result.Content != "已读取" {
		t.Fatalf("result after rejection=%+v", result)
	}
	last := rejecting.inputs[len(rejecting.inputs)-1]
	if carriesMedia(last) || !strings.Contains(messageText(last[len(last)-1]), "text view") {
		t.Fatalf("retry input=%v", messageText(last[len(last)-1]))
	}
}
