//go:build server

package agentrun

import (
	"errors"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// TestRunStreamSubscription 验证积压超限的订阅被关闭、重新订阅取得完整快照、执行尝试退出后结束订阅。
func TestRunStreamSubscription(t *testing.T) {
	stream := newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream", Attempt: 1})
	delta := func(sequence int64, operation agentruntime.StreamOperation) agentruntime.StreamDelta {
		return agentruntime.StreamDelta{RunID: "run", StreamID: "stream", Attempt: 1, Sequence: sequence, Operations: []agentruntime.StreamOperation{operation}}
	}
	stream.publish(delta(1, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationUpsertBlock, Block: &agentruntime.StreamBlock{ID: "thinking", Position: 1, Kind: domain.AgentRunBlockThinking, Text: "0"}}))
	slow, ok := stream.subscribe()
	if !ok || slow.Snapshot.Sequence != 1 || slow.Snapshot.Blocks[0].Text != "0" {
		t.Fatalf("slow subscription = %#v", slow)
	}
	want := "0"
	for sequence := int64(2); sequence <= runStreamSubscriberBuffer+2; sequence++ {
		stream.publish(delta(sequence, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationAppendBlockText, BlockID: "thinking", Text: "1"}))
		want += "1"
	}
	received := 0
	for range slow.Deltas() {
		received++
	}
	if received != runStreamSubscriberBuffer || slow.Reason() != RunStreamOverflow {
		t.Fatalf("slow subscriber received %d deltas, reason = %q", received, slow.Reason())
	}
	// 漏掉增量的订阅方重新订阅后从快照恢复全文。
	resumed, ok := stream.subscribe()
	if !ok || resumed.Snapshot.Sequence != runStreamSubscriberBuffer+2 || resumed.Snapshot.Blocks[0].Text != want {
		t.Fatalf("resumed snapshot = %#v", resumed.Snapshot)
	}
	resumed.Snapshot.Blocks[0].Text = "changed"
	stream.publish(delta(runStreamSubscriberBuffer+3, agentruntime.StreamOperation{Kind: agentruntime.StreamOperationAppendCandidate, Text: "回答"}))
	if next := <-resumed.Deltas(); next.Sequence != runStreamSubscriberBuffer+3 {
		t.Fatalf("resumed delta = %#v", next)
	}
	cancelled, _ := stream.subscribe()
	if cancelled.Snapshot.Blocks[0].Text != want || cancelled.Snapshot.CandidateContent != "回答" {
		t.Fatalf("snapshot shared with subscriber: %#v", cancelled.Snapshot)
	}
	cancelled.Close()
	if _, open := <-cancelled.Deltas(); open || cancelled.Reason() != "" {
		t.Fatalf("closed subscription reason = %q", cancelled.Reason())
	}
	stream.end()
	if _, open := <-resumed.Deltas(); open || resumed.Reason() != RunStreamEnded {
		t.Fatalf("ended subscription reason = %q", resumed.Reason())
	}
	if _, ok := stream.subscribe(); ok {
		t.Fatal("subscribed to ended stream")
	}
}

// TestSubscribeRunStreamAfterRetry 验证重试替换执行尝试后订阅新流，旧尝试迟到的增量进不了新流。
func TestSubscribeRunStreamAfterRetry(t *testing.T) {
	previous := &runningAgentRun{attempt: 1, streamID: "stream-1", stream: newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream-1", Attempt: 1})}
	action := &ExecuteAction{runningRuns: map[string]*runningAgentRun{"run": previous}}
	stale, ok := action.SubscribeRunStream("run")
	if !ok || stale.Snapshot.StreamID != "stream-1" {
		t.Fatalf("previous subscription = %#v", stale)
	}
	current := &runningAgentRun{attempt: 2, streamID: "stream-2", stream: newRunStream(agentruntime.StreamSnapshot{RunID: "run", StreamID: "stream-2", Attempt: 2})}
	action.runningRuns["run"] = current
	fresh, ok := action.SubscribeRunStream("run")
	if !ok || fresh.Snapshot.StreamID != "stream-2" || fresh.Snapshot.Attempt != 2 {
		t.Fatalf("current subscription = %#v", fresh)
	}
	tail := agentruntime.StreamDelta{RunID: "run", StreamID: "stream-1", Attempt: 1, Sequence: 1, Operations: []agentruntime.StreamOperation{{Kind: agentruntime.StreamOperationAppendCandidate, Text: "旧尝试尾部"}}}
	previous.stream.publish(tail)
	previous.stream.end()
	if _, open := <-stale.Deltas(); !open {
		t.Fatal("previous subscriber missed its own delta")
	}
	if _, open := <-stale.Deltas(); open || stale.Reason() != RunStreamEnded {
		t.Fatalf("previous subscription reason = %q", stale.Reason())
	}
	select {
	case delta := <-fresh.Deltas():
		t.Fatalf("current stream received previous delta: %#v", delta)
	default:
	}
	if _, err := fresh.Snapshot.Apply(tail); !errors.Is(err, agentruntime.ErrStreamMismatch) || fresh.Snapshot.CandidateContent != "" {
		t.Fatalf("apply previous tail error = %v, snapshot = %#v", err, fresh.Snapshot)
	}
}
