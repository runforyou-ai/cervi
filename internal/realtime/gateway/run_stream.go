//go:build server

package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// runStream 是一条运行过程流，写协程独占响应写入；快照分片先于增量写出，待发增量按序号相接合并为一条。
type runStream struct {
	gateway *Gateway
	id      string
	runID   string
	cancel  context.CancelFunc

	// subjects、tokenSessionID 与 conversationID 在登记事件流前写入，之后只读。
	subjects       []string
	tokenSessionID string
	conversationID string

	mu         sync.Mutex
	snapshot   []protocol.RunStreamSnapshot
	delta      *agentruntime.StreamDelta
	controller *http.ResponseController
	// ended 表示源运行流已结束，剩余事件写出后补发结束事件。
	ended   bool
	closing bool
	wake    chan struct{}
}

// newRunStream 创建尚未输出事件流的运行过程流，cancel 结束该流的请求处理。
func newRunStream(gateway *Gateway, runID string, cancel context.CancelFunc) *runStream {
	return &runStream{
		gateway: gateway,
		id:      uuid.NewV7().String(),
		runID:   runID,
		cancel:  cancel,
		wake:    make(chan struct{}, 1),
	}
}

// serveRun 认证请求、校验运行所属会话的阅读资格后挂接该运行的过程流，直到运行流结束或请求断开。
func (g *Gateway) serveRun(writer http.ResponseWriter, request *http.Request, runID string) {
	meta := appservice.RequestMeta{Token: bearerToken(request.Header.Get("Authorization")), Locale: appservice.Locale(request.Header.Get("Accept-Language"))}
	identity, err := g.backend.AuthenticateMember(request.Context(), meta)
	if err != nil {
		writeError(writer, meta, err)
		return
	}
	conversationID, err := g.backend.AuthorizeAgentRunStream(request.Context(), meta, identity, runID)
	if err != nil {
		writeError(writer, meta, err)
		return
	}
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	current := newRunStream(g, runID, cancel)
	current.tokenSessionID, current.conversationID = identity.Token.ID, conversationID
	// 运行过程流只加入本人用户受众，用于接收登出、停用与所属会话失权的撤销控制。
	current.subjects = []string{realtime.Subject(g.namespace, identity.Organization.ID, realtime.AudienceUser, identity.User.ID)}
	if !g.register(current) {
		writeUnavailable(writer, meta)
		return
	}
	defer g.unregister(current)
	if err := g.joinAudiences(ctx, current); err != nil {
		slog.Warn("运行过程流受众订阅失败", "stream_id", current.id, "agent_run_id", runID, "user_id", identity.User.ID, "error", err)
		writeUnavailable(writer, meta)
		return
	}
	// 订阅生效后再次校验登录会话与阅读资格，之后提交的登出、停用与失权经受众通知送达。
	if _, err := g.backend.AuthenticateMember(ctx, meta); err != nil {
		writeError(writer, meta, err)
		return
	}
	if _, err := g.backend.AuthorizeAgentRunStream(ctx, meta, identity, runID); err != nil {
		writeError(writer, meta, err)
		return
	}

	snapshot, unsubscribe, running := g.backend.SubscribeAgentRunStream(runID, current.publish, current.finish)
	if running {
		defer unsubscribe()
	}

	// 事件流是长响应：清除服务器读超时，写超时按每次写入设置；网关已开始下线时不输出事件流。
	controller := http.NewResponseController(writer)
	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		slog.Warn("清除运行过程流读超时失败", "stream_id", current.id, "error", err)
		writeUnavailable(writer, meta)
		return
	}
	if !current.attach(controller) {
		writeUnavailable(writer, meta)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	if !running {
		// 运行不在本进程执行：直接结束该流，客户端按持久事实收敛。
		current.write(writer, controller, protocol.RunStreamEnded{RunID: runID})
		return
	}
	current.enqueueSnapshot(snapshot)

	// 事件流最长存活时间不晚于登录会话到期。
	lifetime := min(g.options.MaxLifetime, time.Until(identity.Token.ExpiresAt))
	expiry := time.AfterFunc(lifetime, func() {
		slog.Info("运行过程流到达最长存活时间", "stream_id", current.id)
		current.close()
	})
	defer expiry.Stop()
	slog.Info("运行过程流已就绪", "stream_id", current.id, "agent_run_id", runID,
		"organization_id", identity.Organization.ID, "user_id", identity.User.ID, "sequence", snapshot.Sequence)
	current.run(ctx, writer, controller)
	slog.Info("运行过程流已结束", "stream_id", current.id, "agent_run_id", runID, "user_id", identity.User.ID)
}

