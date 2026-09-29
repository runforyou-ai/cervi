//go:build server

package agentrun

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
)

// fakeMCPConnection 记录关闭次数的测试 MCP 连接。
type fakeMCPConnection struct{ closed *atomic.Int32 }

// Tools 返回空工具目录。
func (c fakeMCPConnection) Tools(context.Context) ([]mcpintegration.Tool, error) { return nil, nil }

// Call 返回固定结果。
func (c fakeMCPConnection) Call(context.Context, string, json.RawMessage) (string, error) {
	return "ok", nil
}

// Close 记录一次关闭。
func (c fakeMCPConnection) Close() error {
	c.closed.Add(1)
	return nil
}

// TestDeviceMCPSessionsReuseDiscardAndRelease 验证同一运行复用连接，丢弃后重新建立，运行结束时关闭全部连接。
func TestDeviceMCPSessionsReuseDiscardAndRelease(t *testing.T) {
	var opened, closed atomic.Int32
	server := agentruntime.MCPServer{ID: "orders", Connect: func(context.Context) (agentruntime.MCPConnection, error) {
		opened.Add(1)
		return fakeMCPConnection{closed: &closed}, nil
	}}
	sessions := newDeviceMCPSessions()
	ctx := context.Background()
	use := func(discard bool) {
		t.Helper()
		if _, done, err := sessions.use(ctx, "run", server); err != nil {
			t.Fatal(err)
		} else {
			done(discard)
		}
	}
	use(false)
	use(false)
	if opened.Load() != 1 || closed.Load() != 0 {
		t.Fatalf("reuse opened=%d closed=%d", opened.Load(), closed.Load())
	}
	use(true)
	if closed.Load() != 1 {
		t.Fatalf("discard closed=%d", closed.Load())
	}
	use(false)
	if opened.Load() != 2 {
		t.Fatalf("reconnect opened=%d", opened.Load())
	}
	sessions.release("run")
	if closed.Load() != 2 || len(sessions.runs) != 0 {
		t.Fatalf("release closed=%d runs=%d", closed.Load(), len(sessions.runs))
	}
}

// TestDeviceMCPSessionsOpenFailure 验证建立连接失败时不缓存连接，下次重新建立。
func TestDeviceMCPSessionsOpenFailure(t *testing.T) {
	var attempts atomic.Int32
	var closed atomic.Int32
	server := agentruntime.MCPServer{ID: "orders", Connect: func(context.Context) (agentruntime.MCPConnection, error) {
		if attempts.Add(1) == 1 {
			return nil, errors.New("unavailable")
		}
		return fakeMCPConnection{closed: &closed}, nil
	}}
	sessions := newDeviceMCPSessions()
	if _, _, err := sessions.use(context.Background(), "run", server); err == nil {
		t.Fatal("open failure not reported")
	}
	if _, done, err := sessions.use(context.Background(), "run", server); err != nil {
		t.Fatal(err)
	} else {
		done(false)
	}
	sessions.release("run")
	if attempts.Load() != 2 || closed.Load() != 1 {
		t.Fatalf("attempts=%d closed=%d", attempts.Load(), closed.Load())
	}
}
