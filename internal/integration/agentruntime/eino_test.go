//go:build server

package agentruntime

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
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
	firstCall        chan struct{}
}

func (m *steeringChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.calls == 1 {
		close(m.firstCall)
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID: "calculator-call-1", Type: "function",
			Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2,"delayMilliseconds":500}`},
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
	var logOutput bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	calculator, err := newCalculatorTool()
	if err != nil {
		t.Fatal(err)
	}
	chatModel := &steeringChatModel{firstCall: make(chan struct{})}
	runtime := &EinoRuntime{
		newModel: func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) {
			return chatModel, nil
		},
		tools: []tool.BaseTool{calculator},
	}
	feed := &testInputFeed{}
	feed.appendUser("calculate one plus two")

	go func() {
		<-chatModel.firstCall
		time.Sleep(50 * time.Millisecond)
		feed.appendUser("also include four")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := runtime.Run(ctx, RunRequest{RunID: "test-run-id", Name: "test-agent", MaxTurns: 4}, feed)
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
	logs := logOutput.String()
	for _, expected := range []string{`"msg":"Agent Tool 调用开始"`, `"msg":"Calculator Tool 执行配置"`, `"msg":"Agent Tool 调用成功"`, `"agent_run_id":"test-run-id"`, `"tool_name":"calculator"`, `"tool_call_id":"calculator-call-1"`, `"operation":"add"`, `"delay_ms":500`, `"duration_ms":`} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("tool logs do not contain %q: %s", expected, logs)
		}
	}
	if strings.Contains(logs, "delayMilliseconds") || strings.Contains(logs, `"left"`) || strings.Contains(logs, `"right"`) || strings.Contains(logs, `"arguments"`) {
		t.Fatalf("tool logs contain call arguments: %s", logs)
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
	startedAt := time.Now()
	if _, err := calculate(context.Background(), calculatorInput{Operation: "add", Left: 1, Right: 2, DelayMilliseconds: 30}); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(startedAt); elapsed < 20*time.Millisecond {
		t.Fatalf("calculator delay elapsed = %s", elapsed)
	}
	if _, err := calculate(context.Background(), calculatorInput{Operation: "add", DelayMilliseconds: 30001}); err == nil {
		t.Fatal("calculator delay above the limit should fail")
	}
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := calculate(cancelledContext, calculatorInput{Operation: "add", DelayMilliseconds: 1000}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled calculator error = %v", err)
	}
}

// TestCompatibleBaseURL 验证百炼地址使用 OpenAI 兼容入口。
func TestCompatibleBaseURL(t *testing.T) {
	got, err := compatibleBaseURL("alibaba", "https://dashscope.aliyuncs.com")
	if err != nil || got != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("base URL = %q, error = %v", got, err)
	}
}
