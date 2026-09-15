//go:build server

package agentrun

import (
	"log/slog"
	"sync"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// runStream 保存一次执行尝试的最新快照，并按序向订阅方推送之后的增量。
type runStream struct {
	mu          sync.Mutex
	snapshot    agentruntime.StreamSnapshot
	subscribers map[*RunStreamSubscription]struct{}
	ended       bool
}

// RunStreamSubscription 定义一次运行流订阅，回调在运行流锁内串行执行，不得阻塞，也不得调用 Close 或 SubscribeRunStream。
type RunStreamSubscription struct {
	stream  *runStream
	onDelta func(agentruntime.StreamDelta)
	onEnd   func()
}

// newRunStream 创建从空快照开始的运行流。
func newRunStream(snapshot agentruntime.StreamSnapshot) *runStream {
	return &runStream{snapshot: snapshot, subscribers: make(map[*RunStreamSubscription]struct{})}
}

// publish 把增量应用到快照并推送给订阅方，增量无法应用时结束运行流。
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
		subscription.onDelta(delta)
	}
}

// subscribe 返回当前快照并登记之后的增量回调，运行流已结束时返回 false。
func (s *runStream) subscribe(onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, *RunStreamSubscription, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	subscription := &RunStreamSubscription{stream: s, onDelta: onDelta, onEnd: onEnd}
	s.subscribers[subscription] = struct{}{}
	return s.snapshot.Clone(), subscription, true
}

// end 在执行尝试退出时结束运行流。
func (s *runStream) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endLocked()
}

// endLocked 标记运行流结束并通知全部订阅方，调用方持有运行流锁。
func (s *runStream) endLocked() {
	s.ended = true
	for subscription := range s.subscribers {
		delete(s.subscribers, subscription)
		subscription.onEnd()
	}
}

// Close 取消订阅，之后不再回调。
func (s *RunStreamSubscription) Close() {
	s.stream.mu.Lock()
	defer s.stream.mu.Unlock()
	delete(s.stream.subscribers, s)
}

// SubscribeRunStream 订阅本进程中运行当前执行尝试的临时过程流，返回快照后按序回调只读增量，执行尝试退出时回调结束；调用方负责会话访问校验。
func (a *ExecuteAction) SubscribeRunStream(runID string, onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, *RunStreamSubscription, bool) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	running := a.runningRuns[runID]
	if running == nil {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	return running.stream.subscribe(onDelta, onEnd)
}