// run 先写出快照分片再写出合并后的增量并定期发送心跳，源运行流结束时补发结束事件。
func (s *runStream) run(ctx context.Context, writer http.ResponseWriter, controller *http.ResponseController) {
	ping := time.NewTicker(s.gateway.options.PingInterval)
	defer ping.Stop()

	for {
		s.mu.Lock()
		snapshot, delta := s.snapshot, s.delta
		s.snapshot, s.delta = nil, nil
		s.mu.Unlock()

		for _, part := range snapshot {
			if !s.write(writer, controller, part) {
				return
			}
		}
		if delta != nil && !s.write(writer, controller, runStreamDelta(*delta)) {
			return
		}

		s.mu.Lock()
		pending, ended, closing := len(s.snapshot) > 0 || s.delta != nil, s.ended, s.closing
		s.mu.Unlock()
		if pending {
			continue
		}
		if closing {
			return
		}
		if ended {
			s.write(writer, controller, protocol.RunStreamEnded{RunID: s.runID})
			return
		}
		select {
		case <-s.wake:
		case <-ping.C:
			if !s.write(writer, controller, protocol.Ping{}) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// write 在写截止时间内以单条 SSE data 行写出事件并立即下发，失败时返回 false 结束事件流。
func (s *runStream) write(writer http.ResponseWriter, controller *http.ResponseController, frame protocol.Frame) bool {
	data, err := protocol.Encode(frame)
	if err != nil {
		slog.Warn("编码运行过程事件失败", "stream_id", s.id, "type", frame.FrameType(), "error", err)
		return true
	}
	err = controller.SetWriteDeadline(time.Now().Add(s.gateway.options.WriteTimeout))
	if err == nil {
		_, err = writer.Write(append(append([]byte("data: "), data...), '\n', '\n'))
	}
	if err == nil {
		err = controller.Flush()
	}
	if err != nil {
		slog.Warn("运行过程流写入失败，结束事件流", "stream_id", s.id, "type", frame.FrameType(), "error", err)
		return false
	}
	return true
}

// enqueueSnapshot 把订阅时的快照按文本预算拆分为分片事件，排在全部增量之前写出。
func (s *runStream) enqueueSnapshot(snapshot agentruntime.StreamSnapshot) {
	parts := splitSnapshot(snapshot, s.gateway.options.RunSnapshotPartBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return
	}
	s.snapshot = parts
	s.signal()
}

// publish 合并待发增量；增量不相接或合并后超出文本上限时按慢消费者结束事件流，由客户端重新取快照。
func (s *runStream) publish(delta agentruntime.StreamDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || s.ended {
		return
	}
	if s.delta == nil {
		s.delta = &delta
	} else if merged, ok := agentruntime.MergeStreamDeltas(*s.delta, delta); ok {
		s.delta = &merged
	} else {
		slog.Warn("运行过程流待发增量不相接，结束事件流", "stream_id", s.id, "agent_run_id", s.runID,
			"pending_sequence", s.delta.Sequence, "base_sequence", delta.BaseSequence)
		s.beginClose()
		return
	}
	if size := deltaTextBytes(*s.delta); size > s.gateway.options.RunPendingTextBytes {
		slog.Warn("运行过程流待发增量超出上限，按慢消费者结束", "stream_id", s.id, "agent_run_id", s.runID, "pending_bytes", size)
		s.beginClose()
		return
	}
	s.signal()
}

// finish 登记源运行流已结束，剩余事件写出后补发结束事件。
func (s *runStream) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || s.ended {
		return
	}
	s.ended = true
	s.signal()
}

// tokenSession 返回事件流所属登录会话编号。
func (s *runStream) tokenSession() string { return s.tokenSessionID }

// audienceSubjects 返回事件流加入的受众 Subject。
func (s *runStream) audienceSubjects() []string { return s.subjects }

// revoke 因登出、停用或所属会话失权清除未发送的事件并结束事件流。
func (s *runStream) revoke(kind realtime.Kind) {
	slog.Info("运行过程流按撤销控制结束", "stream_id", s.id, "agent_run_id", s.runID, "kind", kind)
	s.close()
}

// shutdown 在网关下线时结束事件流，客户端重新请求后取新快照。
func (s *runStream) shutdown() { s.close() }

// close 清除未发送的事件并结束事件流，不补发结束事件。
func (s *runStream) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return
	}
	s.beginClose()
}

