//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

// stubRunClient 在内存中模拟设备运行期接口，记录领取、收尾与失败上报。
type stubRunClient struct {
	mu        sync.Mutex
	work      appservice.DeviceWork
	claims    []string
	completed map[string]string
	failures  map[string]appservice.DeviceRunFailureCode
	busy      map[string]bool
	// blockPeek 为 true 时读取输入阻塞到运行 context 结束。
	blockPeek bool
	leaseEnd  bool
	peeked    chan string
}

// GetDeviceWork 返回预设的待领取运行。
func (c *stubRunClient) GetDeviceWork(context.Context, appservice.RequestMeta) (appservice.DeviceWork, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.work, nil
}

// ClaimDeviceRun 记录领取，预设为工作区忙的运行返回冲突。
func (c *stubRunClient) ClaimDeviceRun(_ context.Context, _ appservice.RequestMeta, runID string) (appservice.DeviceRunClaim, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy[runID] {
		return appservice.DeviceRunClaim{}, &appservice.Error{Kind: appservice.ErrorKindConflict, Reason: "workspace_busy"}
	}
	c.claims = append(c.claims, runID)
	return appservice.DeviceRunClaim{Assignment: json.RawMessage("{}"), LeaseExpiresAt: time.Now().Add(time.Minute), LeaseRenewIntervalSeconds: 3600}, nil
}

// RenewDeviceRunLease 按预设返回运行是否已结束。
func (c *stubRunClient) RenewDeviceRunLease(context.Context, appservice.RequestMeta, string) (appservice.DeviceRunLease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return appservice.DeviceRunLease{Ended: c.leaseEnd}, nil
}

// PeekDeviceRunInputs 返回一条输入，需要时阻塞到运行被取消。
func (c *stubRunClient) PeekDeviceRunInputs(ctx context.Context, _ appservice.RequestMeta, runID string, _ appservice.DeviceRunInputPeekInput) (appservice.DeviceRunInputSignals, error) {
	c.mu.Lock()
	block := c.blockPeek
	c.mu.Unlock()
	if c.peeked != nil {
		c.peeked <- runID
	}
	if block {
		<-ctx.Done()
		return appservice.DeviceRunInputSignals{}, ctx.Err()
	}
	return appservice.DeviceRunInputSignals{Seqs: []int64{1}}, nil
}

// ClaimDeviceRunInputs 返回两条上下文消息。
func (c *stubRunClient) ClaimDeviceRunInputs(context.Context, appservice.RequestMeta, string, appservice.DeviceRunInputClaimInput) (appservice.DeviceRunClaimedInput, error) {
	return appservice.DeviceRunClaimedInput{EndSeq: 1, Messages: json.RawMessage(`[{},{}]`)}, nil
}

// CompleteDeviceRun 记录收尾正文。
func (c *stubRunClient) CompleteDeviceRun(_ context.Context, _ appservice.RequestMeta, runID string, input appservice.DeviceRunResultInput) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completed[runID] = input.Content
	return nil
}

// FailDeviceRun 记录失败原因。
func (c *stubRunClient) FailDeviceRun(_ context.Context, _ appservice.RequestMeta, runID string, input appservice.DeviceRunFailureInput) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[runID] = input.ErrorCode
	return nil
}

// OpenDeviceEventStream 在测试中不建立事件流。
func (c *stubRunClient) OpenDeviceEventStream(context.Context, appservice.RequestMeta) (io.ReadCloser, error) {
	return nil, io.EOF
}

// stubWorkspaceStore 在内存中保存工作区路径。
type stubWorkspaceStore map[string]string

// LoadAgentWorkspacePath 读取内存中的工作区路径。
func (s stubWorkspaceStore) LoadAgentWorkspacePath(_ context.Context, _, _, workspaceID string) (string, bool, error) {
	path, found := s[workspaceID]
	return path, found, nil
}

// newTestWorker 创建已登录并已注册设备的执行循环，不启动后台循环。
func newTestWorker(t *testing.T, client *stubRunClient, workspaces stubWorkspaceStore) *Worker {
	t.Helper()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{serverURL + "|org-1|user-1": "device-1"}}
	registrar, sessions := newTestRegistrar(t, store, &stubClient{serverURL: serverURL, deviceID: "device-1"})
	if err := sessions.Establish(context.Background(), credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}
	client.completed = map[string]string{}
	client.failures = map[string]appservice.DeviceRunFailureCode{}
	worker := NewWorker(registrar, workspaces, client)
	t.Cleanup(worker.Stop)
	return worker
}

// TestWorkerSerializesWorkspace 验证同一工作区只领取一个运行，执行完成后再领取下一个并回报本机收到的上下文。
func TestWorkerSerializesWorkspace(t *testing.T) {
	directory := t.TempDir()
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{
		{RunID: "run-1", WorkspaceID: "workspace-1"}, {RunID: "run-2", WorkspaceID: "workspace-1"},
	}}}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": directory})

	worker.poll()
	worker.runs.Wait()
	client.mu.Lock()
	if len(client.claims) != 1 || client.claims[0] != "run-1" || client.completed["run-1"] == "" {
		t.Fatalf("首轮领取 = %v，收尾 = %v", client.claims, client.completed)
	}
	client.work.Runs = client.work.Runs[1:]
	client.mu.Unlock()

	worker.poll()
	worker.runs.Wait()
	if len(client.claims) != 2 || client.claims[1] != "run-2" {
		t.Fatalf("次轮领取 = %v", client.claims)
	}
}

// TestWorkerRetriesBusyWorkspace 验证服务端判定工作区忙时不执行并要求尽快重新检查。
func TestWorkerRetriesBusyWorkspace(t *testing.T) {
	client := &stubRunClient{
		work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1", WorkspaceID: "workspace-1"}}},
		busy: map[string]bool{"run-1": true},
	}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": t.TempDir()})

	if !worker.poll() {
		t.Fatal("工作区忙时没有要求重新检查")
	}
	if len(client.claims) != 0 || len(worker.active) != 0 {
		t.Fatalf("领取 = %v，本机登记 = %v", client.claims, worker.active)
	}
}

// TestWorkerReportsMissingWorkspace 验证本机找不到工作区目录时不领取并上报工作区缺失。
func TestWorkerReportsMissingWorkspace(t *testing.T) {
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{
		{RunID: "run-1", WorkspaceID: "workspace-1"}, {RunID: "run-2", WorkspaceID: "workspace-2"},
	}}}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-2": t.TempDir() + "/removed"})

	worker.poll()
	if len(client.claims) != 0 ||
		client.failures["run-1"] != appservice.DeviceRunFailureWorkspaceMissing ||
		client.failures["run-2"] != appservice.DeviceRunFailureWorkspaceMissing {
		t.Fatalf("领取 = %v，失败上报 = %v", client.claims, client.failures)
	}
}

// TestWorkerStopsEndedRun 验证续租得知运行已结束后中断本机执行，且不上报失败。
func TestWorkerStopsEndedRun(t *testing.T) {
	client := &stubRunClient{
		work:      appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1", WorkspaceID: "workspace-1"}}},
		blockPeek: true,
		leaseEnd:  true,
		peeked:    make(chan string, 1),
	}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": t.TempDir()})

	worker.poll()
	<-client.peeked
	worker.nudgeLeases()
	worker.runs.Wait()
	if len(client.completed) != 0 || len(client.failures) != 0 || len(worker.active) != 0 {
		t.Fatalf("收尾 = %v，失败上报 = %v，本机登记 = %v", client.completed, client.failures, worker.active)
	}
}
