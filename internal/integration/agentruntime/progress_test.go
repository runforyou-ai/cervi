//go:build server

package agentruntime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

type processChatModel struct {
	generate func(context.Context, []*schema.Message) (*schema.Message, error)
}

// TestRecorderDropsSkippedTools 验证安全点补入新输入后不保留尚未执行的工具。
func TestRecorderDropsSkippedTools(t *testing.T) {
	recorder := newProcessRecorder(RunRequest{RunID: "run"})
	message := schema.AssistantMessage("准备计算", []schema.ToolCall{
		{ID: "skipped-1", Function: schema.FunctionCall{Name: "calculator", Arguments: "{}"}},
		{ID: "skipped-2", Function: schema.FunctionCall{Name: "calculator", Arguments: "{}"}},
	})
	message.ReasoningContent = "先分析旧问题"
	state := &adk.ChatModelAgentState{Messages: []*schema.Message{message}}
	if _, _, err := recorder.AfterModelRewriteState(context.Background(), state, nil); err != nil {
		t.Fatal(err)
	}
	before := recorder.blocks()
	recorder.resetCandidate()
	state.Messages = append(state.Messages, schema.AssistantMessage("根据补充信息得出的结果", nil))
	if _, _, err := recorder.AfterModelRewriteState(context.Background(), state, nil); err != nil {
		t.Fatal(err)
	}
	after := recorder.blocks()
	if len(after) != 2 || after[0].ID != before[0].ID || after[1].ID != before[1].ID || len(recorder.toolPositions) != 0 {
		t.Fatalf("blocks after safe point = %#v", after)
	}
	if recorder.progress.CandidateContent != "根据补充信息得出的结果" {
		t.Fatalf("candidate content = %q", recorder.progress.CandidateContent)
	}
}

// Generate 返回当前测试步骤指定的模型输出。
func (m *processChatModel) Generate(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.generate(ctx, input)
}

// Stream 拒绝当前测试未使用的流式调用。
func (m *processChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected streaming call")
}

// WithTools 保留测试模型的执行步骤。
func (m *processChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// TestRunRecordsToolCorrection 验证工具乱序完成、参数修正和成功过程的完整顺序。
func TestRunRecordsToolCorrection(t *testing.T) {
	runtime, err := New()
	if err != nil {
		t.Fatal(err)
	}
	modelCalls := 0
	chatModel := &processChatModel{generate: func(_ context.Context, messages []*schema.Message) (*schema.Message, error) {
		modelCalls++
		switch modelCalls {
		case 1:
			message := schema.AssistantMessage("先计算两个结果", []schema.ToolCall{
				{ID: "slow", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2,"delayMilliseconds":100}`}},
				{ID: "invalid", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":`}},
			})
			message.ReasoningContent = "需要分两步计算"
			return message, nil
		case 2:
			foundError := false
			for _, message := range messages {
				if message.Role == schema.Tool && message.ToolCallID == "invalid" && strings.Contains(message.Content, `"error"`) {
					foundError = true
				}
			}
			if !foundError {
				return nil, errors.New("model did not receive tool error")
			}
			return schema.AssistantMessage("修正参数再试一次", []schema.ToolCall{
				{ID: "corrected", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"multiply","left":3,"right":4}`}},
			}), nil
		default:
			message := schema.AssistantMessage("最终结果为 12", nil)
			message.ReasoningContent = "两个步骤均已完成"
			return message, nil
		}
	}}
	runtime.newModel = func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }
	feed := &testInputFeed{}
	feed.appendUser("开始计算")
	var snapshots []Progress
	result, err := runtime.Run(context.Background(), RunRequest{RunID: "run", Name: "test", Attempt: 2, OnProgress: func(progress Progress) {
		snapshots = append(snapshots, progress)
	}}, feed)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "最终结果为 12" || modelCalls != 3 || len(result.Blocks) != 7 {
		t.Fatalf("result = %#v, model calls = %d", result, modelCalls)
	}
	wantKinds := []domain.AgentRunBlockKind{domain.AgentRunBlockThinking, domain.AgentRunBlockContent, domain.AgentRunBlockToolCall, domain.AgentRunBlockToolCall, domain.AgentRunBlockContent, domain.AgentRunBlockToolCall, domain.AgentRunBlockThinking}
	for i, block := range result.Blocks {
		if block.Position != int64(i+1) || block.Kind != wantKinds[i] || block.ID == "" || block.ModelCallID == "" {
			t.Fatalf("block %d = %#v", i, block)
		}
	}
	failed, corrected := result.Blocks[3].Payload.ToolCall, result.Blocks[5].Payload.ToolCall
	if failed.Status != domain.AgentToolCallFailed || failed.Arguments != `{"operation":` || failed.Error == nil || *failed.Error == "" || failed.Result != nil {
		t.Fatalf("failed call = %#v", failed)
	}
	if corrected.Status != domain.AgentToolCallSucceeded || corrected.Result == nil || !strings.Contains(*corrected.Result, "12") || corrected.Error != nil {
		t.Fatalf("corrected call = %#v", corrected)
	}
	outOfOrder := false
	for i, snapshot := range snapshots {
		if snapshot.Sequence != int64(i+1) || snapshot.StreamID == "" || snapshot.Attempt != 2 {
			t.Fatalf("snapshot = %#v", snapshot)
		}
		if len(snapshot.Blocks) >= 4 && snapshot.Blocks[2].Payload.ToolCall.Status != domain.AgentToolCallSucceeded && snapshot.Blocks[3].Payload.ToolCall.Status == domain.AgentToolCallFailed {
			outOfOrder = true
		}
	}
	if !outOfOrder || snapshots[len(snapshots)-1].CandidateContent != result.Content {
		t.Fatal("missing out-of-order tool snapshot or final candidate")
	}
	// 修改已发出的快照不能改变成功结果，失败调用的错误指针也必须隔离。
	*snapshots[len(snapshots)-1].Blocks[3].Payload.ToolCall.Error = "changed"
	if *failed.Error == "changed" {
		t.Fatal("snapshot modified final blocks")
	}
}

// TestRunCancellationDiscardsProcess 验证取消工具执行时不把取消作为可修正错误继续调用模型。
func TestRunCancellationDiscardsProcess(t *testing.T) {
	runtime, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	modelCalls := 0
	chatModel := &processChatModel{generate: func(context.Context, []*schema.Message) (*schema.Message, error) {
		modelCalls++
		return schema.AssistantMessage("准备计算", []schema.ToolCall{{ID: "slow", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2,"delayMilliseconds":1000}`}}}), nil
	}}
	runtime.newModel = func(context.Context, ModelConfig) (model.ToolCallingChatModel, error) { return chatModel, nil }
	feed := &testInputFeed{}
	feed.appendUser("计算")
	result, err := runtime.Run(ctx, RunRequest{Name: "test", OnProgress: func(progress Progress) {
		for _, block := range progress.Blocks {
			if block.Payload.ToolCall != nil && block.Payload.ToolCall.Status == domain.AgentToolCallRunning {
				cancel()
			}
		}
	}}, feed)
	if err == nil || modelCalls != 1 || len(result.Blocks) != 0 || result.Content != "" {
		t.Fatalf("cancelled result = %#v, model calls = %d, error = %v", result, modelCalls, err)
	}
}