// beginClose 在持有流锁时进入关闭状态、清除未发送的事件并唤醒写协程。
func (s *runStream) beginClose() {
	s.closing = true
	s.snapshot, s.delta = nil, nil
	s.signal()
}

// attach 登记事件流的响应控制器，事件流已进入关闭状态时返回 false。
func (s *runStream) attach(controller *http.ResponseController) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.controller = controller
	return true
}

// abort 取消请求处理，已输出事件流时让阻塞中的写入立即超时。
func (s *runStream) abort() {
	s.cancel()
	s.mu.Lock()
	controller := s.controller
	s.mu.Unlock()
	if controller != nil {
		_ = controller.SetWriteDeadline(time.Now())
	}
}

// signal 唤醒写协程，已有待处理唤醒时直接返回。
func (s *runStream) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// splitSnapshot 把快照按文本预算拆分为分片事件，每个分片至少包含一个内容块。
// 候选正文整段放在首个分片并计入该分片预算，其长度由模型单次最大输出约束。
func splitSnapshot(snapshot agentruntime.StreamSnapshot, budget int) []protocol.RunStreamSnapshot {
	header := protocol.RunStreamSnapshot{RunID: snapshot.RunID, StreamID: snapshot.StreamID, Attempt: snapshot.Attempt, Sequence: snapshot.Sequence}
	first := header
	first.CandidateContent, first.Blocks = snapshot.CandidateContent, []protocol.RunStreamBlock{}
	parts := []protocol.RunStreamSnapshot{first}
	size := len(snapshot.CandidateContent)
	for _, block := range snapshot.Blocks {
		view := runStreamBlock(block)
		last := &parts[len(parts)-1]
		if len(last.Blocks) > 0 && size+len(view.Text) > budget {
			next := header
			next.Blocks = []protocol.RunStreamBlock{}
			parts = append(parts, next)
			last, size = &parts[len(parts)-1], 0
		}
		last.Blocks = append(last.Blocks, view)
		size += len(view.Text)
	}
	for i := range parts {
		parts[i].Part, parts[i].PartCount = i, len(parts)
	}
	return parts
}

// runStreamDelta 把运行流增量转换为事件契约中的增量。
func runStreamDelta(delta agentruntime.StreamDelta) protocol.RunStreamDelta {
	operations := make([]protocol.RunStreamOperation, 0, len(delta.Operations))
	for _, operation := range delta.Operations {
		item := protocol.RunStreamOperation{
			Kind:     protocol.RunStreamOperationKind(operation.Kind),
			BlockID:  operation.BlockID,
			BlockIDs: operation.BlockIDs,
			Text:     operation.Text,
		}
		if operation.Block != nil {
			block := runStreamBlock(*operation.Block)
			item.Block = &block
		}
		operations = append(operations, item)
	}
	return protocol.RunStreamDelta{RunID: delta.RunID, StreamID: delta.StreamID, Attempt: delta.Attempt,
		BaseSequence: delta.BaseSequence, Sequence: delta.Sequence, Operations: operations}
}

// runStreamBlock 把运行流内容块转换为事件契约中的展示块。
func runStreamBlock(block agentruntime.StreamBlock) protocol.RunStreamBlock {
	view := protocol.RunStreamBlock{ID: block.ID, Position: block.Position, Kind: block.Kind, Text: block.Text}
	if call := block.ToolCall; call != nil {
		view.ToolCall = &protocol.RunStreamToolCall{Name: call.Name, Status: call.Status, StartedAt: call.StartedAt, CompletedAt: call.CompletedAt}
	}
	return view
}

// deltaTextBytes 返回增量中全部操作携带的文本字节数。
func deltaTextBytes(delta agentruntime.StreamDelta) int {
	total := 0
	for _, operation := range delta.Operations {
		total += len(operation.Text)
		if operation.Block != nil {
			total += len(operation.Block.Text)
		}
	}
	return total
}
