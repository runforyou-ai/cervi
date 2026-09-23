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
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// runStream 是一条运行过程流，写协程独占响应写入，待写事件由 queue 按序缓存。
type runStream struct {
	gateway *Gateway
	id      string
	runID   string
	cancel  context.CancelFunc
	queue   *protocol.RunStreamQueue

	// subjects、tokenSessionID 与 conversationID 在登记事件流前写入，之后只读。
	subjects       []string
	tokenSessionID string
	conversationID string

	mu         sync.Mutex
	controller *http.ResponseController
}

// newRunStream 创建尚未输出事件流的运行过程流，cancel 结束该流的请求处理。
func newRunStream(gateway *Gateway, runID string, cancel context.CancelFunc) *runStream {
	return &runStream{
		gateway: gateway,
		id:      uuid.NewV7().String(),
		runID:   runID,
		cancel:  cancel,
		queue:   protocol.NewRunStreamQueue(runID, gateway.options.RunPendingTextBytes),
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

	snapshot, unsubscribe, running := g.backend.SubscribeAgentRunStream(runID, current.queue.Publish, current.queue.Finish)
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
	current.queue.Snapshot(snapshot, g.options.RunSnapshotPartBytes)

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

// run 按序写出队列中的快照分片、合并后的增量与结束事件并定期发送心跳，事件流结束或关闭时返回。
func (s *runStream) run(ctx context.Context, writer http.ResponseWriter, controller *http.ResponseController) {
	ping := time.NewTicker(s.gateway.options.PingInterval)
	defer ping.Stop()

	for {
		frames, done := s.queue.Take()
		for _, frame := range frames {
			if !s.write(writer, controller, frame) {
				return
			}
		}
		if done {
			return
		}
		if len(frames) > 0 {
			continue
		}
		select {
		case <-s.queue.Wake():
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
	s.queue.Close()
}

// attach 登记事件流的响应控制器，事件流已进入关闭状态时返回 false。
func (s *runStream) attach(controller *http.ResponseController) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue.Closed() {
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
