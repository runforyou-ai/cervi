//go:build server

package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// mergeKey 标识发送队列中可按最高版本合并的变更通知事件。
type mergeKey struct {
	frameType      protocol.Type
	conversationID string
}

// connection 是一条实时事件流，写协程独占响应写入。
type connection struct {
	gateway *Gateway
	id      string
	cancel  context.CancelFunc

	// subjects 在加入受众时写入，allowed 与 tokenSessionID 在建立连接时写入，之后只读。
	subjects       []string
	allowed        map[protocol.Type]bool
	tokenSessionID string

	mu         sync.Mutex
	queue      []protocol.Frame
	merged     map[mergeKey]int
	controller *http.ResponseController
	// epoch 在清除队列时递增，写协程据此丢弃已取出但未发送的事件。
	epoch   int
	wake    chan struct{}
	closing bool
}

// newConnection 创建尚未输出事件流的连接，cancel 结束该连接的请求处理。
func newConnection(gateway *Gateway, cancel context.CancelFunc, route streamRoute) *connection {
	allowed := make(map[protocol.Type]bool, len(route.allowed))
	for _, frameType := range route.allowed {
		allowed[frameType] = true
	}
	return &connection{
		gateway:        gateway,
		id:             uuid.NewV7().String(),
		cancel:         cancel,
		allowed:        allowed,
		tokenSessionID: route.tokenSessionID,
		merged:         map[mergeKey]int{},
		wake:           make(chan struct{}, 1),
	}
}

// run 按入队顺序写出事件并定期发送心跳，直到请求结束、写入失败或连接进入关闭状态且队列发送完毕。
func (c *connection) run(ctx context.Context, writer http.ResponseWriter, controller *http.ResponseController) {
	ping := time.NewTicker(c.gateway.options.PingInterval)
	defer ping.Stop()

	for {
		c.mu.Lock()
		frames, epoch := c.queue, c.epoch
		c.queue = nil
		clear(c.merged)
		c.mu.Unlock()

		for _, frame := range frames {
			c.mu.Lock()
			discarded := c.epoch != epoch
			c.mu.Unlock()
			if discarded {
				break
			}
			if !c.write(writer, controller, frame) {
				return
			}
		}

		c.mu.Lock()
		pending, closing := len(c.queue), c.closing
		c.mu.Unlock()
		if pending > 0 {
			continue
		}
		if closing {
			return
		}
		select {
		case <-c.wake:
		case <-ping.C:
			if !c.write(writer, controller, protocol.Ping{}) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// write 在写截止时间内以单条 SSE data 行写出事件并立即下发，失败时返回 false 结束事件流。
func (c *connection) write(writer http.ResponseWriter, controller *http.ResponseController, frame protocol.Frame) bool {
	data, err := protocol.Encode(frame)
	if err != nil {
		slog.Warn("编码实时事件失败", "connection_id", c.id, "type", frame.FrameType(), "error", err)
		return true
	}
	err = controller.SetWriteDeadline(time.Now().Add(c.gateway.options.WriteTimeout))
	if err == nil {
		_, err = writer.Write(append(append([]byte("data: "), data...), '\n', '\n'))
	}
	if err == nil {
		err = controller.Flush()
	}
	if err != nil {
		slog.Warn("实时事件流写入失败，结束事件流", "connection_id", c.id, "type", frame.FrameType(), "error", err)
		return false
	}
	return true
}

// send 把事件加入发送队列；受众可下发事件之外的事件直接丢弃，变更通知按会话与种类保留最高版本，队列溢出时按慢连接结束事件流。
func (c *connection) send(frame protocol.Frame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || !c.allowed[frame.FrameType()] {
		return
	}
	key, version, mergeable := mergeTarget(frame)
	if index, exists := c.merged[key]; mergeable && exists {
		if _, current, _ := mergeTarget(c.queue[index]); version > current {
			c.queue[index] = frame
		}
		return
	}
	if len(c.queue) >= c.gateway.options.QueueSize {
		slog.Warn("实时事件流发送队列溢出，按慢连接结束", "connection_id", c.id, "queued", len(c.queue))
		c.beginClose(true)
		return
	}
	if mergeable {
		c.merged[key] = len(c.queue)
	}
	c.queue = append(c.queue, frame)
	c.signal()
}

// mergeTarget 返回变更通知事件的合并键与版本，其他事件不可合并。
func mergeTarget(frame protocol.Frame) (mergeKey, int64, bool) {
	switch value := frame.(type) {
	case protocol.ConversationChanged:
		return mergeKey{protocol.TypeConversationChanged, value.ConversationID}, value.Version, true
	case protocol.ConversationStateChanged:
		return mergeKey{protocol.TypeConversationStateChanged, value.ConversationID}, value.Version, true
	case protocol.IdentityProfileChanged:
		return mergeKey{frameType: protocol.TypeIdentityProfileChanged}, value.Version, true
	}
	return mergeKey{}, 0, false
}

// revoke 清除未发送的事件并结束事件流。
func (c *connection) revoke(kind realtime.Kind) {
	slog.Info("实时事件流登录会话已撤销", "connection_id", c.id, "kind", kind)
	c.close(true)
}

// close 停止接收新事件，按需清除未发送的事件，由写协程发送剩余事件后结束事件流。
func (c *connection) close(discard bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return
	}
	c.beginClose(discard)
}

// beginClose 在持有连接锁时进入关闭状态并唤醒写协程。
func (c *connection) beginClose(discard bool) {
	c.closing = true
	if discard {
		c.queue = nil
		clear(c.merged)
		c.epoch++
	}
	c.signal()
}

// attach 登记事件流的响应控制器，连接已进入关闭状态时返回 false。
func (c *connection) attach(controller *http.ResponseController) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return false
	}
	c.controller = controller
	return true
}

// abort 取消请求处理，已输出事件流时让阻塞中的写入立即超时。
func (c *connection) abort() {
	c.cancel()
	c.mu.Lock()
	controller := c.controller
	c.mu.Unlock()
	if controller != nil {
		_ = controller.SetWriteDeadline(time.Now())
	}
}

// signal 唤醒写协程，已有待处理唤醒时直接返回。
func (c *connection) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
