//go:build server

package gateway

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// newTestRunStream 创建带指定分片预算与待发文本上限的运行过程流。
func newTestRunStream(partBytes, pendingBytes int) *runStream {
	return newRunStream(New(nil, nil, "test", Options{RunSnapshotPartBytes: partBytes, RunPendingTextBytes: pendingBytes}), "run-1", func() {})
}

// textDelta 构造一条向指定块追加文本的增量。
func textDelta(base, sequence int64, blockID, text string) agentruntime.StreamDelta {
	return agentruntime.StreamDelta{RunID: "run-1", StreamID: "stream-1", Attempt: 1, BaseSequence: base, Sequence: sequence,
		Operations: []agentruntime.StreamOperation{{Kind: agentruntime.StreamOperationAppendBlockText, BlockID: blockID, Text: text}}}
}

// TestRunStreamMergesPendingDeltas 验证首尾相接的待发增量合并为一条，序号跨越全部增量。
func TestRunStreamMergesPendingDeltas(t *testing.T) {
	stream := newTestRunStream(1024, 1024)
	stream.publish(textDelta(1, 2, "block-1", "甲"))
	stream.publish(textDelta(2, 3, "block-1", "乙"))
	stream.publish(textDelta(3, 4, "block-1", "丙"))
	if stream.delta == nil || stream.delta.BaseSequence != 1 || stream.delta.Sequence != 4 {
		t.Fatalf("delta = %#v", stream.delta)
	}
	want := []agentruntime.StreamOperation{{Kind: agentruntime.StreamOperationAppendBlockText, BlockID: "block-1", Text: "甲乙丙"}}
	if !reflect.DeepEqual(stream.delta.Operations, want) {
		t.Fatalf("operations = %#v, want %#v", stream.delta.Operations, want)
	}
}

// TestRunStreamEndsOnDisjointDelta 验证增量不相接时结束事件流，由客户端重新取快照。
func TestRunStreamEndsOnDisjointDelta(t *testing.T) {
	stream := newTestRunStream(1024, 1024)
	stream.publish(textDelta(1, 2, "block-1", "甲"))
	stream.publish(textDelta(5, 6, "block-1", "乙"))
	if !stream.closing || stream.delta != nil {
		t.Fatalf("closing = %v, delta = %#v", stream.closing, stream.delta)
	}
}

// TestRunStreamEndsOnPendingOverflow 验证待发增量超出文本上限时按慢消费者结束事件流。
func TestRunStreamEndsOnPendingOverflow(t *testing.T) {
	stream := newTestRunStream(1024, 16)
	stream.publish(textDelta(1, 2, "block-1", strings.Repeat("a", 10)))
	if stream.closing {
		t.Fatal("待发增量未超出上限时不应结束事件流")
	}
	stream.publish(textDelta(2, 3, "block-1", strings.Repeat("b", 10)))
	if !stream.closing || stream.delta != nil {
		t.Fatalf("closing = %v, delta = %#v", stream.closing, stream.delta)
	}
}

// TestRunStreamPublishAfterEnd 验证事件流结束后不再接收增量。
func TestRunStreamPublishAfterEnd(t *testing.T) {
	stream := newTestRunStream(1024, 1024)
	stream.finish()
	stream.publish(textDelta(0, 1, "block-1", "甲"))
	if stream.delta != nil {
		t.Fatalf("delta = %#v", stream.delta)
	}
}

// TestSplitSnapshot 验证快照按文本预算拆分，候选正文只在首个分片，每个分片至少一个内容块。
func TestSplitSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	snapshot := agentruntime.StreamSnapshot{
		RunID: "run-1", StreamID: "stream-1", Attempt: 2, Sequence: 9, CandidateContent: "候选",
		Blocks: []agentruntime.StreamBlock{
			{ID: "block-1", Position: 1, Kind: domain.AgentRunBlockThinking, Text: strings.Repeat("a", 10)},
			{ID: "block-2", Position: 2, Kind: domain.AgentRunBlockThinking, Text: strings.Repeat("b", 10)},
			{ID: "block-3", Position: 3, Kind: domain.AgentRunBlockToolCall, ToolCall: &agentruntime.StreamToolCall{
				Name: "search_knowledge", Status: domain.AgentToolCallRunning, StartedAt: &startedAt,
			}},
		},
	}
	parts := splitSnapshot(snapshot, 12)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	for i, part := range parts {
		if part.Part != i || part.PartCount != 2 || part.RunID != "run-1" || part.StreamID != "stream-1" || part.Attempt != 2 || part.Sequence != 9 {
			t.Fatalf("part %d = %#v", i, part)
		}
	}
	if parts[0].CandidateContent != "候选" || parts[1].CandidateContent != "" {
		t.Fatalf("candidate = %q / %q", parts[0].CandidateContent, parts[1].CandidateContent)
	}
	if len(parts[0].Blocks) != 1 || parts[0].Blocks[0].ID != "block-1" {
		t.Fatalf("first part blocks = %#v", parts[0].Blocks)
	}
	if len(parts[1].Blocks) != 2 || parts[1].Blocks[1].ToolCall == nil || parts[1].Blocks[1].ToolCall.Name != "search_knowledge" {
		t.Fatalf("second part blocks = %#v", parts[1].Blocks)
	}
	// 工具调用只携带名称、状态和起止时间。
	if parts[1].Blocks[1].ToolCall.Status != domain.AgentToolCallRunning || !parts[1].Blocks[1].ToolCall.StartedAt.Equal(startedAt) {
		t.Fatalf("tool call = %#v", parts[1].Blocks[1].ToolCall)
	}
}

// TestSplitSnapshotEmpty 验证空快照仍拆分为一个分片。
func TestSplitSnapshotEmpty(t *testing.T) {
	parts := splitSnapshot(agentruntime.StreamSnapshot{RunID: "run-1", StreamID: "stream-1"}, 16)
	want := []protocol.RunStreamSnapshot{{RunID: "run-1", StreamID: "stream-1", PartCount: 1, Blocks: []protocol.RunStreamBlock{}}}
	if !reflect.DeepEqual(parts, want) {
		t.Fatalf("parts = %#v, want %#v", parts, want)
	}
}
