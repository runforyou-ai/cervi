//go:build server

package agentruntime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type testInputFeed struct {
	mu       sync.Mutex
	desired  int64
	claimed  int64
	messages []Message
}

func (f *testInputFeed) Peek(_ context.Context, afterSeq int64) ([]Trigger, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]Trigger, 0)
	for seq := afterSeq + 1; seq <= f.desired; seq++ {
		result = append(result, Trigger{Seq: seq})
	}
	return result, nil
}

func (f *testInputFeed) Claim(_ context.Context, throughSeq int64) (ClaimedInput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimed = min(throughSeq, f.desired)
	return ClaimedInput{Messages: append([]Message(nil), f.messages[:f.claimed]...), EndSeq: f.claimed}, nil
}

func (f *testInputFeed) appendUser(content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.desired++
	f.messages = append(f.messages, Message{Role: MessageRoleUser, Content: content})
}

type steeringChatModel struct {
	mu               sync.Mutex
	calls            int
	calledWithoutNew bool
}

func (m *steeringChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls == 1 {
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID: "calculator-call-1", Type: "function",
			Function: schema.FunctionCall{Name: "blocking_calculator", Arguments: `{"left":1,"right":2}`},
		}}), nil
	}
	userMessages := 0
	for _, message := range input {
		if message.Role == schema.User {
			userMessages++
		}
	}
	if userMessages < 2 {
		m.calledWithoutNew = true
		return schema.AssistantMessage("stale response", nil), nil
	}
	return schema.AssistantMessage("response with follow-up", nil), nil
}

func (m *steeringChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *steeringChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type blockingCalculatorInput struct {
	Left  float64 `json:"left" jsonschema:"required"`
	Right float64 `json:"right" jsonschema:"required"`
}

type cancelRaceInputFeed struct {
	mu                  sync.Mutex
	blockingPeekStarted chan struct{}
	blocked             bool
}

func (f *cancelRaceInputFeed) Peek(ctx context.Context, afterSeq int64) ([]Trigger, error) {
	if afterSeq == 0 {
		return []Trigger{{Seq: 1}}, nil
	}
	f.mu.Lock()
	if f.blocked {
		f.mu.Unlock()
		return nil, nil
	}
	f.blocked = true
	close(f.blockingPeekStarted)
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *cancelRaceInputFeed) Claim(context.Context, int64) (ClaimedInput, error) {
	return ClaimedInput{Messages: []Message{{Role: MessageRoleUser, Content: "hello"}}, EndSeq: 1}, nil
}

type finalAfterWatcherModel struct {
	watcherStarted <-chan struct{}
}

func (m *finalAfterWatcherModel) Generate(ctx context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.watcherStarted:
		return schema.AssistantMessage("final response", nil), nil
	}
}

func (m *finalAfterWatcherModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

func (m *finalAfterWatcherModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestEinoRuntimeSteersBeforeNextModelCall 验证 Tool 完成后先吸收新输入再继续规划。
func TestEinoRuntimeSteersBeforeNextModelCall(t *testing.T) {
	toolStarted := make(chan struct{})
	releaseTool := make(chan struct{})
	blockingTool, err := toolutils.InferTool("blocking_calculator", "test calculator", func(ctx context.Context, input blockingCalculatorInput) (float64, error) {
		close(toolStarted)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-releaseTool:
			return input.Left + input.Right, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	chatModel := &steeringChatModel{}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) {
			return chatModel, nil
		},
		tools: []tool.BaseTool{blockingTool},
	}
	feed := &testInputFeed{}
	feed.appendUser("calculate one plus two")

	go func() {
		<-toolStarted
		feed.appendUser("also include four")
		close(releaseTool)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{Name: "test-agent", MaxTurns: 4}, feed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "response with follow-up" || result.EndSeq != 2 {
		t.Fatalf("result = %#v", result)
	}
	chatModel.mu.Lock()
	defer chatModel.mu.Unlock()
	if chatModel.calledWithoutNew {
		t.Fatal("model continued after tool call without the follow-up input")
	}
	if chatModel.calls != 2 {
		t.Fatalf("model calls = %d, want 2", chatModel.calls)
	}
}

// TestEinoRuntimeIgnoresWatcherCancellationAfterSuccess 验证成功停止不会被 watcher 的协作取消改写为失败。
func TestEinoRuntimeIgnoresWatcherCancellationAfterSuccess(t *testing.T) {
	feed := &cancelRaceInputFeed{blockingPeekStarted: make(chan struct{})}
	chatModel := &finalAfterWatcherModel{watcherStarted: feed.blockingPeekStarted}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) {
			return chatModel, nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{Name: "test-agent", MaxTurns: 2}, feed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "final response" || result.EndSeq != 1 {
		t.Fatalf("result = %#v", result)
	}
}

// TestCalculate 验证计算器 Tool 的四则运算与除零保护。
func TestCalculate(t *testing.T) {
	result, err := calculate(context.Background(), calculatorInput{Operation: "multiply", Left: 6, Right: 7})
	if err != nil || result.Result != 42 {
		t.Fatalf("calculate result = %#v, error = %v", result, err)
	}
	if _, err := calculate(context.Background(), calculatorInput{Operation: "divide", Left: 1, Right: 0}); err == nil {
		t.Fatal("divide by zero should fail")
	}
}

// TestCompatibleBaseURL 验证百炼地址使用 OpenAI 兼容入口。
func TestCompatibleBaseURL(t *testing.T) {
	got, err := compatibleBaseURL("alibaba", "https://dashscope.aliyuncs.com")
	if err != nil || got != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("base URL = %q, error = %v", got, err)
	}
}
