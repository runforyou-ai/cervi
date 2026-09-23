//go:build server

package agentrun

import (
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// SubscribeRunStream 订阅本进程中运行当前执行尝试的临时过程流，返回快照后按序回调只读增量，执行尝试退出时回调结束；调用方负责会话访问校验。
func (a *ExecuteAction) SubscribeRunStream(runID string, onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, *agentruntime.StreamSubscription, bool) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	running := a.runningRuns[runID]
	if running == nil {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	return running.stream.Subscribe(onDelta, onEnd)
}
