//go:build server

package agentrun

import (
	"log/slog"
	"sync"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// runStreamSubscriberBuffer 是单个订阅方可积压的增量数量。
const runStreamSubscriberBuffer = 64

// RunStreamCloseReason 表示运行流订阅结束的原因。
type RunStreamCloseReason string

const (
	// RunStreamEnded 表示执行尝试已退出，运行终态以持久数据为准。
	RunStreamEnded RunStreamCloseReason = "ended"
	// RunStreamOverflow 表示订阅方积压超过上限，需要重新订阅读取新快照。
	RunStreamOverflow RunStreamCloseReason = "overflow"
)

// runStream 保存一次执行尝试的最新快照，并向订阅方分发之后的增量。
type runStream struct {
	mu          sync.Mutex
	snapshot    agentruntime.StreamSnapshot
	subscribers map[*RunStreamSubscription]struct{}
	ended       bool
}

// RunStreamSubscription 定义一次运行流订阅，快照与之后的增量在同一把锁内衔接。
type RunStreamSubscription struct {
	Snapshot agentruntime.StreamSnapshot
	deltas   chan agentruntime.StreamDelta
	stream   *runStream
	reason   RunStreamCloseReason
}

// newRunStream 创建从空快照开始的运行流。
func newRunStream(snapshot agentruntime.StreamSnapshot) *runStream {
	return &runStream{snapshot: snapshot, subscribers: make(map[*RunStreamSubscription]struct{})}
}

// publish 把增量应用到快照并非阻塞地分发，积压超限的订阅方被关闭。
func (s *runStream) publish(delta agentruntime.StreamDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if _, err := s.snapshot.Apply(delta); err != nil {
		slog.Warn("Agent 运行流增量无法应用，结束运行流",
			"agent_run_id", delta.RunID, "stream_id", delta.StreamID, "sequence", delta.Sequence, "error", err)
		s.endLocked()
		return
	}
	for subscription := range s.subscribers {
		select {
		case subscription.deltas <- delta:
		default:
			slog.Warn("Agent 运行流订阅积压超限，关闭订阅",
				"agent_run_id", delta.RunID, "stream_id", delta.StreamID, "sequence", delta.Sequence)
			s.closeLocked(subscription, RunStreamOverflow)
		}
	}
}

// subscribe 返回当前快照并登记之后的增量接收，执行尝试已退出时返回 false。
func (s *runStream) subscribe() (*RunStreamSubscription, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return nil, false
	}
	subscription := &RunStreamSubscription{Snapshot: s.snapshot.Clone(), deltas: make(chan agentruntime.StreamDelta, runStreamSubscriberBuffer), stream: s}
	s.subscribers[subscription] = struct{}{}
	return subscription, true
}

// end 在执行尝试退出时结束全部订阅。
func (s *runStream) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endLocked()
}

// endLocked 标记运行流结束并关闭全部订阅，调用方持有运行流锁。
func (s *runStream) endLocked() {
	s.ended = true
	for subscription := range s.subscribers {
		s.closeLocked(subscription, RunStreamEnded)
	}
}

// closeLocked 记录原因后关闭订阅的增量通道，调用方持有运行流锁。
func (s *runStream) closeLocked(subscription *RunStreamSubscription, reason RunStreamCloseReason) {
	if _, ok := s.subscribers[subscription]; !ok {
		return
	}
	delete(s.subscribers, subscription)
	subscription.reason = reason
	close(subscription.deltas)
}

// Deltas 返回快照之后按序号递增的只读增量通道。
func (s *RunStreamSubscription) Deltas() <-chan agentruntime.StreamDelta {
	return s.deltas
}

// Reason 返回订阅结束原因，在增量通道关闭后读取；订阅方主动关闭时为空。
func (s *RunStreamSubscription) Reason() RunStreamCloseReason {
	return s.reason
}

// Close 取消订阅并关闭增量通道。
func (s *RunStreamSubscription) Close() {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()
	s.stream.closeLocked(s, "")
}

// SubscribeRunStream 订阅本进程中运行当前执行尝试的临时过程流，调用方负责会话访问校验。
func (a *ExecuteAction) SubscribeRunStream(runID string) (*RunStreamSubscription, bool) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	running := a.runningRuns[runID]
	if running == nil {
		return nil, false
	}
	return running.stream.subscribe()
}
