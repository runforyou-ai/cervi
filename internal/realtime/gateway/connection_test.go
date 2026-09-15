//go:build server

package gateway

import (
	"reflect"
	"testing"

	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

// TestConnectionQueue 验证发送队列按会话与种类保留最高版本，失权帧不合并，溢出时清空队列并按慢连接关闭。
func TestConnectionQueue(t *testing.T) {
	current := newConnection(New(nil, "test", Options{QueueSize: 4}), nil)
	current.send(protocol.ConversationChanged{ConversationID: "a", Version: 2})
	current.send(protocol.ConversationChanged{ConversationID: "a", Version: 1})
	current.send(protocol.ConversationStateChanged{ConversationID: "a", Version: 5})
	current.send(protocol.ConversationRemoved{ConversationID: "a"})
	current.send(protocol.ConversationRemoved{ConversationID: "a"})
	current.send(protocol.ConversationChanged{ConversationID: "a", Version: 3})
	want := []protocol.Frame{
		protocol.ConversationChanged{ConversationID: "a", Version: 3},
		protocol.ConversationStateChanged{ConversationID: "a", Version: 5},
		protocol.ConversationRemoved{ConversationID: "a"},
		protocol.ConversationRemoved{ConversationID: "a"},
	}
	if !reflect.DeepEqual(current.queue, want) {
		t.Fatalf("queue = %#v, want %#v", current.queue, want)
	}

	// 队列已满时新帧触发慢连接关闭，关闭后不再入队。
	current.send(protocol.IdentityProfileChanged{Version: 1})
	current.send(protocol.Pong{})
	if !current.closing || current.closeReason != string(protocol.CloseSlowConsumer) || len(current.queue) != 0 {
		t.Fatalf("closing = %v, reason = %q, queue = %#v", current.closing, current.closeReason, current.queue)
	}
}

// TestConnectionRevokeDiscardsQueue 验证撤销清除未发送的帧，只保留撤销帧。
func TestConnectionRevokeDiscardsQueue(t *testing.T) {
	current := newConnection(New(nil, "test", Options{QueueSize: 4}), nil)
	current.send(protocol.ConversationChanged{ConversationID: "a", Version: 1})
	current.revoke(protocol.SessionRevokedLogout)
	want := []protocol.Frame{protocol.SessionRevoked{Reason: protocol.SessionRevokedLogout}}
	if !reflect.DeepEqual(current.queue, want) || current.closeReason != string(protocol.CloseSessionRevoked) {
		t.Fatalf("queue = %#v, reason = %q", current.queue, current.closeReason)
	}
}
