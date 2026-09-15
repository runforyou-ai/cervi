//go:build server

package agentrun

import (
	"errors"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// testRunStreamDelta 构造指定起止序号的单操作增量。
func testRunStreamDelta(streamID string, sequence int64, operation agentruntime.StreamOperation) agentruntime.StreamDelta {
	return agentruntime.StreamDelta{RunID: "run", StreamID: streamID, BaseSequence: sequence - 1, Sequence: sequence, Operations: []agentruntime.StreamOperation{operation}}
}

// TestRunStreamSubscription 验证订阅按序接收增量、漏收后重新订阅恢复全文、取消与结束后停止回调。
func TestRunStreamSubscription(t *testing.T) {
	stream := newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream", Attempt: 1})
	appendText := agentruntime.StreamOperation{Kind: agentruntime.StreamOperationAppendBlockText, BlockID: "thinking", Text: "1"}
	stream.publish(testRunStreamDelta("stream", 1, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationUpsertBlock, Block: &agentruntime.StreamBlock{ID: "thinking", Position: 1, Kind: domain.AgentRunBlockThinking, Text: "0"}}))
	var received []agentruntime.StreamDelta
	ended := 0
	snapshot, subscription, ok := stream.subscribe(func(delta agentruntime.StreamDelta) { received = append(received, delta) }, func() { ended++ })
	if !ok || snapshot.Sequence != 1 || snapshot.Blocks[0].Text != "0" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	stream.publish(testRunStreamDelta("stream", 2, appendText))
	stream.publish(testRunStreamDelta("stream", 3, appendText))
	if len(received) != 2 || received[0].Sequence != 2 || received[1].Sequence != 3 {
		t.Fatalf("received = %#v", received)
	}
	// 漏收一条增量后应用出现缺口，重新订阅从快照恢复全文。
	if _, err := snapshot.Apply(received[1]); !errors.Is(err, agentruntime.ErrStreamGap) {
		t.Fatalf("apply after missing delta error = %v", err)
	}
	resumed, later, ok := stream.subscribe(func(agentruntime.StreamDelta) {}, func() {})
	if !ok || resumed.Sequence != 3 || resumed.Blocks[0].Text != "011" {
		t.Fatalf("resumed snapshot = %#v", resumed)
	}
	resumed.Blocks[0].Text = "changed"
	if copied, _, _ := stream.subscribe(func(agentruntime.StreamDelta) {}, func() {}); copied.Blocks[0].Text != "011" {
		t.Fatalf("snapshot shared with subscriber: %#v", copied)
	}
	later.Close()
	subscription.Close()
	stream.publish(testRunStreamDelta("stream", 4, appendText))
	if len(received) != 2 {
		t.Fatalf("closed subscription received = %#v", received)
	}
	_, active, _ := stream.subscribe(func(agentruntime.StreamDelta) {}, func() { ended++ })
	stream.end()
	stream.end()
	if ended != 1 {
		t.Fatalf("end callbacks = %d", ended)
	}
	active.Close()
	if _, _, ok := stream.subscribe(func(agentruntime.StreamDelta) {}, func() {}); ok {
		t.Fatal("subscribed to ended stream")
	}
}

// TestRunStreamEndsOnInvalidDelta 验证增量无法应用时结束运行流并通知订阅方。
func TestRunStreamEndsOnInvalidDelta(t *testing.T) {
	stream := newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream"})
	delivered, ended := 0, 0
	stream.subscribe(func(agentruntime.StreamDelta) { delivered++ }, func() { ended++ })
	stream.publish(testRunStreamDelta("stream", 2, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationClearCandidate}))
	if delivered != 0 || ended != 1 {
		t.Fatalf("delivered = %d, ended = %d", delivered, ended)
	}
	if _, _, ok := stream.subscribe(func(agentruntime.StreamDelta) {}, func() {}); ok {
		t.Fatal("subscribed to ended stream")
	}
}

// TestSubscribeRunStreamAfterRetry 验证重试替换执行尝试后订阅新流，旧尝试迟到的增量进不了新流。
func TestSubscribeRunStreamAfterRetry(t *testing.T) {
	previous := &runningAgentRun{attempt: 1, streamID: "stream-1", stream: newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream-1", Attempt: 1})}
	action := &ExecuteAction{runningRuns: map[string]*runningAgentRun{"run": previous}}
	var staleDeltas, freshDeltas []agentruntime.StreamDelta
	staleEnded := false
	stale, _, ok := action.SubscribeRunStream("run", func(delta agentruntime.StreamDelta) { staleDeltas = append(staleDeltas, delta) }, func() { staleEnded = true })
	if !ok || stale.StreamID != "stream-1" {
		t.Fatalf("previous snapshot = %#v", stale)
	}
	action.runningRuns["run"] = &runningAgentRun{attempt: 2, streamID: "stream-2", stream: newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream-2", Attempt: 2})}
	fresh, _, ok := action.SubscribeRunStream("run", func(delta agentruntime.StreamDelta) { freshDeltas = append(freshDeltas, delta) }, func() {})
	if !ok || fresh.StreamID != "stream-2" || fresh.Attempt != 2 {
		t.Fatalf("current snapshot = %#v", fresh)
	}
	tail := testRunStreamDelta("stream-1", 1, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationAppendCandidate, Text: "旧尝试尾部"})
	previous.stream.publish(tail)
	previous.stream.end()
	if len(staleDeltas) != 1 || !staleEnded || len(freshDeltas) != 0 {
		t.Fatalf("stale deltas = %#v, stale ended = %t, fresh deltas = %#v", staleDeltas, staleEnded, freshDeltas)
	}
	if _, err := fresh.Apply(tail); !errors.Is(err, agentruntime.ErrStreamMismatch) || fresh.CandidateContent != "" {
		t.Fatalf("apply previous tail error = %v, snapshot = %#v", err, fresh)
	}
}
