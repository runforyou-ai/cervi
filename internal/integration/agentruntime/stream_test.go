package agentruntime

import (
	"errors"
	"reflect"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestStreamSnapshotApply 验证增量按起止序号应用，重复被忽略，缺口、换流和无法应用的操作不修改快照。
func TestStreamSnapshotApply(t *testing.T) {
	snapshot := StreamSnapshot{RunID: "run", StreamID: "stream"}
	first := StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: 0, Sequence: 1, Operations: []StreamOperation{
		{Kind: StreamOperationUpsertBlock, Block: &StreamBlock{ID: "thinking", Position: 1, Kind: domain.AgentRunBlockThinking, Text: "先"}},
		{Kind: StreamOperationAppendBlockText, BlockID: "thinking", Text: "想"},
		{Kind: StreamOperationAppendCandidate, Text: "回答"},
	}}
	if applied, err := snapshot.Apply(first); !applied || err != nil {
		t.Fatalf("apply first delta: applied = %t, error = %v", applied, err)
	}
	if applied, err := snapshot.Apply(first); applied || err != nil {
		t.Fatalf("apply duplicate delta: applied = %t, error = %v", applied, err)
	}
	if _, err := snapshot.Apply(StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: 2, Sequence: 3}); !errors.Is(err, ErrStreamGap) {
		t.Fatalf("apply gap error = %v", err)
	}
	// 合并增量的起点早于快照序号时无法部分应用。
	if _, err := snapshot.Apply(StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: 0, Sequence: 2}); !errors.Is(err, ErrStreamGap) {
		t.Fatalf("apply overlapping delta error = %v", err)
	}
	if _, err := snapshot.Apply(StreamDelta{RunID: "run", StreamID: "retry", BaseSequence: 1, Sequence: 2}); !errors.Is(err, ErrStreamMismatch) {
		t.Fatalf("apply other stream error = %v", err)
	}
	invalid := StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: 1, Sequence: 2, Operations: []StreamOperation{
		{Kind: StreamOperationClearCandidate},
		{Kind: StreamOperationAppendBlockText, BlockID: "missing", Text: "尾部"},
	}}
	if _, err := snapshot.Apply(invalid); err == nil {
		t.Fatal("apply unknown block succeeded")
	}
	if snapshot.Sequence != 1 || snapshot.CandidateContent != "回答" || len(snapshot.Blocks) != 1 || snapshot.Blocks[0].Text != "先想" {
		t.Fatalf("snapshot after rejected deltas = %#v", snapshot)
	}
}

// testStreamOperations 返回覆盖重置、文本追加、候选正文和工具状态更新的操作序列。
func testStreamOperations() []StreamOperation {
	return []StreamOperation{
		{Kind: StreamOperationUpsertBlock, Block: &StreamBlock{ID: "stale", Position: 1, Kind: domain.AgentRunBlockThinking}},
		{Kind: StreamOperationReset},
		{Kind: StreamOperationUpsertBlock, Block: &StreamBlock{ID: "thinking", Position: 1, Kind: domain.AgentRunBlockThinking, Text: "先"}},
		{Kind: StreamOperationAppendBlockText, BlockID: "thinking", Text: "想"},
		{Kind: StreamOperationAppendBlockText, BlockID: "thinking", Text: "想"},
		{Kind: StreamOperationAppendCandidate, Text: "我"},
		{Kind: StreamOperationAppendCandidate, Text: "来"},
		{Kind: StreamOperationClearCandidate},
		{Kind: StreamOperationUpsertBlock, Block: &StreamBlock{ID: "tool", Position: 2, Kind: domain.AgentRunBlockToolCall, ToolCall: &StreamToolCall{CallID: "call", Name: "calculator", Status: domain.AgentToolCallQueued}}},
		{Kind: StreamOperationUpsertBlock, Block: &StreamBlock{ID: "tool", Position: 2, Kind: domain.AgentRunBlockToolCall, ToolCall: &StreamToolCall{CallID: "call", Name: "calculator", Status: domain.AgentToolCallRunning}}},
	}
}

// TestMergeStreamOperations 验证合并后的操作与逐条应用得到相同快照，重置之前的操作被丢弃。
func TestMergeStreamOperations(t *testing.T) {
	operations := testStreamOperations()
	stepwise := StreamSnapshot{RunID: "run", StreamID: "stream"}
	for i, operation := range operations {
		if _, err := stepwise.Apply(StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: int64(i), Sequence: int64(i + 1), Operations: []StreamOperation{operation}}); err != nil {
			t.Fatal(err)
		}
	}
	merged := mergeStreamOperations(operations)
	combined := StreamSnapshot{RunID: "run", StreamID: "stream"}
	if _, err := combined.Apply(StreamDelta{RunID: "run", StreamID: "stream", Sequence: 1, Operations: merged}); err != nil {
		t.Fatal(err)
	}
	if len(merged) != 5 || merged[0].Kind != StreamOperationReset || !reflect.DeepEqual(stepwise.Blocks, combined.Blocks) || stepwise.CandidateContent != combined.CandidateContent {
		t.Fatalf("merged = %#v, stepwise = %#v, combined = %#v", merged, stepwise, combined)
	}
	if operations[2].Block.Text != "先" {
		t.Fatalf("merge modified source block: %#v", operations[2].Block)
	}
}

// TestMergeStreamDeltas 验证首尾相接的增量合并后与逐条应用结果一致，不相接或不同流的增量不能合并。
func TestMergeStreamDeltas(t *testing.T) {
	operations := testStreamOperations()
	stepwise := StreamSnapshot{RunID: "run", StreamID: "stream"}
	var merged StreamDelta
	for i, operation := range operations {
		delta := StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: int64(i), Sequence: int64(i + 1), Operations: []StreamOperation{operation}}
		if _, err := stepwise.Apply(delta); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			merged = delta
			continue
		}
		var ok bool
		if merged, ok = MergeStreamDeltas(merged, delta); !ok {
			t.Fatalf("merge delta %d failed", i)
		}
	}
	combined := StreamSnapshot{RunID: "run", StreamID: "stream"}
	if _, err := combined.Apply(merged); err != nil {
		t.Fatal(err)
	}
	if merged.BaseSequence != 0 || merged.Sequence != int64(len(operations)) || len(merged.Operations) != 5 ||
		!reflect.DeepEqual(stepwise.Blocks, combined.Blocks) || stepwise.CandidateContent != combined.CandidateContent || combined.Sequence != stepwise.Sequence {
		t.Fatalf("merged = %#v, stepwise = %#v, combined = %#v", merged, stepwise, combined)
	}
	if _, ok := MergeStreamDeltas(StreamDelta{RunID: "run", StreamID: "stream", Sequence: 1}, StreamDelta{RunID: "run", StreamID: "stream", BaseSequence: 2, Sequence: 3}); ok {
		t.Fatal("merged non-adjacent deltas")
	}
	if _, ok := MergeStreamDeltas(StreamDelta{RunID: "run", StreamID: "stream", Sequence: 1}, StreamDelta{RunID: "run", StreamID: "retry", BaseSequence: 1, Sequence: 2}); ok {
		t.Fatal("merged deltas from different streams")
	}
}
