//go:build server

package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// toolHungryChatModel 只要还能看到普通工具就继续调用，工具被收敛后给出最终结果。
type toolHungryChatModel struct {
	mu               sync.Mutex
	preferGroupReply bool // 优先提交群内结果，用于验证提交后的收尾规划。
	lastTools        []string
	toolsByCall      [][]string
	lastUserText     string
}

// Generate 按本次可用工具决定继续调用还是收尾。
func (m *toolHungryChatModel) Generate(_ context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastTools = nil
	for _, info := range model.GetCommonOptions(&model.Options{}, opts...).Tools {
		m.lastTools = append(m.lastTools, info.Name)
	}
	m.toolsByCall = append(m.toolsByCall, m.lastTools)
	for _, message := range input {
		if message.Role == schema.User {
			m.lastUserText = message.Content
		}
	}
	submitReply := schema.AssistantMessage("", []schema.ToolCall{{
		ID: "group-reply-call", Type: "function",
		Function: schema.FunctionCall{Name: groupReplyToolName, Arguments: `{"outcome":"reply","body":"按已有资料回答"}`},
	}})
	if slices.Contains(m.lastTools, groupReplyToolName) && (m.preferGroupReply || !slices.Contains(m.lastTools, "calculator")) {
		return submitReply, nil
	}
	if slices.Contains(m.lastTools, "calculator") {
		return schema.AssistantMessage("继续算", []schema.ToolCall{{
			ID: "calculator-call", Type: "function",
			Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":1}`},
		}}), nil
	}
	return schema.AssistantMessage("按已有资料回答", nil), nil
}

func (m *toolHungryChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *toolHungryChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// runToolHungryAgent 以指定迭代上限执行一次持续调用工具的运行。
func runToolHungryAgent(t *testing.T, chatModel *toolHungryChatModel, request RunRequest) RunResult {
	t.Helper()
	calculator, err := newCalculatorTool()
	if err != nil {
		t.Fatal(err)
	}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil },
		tools:    []tool.BaseTool{calculator},
	}
	feed := &testInputFeed{}
	feed.appendUser("帮我查一下")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, request, feed)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestFinalIterationAnswersWithoutTools 验证到达迭代上限时移除工具，本轮以最终回答收尾。
func TestFinalIterationAnswersWithoutTools(t *testing.T) {
	chatModel := &toolHungryChatModel{}
	result := runToolHungryAgent(t, chatModel, RunRequest{RunID: "budget-run", Name: "test-agent", MaxIterations: 3, MaxTurns: 2})
	if result.Content != "按已有资料回答" {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if len(chatModel.toolsByCall) != 3 || len(chatModel.toolsByCall[1]) != 1 || len(chatModel.toolsByCall[2]) != 0 {
		t.Fatalf("tools by iteration = %v", chatModel.toolsByCall)
	}
	if chatModel.lastUserText != "工具调用次数已达本轮上限，请基于已获得的信息给出最终回答。" {
		t.Fatalf("final iteration hint = %q", chatModel.lastUserText)
	}
}

// TestFinalIterationKeepsGroupReplyTool 验证群内运行在预算用尽时只保留结束工具，提交后按已提交结果收尾。
func TestFinalIterationKeepsGroupReplyTool(t *testing.T) {
	chatModel := &toolHungryChatModel{}
	result := runToolHungryAgent(t, chatModel, RunRequest{
		RunID: "budget-group-run", Name: "test-agent", MaxIterations: 2, MaxTurns: 2, GroupReply: &GroupReplyConfig{},
	})
	if result.Outcome != RunOutcomeReply || result.Content != "按已有资料回答" {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if len(chatModel.toolsByCall) != 2 || len(chatModel.toolsByCall[1]) != 1 ||
		chatModel.toolsByCall[1][0] != groupReplyToolName {
		t.Fatalf("tools by iteration = %v", chatModel.toolsByCall)
	}
}

// TestFinalIterationStopsAfterGroupSubmission 验证群内结果提交后本轮直接结束，不再规划。
func TestFinalIterationStopsAfterGroupSubmission(t *testing.T) {
	chatModel := &toolHungryChatModel{preferGroupReply: true}
	result := runToolHungryAgent(t, chatModel, RunRequest{
		RunID: "submitted-group-run", Name: "test-agent", MaxIterations: 5, MaxTurns: 2, GroupReply: &GroupReplyConfig{},
	})
	if result.Outcome != RunOutcomeReply || result.Content != "按已有资料回答" {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if len(chatModel.toolsByCall) != 1 {
		t.Fatalf("tools by iteration = %v", chatModel.toolsByCall)
	}
}

type invalidThenValidChatModel struct {
	mu    sync.Mutex
	calls int
	retry string
}

// Generate 先提交不合法的点名，再按工具返回的原因重新提交。
func (m *invalidThenValidChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	arguments := `{"outcome":"reply","body":"按已有资料回答"}`
	if m.calls == 1 {
		arguments = `{"outcome":"reply","body":"按已有资料回答","mentions":["查无此人"]}`
	} else {
		m.retry = input[len(input)-1].Content
	}
	return schema.AssistantMessage("", []schema.ToolCall{{
		ID: fmt.Sprintf("group-reply-call-%d", m.calls), Type: "function",
		Function: schema.FunctionCall{Name: groupReplyToolName, Arguments: arguments},
	}}), nil
}

func (m *invalidThenValidChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *invalidThenValidChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestGroupReplyRetriesInvalidSubmission 验证提交校验不通过时本轮继续，模型按返回的原因重新提交。
func TestGroupReplyRetriesInvalidSubmission(t *testing.T) {
	chatModel := &invalidThenValidChatModel{}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil },
	}
	feed := &testInputFeed{}
	feed.appendUser("帮我看下")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{
		RunID: "invalid-submission-run", Name: "test-agent", MaxIterations: 5, MaxTurns: 2,
		GroupReply: &GroupReplyConfig{MentionCandidates: []string{"产品经理"}},
	}, feed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != RunOutcomeReply || result.Content != "按已有资料回答" || len(result.Mentions) != 0 {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if chatModel.calls != 2 {
		t.Fatalf("model calls = %d, want 2", chatModel.calls)
	}
	if !strings.Contains(chatModel.retry, "不在可点名范围内") {
		t.Fatalf("retry hint = %q", chatModel.retry)
	}
}
