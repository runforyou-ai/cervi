//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
)

// stubRunClient 在内存中模拟设备运行期接口，记录领取、收尾与失败上报。
type stubRunClient struct {
	mu        sync.Mutex
	work      appservice.DeviceWork
	claims    []string
	completed map[string]string
	failures  map[string]appservice.DeviceRunFailureCode
	// failedBlocks 按运行编号记录失败上报携带的过程内容块。
	failedBlocks map[string]json.RawMessage
	busy         map[string]bool
	// blockPeek 为 true 时读取输入阻塞到运行 context 结束。
	blockPeek bool
	leaseEnd  bool
	peeked    chan string
	// assignment 非空时作为领取返回的有效配置。
	assignment json.RawMessage
	// searches 记录知识检索请求。
	searches []json.RawMessage
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
	assignment := c.assignment
	if assignment == nil {
		assignment = json.RawMessage("{}")
	}
	return appservice.DeviceRunClaim{Assignment: assignment, LeaseExpiresAt: time.Now().Add(time.Minute), LeaseRenewIntervalSeconds: 3600}, nil
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

// SearchDeviceRunKnowledge 记录检索请求并返回一条固定记录。
func (c *stubRunClient) SearchDeviceRunKnowledge(_ context.Context, _ appservice.RequestMeta, _ string, input appservice.DeviceRunKnowledgeSearchInput) (appservice.DeviceRunKnowledgeSearchResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.searches = append(c.searches, input.Request)
	return appservice.DeviceRunKnowledgeSearchResult{Result: json.RawMessage(`{"records":[{"content":"退款三天到账"}]}`)}, nil
}

// ReadDeviceRunAttachment 返回附件消息编号对应的固定内容。
func (c *stubRunClient) ReadDeviceRunAttachment(_ context.Context, _ appservice.RequestMeta, _, messageID string) ([]byte, error) {
	return []byte("content:" + messageID), nil
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
	c.failedBlocks[runID] = input.Blocks
	return nil
}

// OpenDeviceEventStream 在测试中不建立事件流。
func (c *stubRunClient) OpenDeviceEventStream(context.Context, appservice.RequestMeta) (io.ReadCloser, error) {
	return nil, io.EOF
}

// DeviceModelEndpoint 返回固定的模型代理入口。
func (c *stubRunClient) DeviceModelEndpoint(context.Context, appservice.RequestMeta, string) (string, http.RoundTripper, error) {
	return "https://cervi.example.com/api/agent-runs/run/model", http.DefaultTransport, nil
}

// stubRuntime 读取并认领全部输入，以收到的上下文消息数量作为回复；failure 非空时返回该错误与一个过程内容块，inspect 非空时先检查运行请求。
type stubRuntime struct {
	failure error
	inspect func(context.Context, agentruntime.RunRequest)
}

// Run 按预设认领输入并返回回复或失败。
func (r stubRuntime) Run(ctx context.Context, request agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
	if r.inspect != nil {
		r.inspect(ctx, request)
	}
	triggers, err := feed.Peek(ctx, 0)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
	if err != nil {
		return agentruntime.RunResult{}, err
	}
	if r.failure != nil {
		return agentruntime.RunResult{Blocks: []agentruntime.Block{{ID: "block-1"}}}, r.failure
	}
	return agentruntime.RunResult{Content: fmt.Sprintf("收到 %d 条上下文消息", len(claimed.Messages)), EndSeq: claimed.EndSeq}, nil
}

// stubWorkspaceStore 在内存中保存工作区路径。
type stubWorkspaceStore map[string]string

// LoadAgentWorkspacePath 读取内存中的工作区路径。
func (s stubWorkspaceStore) LoadAgentWorkspacePath(_ context.Context, _, _, workspaceID string) (string, bool, error) {
	path, found := s[workspaceID]
	return path, found, nil
}

// newTestWorker 创建已登录并已注册设备的执行循环，不启动后台循环。
func newTestWorker(t *testing.T, client *stubRunClient, workspaces stubWorkspaceStore, runtime stubRuntime) *Worker {
	t.Helper()
	const serverURL = "https://cervi.example.com"
	store := &stubStore{installID: "install-1", registrations: map[string]string{serverURL + "|org-1|user-1": "device-1"}}
	registrar, sessions := newTestRegistrar(t, store, &stubClient{serverURL: serverURL, deviceID: "device-1"})
	if err := sessions.Establish(context.Background(), credentialFor(serverURL, "org-1", "user-1", "token-1")); err != nil {
		t.Fatal(err)
	}
	client.completed = map[string]string{}
	client.failures = map[string]appservice.DeviceRunFailureCode{}
	client.failedBlocks = map[string]json.RawMessage{}
	worker := NewWorker(registrar, workspaces, client, runtime)
	t.Cleanup(worker.Stop)
	return worker
}

// TestWorkerSerializesWorkspace 验证同一工作区只领取一个运行，执行完成后再领取下一个并回报运行时的回复。
func TestWorkerSerializesWorkspace(t *testing.T) {
	directory := t.TempDir()
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{
		{RunID: "run-1", WorkspaceID: "workspace-1"}, {RunID: "run-2", WorkspaceID: "workspace-1"},
	}}}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": directory}, stubRuntime{})

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
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": t.TempDir()}, stubRuntime{})

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
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-2": t.TempDir() + "/removed"}, stubRuntime{})

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
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": t.TempDir()}, stubRuntime{})

	worker.poll()
	<-client.peeked
	worker.nudgeLeases()
	worker.runs.Wait()
	if len(client.completed) != 0 || len(client.failures) != 0 || len(worker.active) != 0 {
		t.Fatalf("收尾 = %v，失败上报 = %v，本机登记 = %v", client.completed, client.failures, worker.active)
	}
}

// TestWorkerReportsRuntimeFailure 验证运行时出错时上报运行失败并携带已产生的过程内容块。
func TestWorkerReportsRuntimeFailure(t *testing.T) {
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1", WorkspaceID: "workspace-1"}}}}
	worker := newTestWorker(t, client, stubWorkspaceStore{"workspace-1": t.TempDir()}, stubRuntime{failure: errors.New("model unavailable")})

	worker.poll()
	worker.runs.Wait()
	var blocks []agentruntime.Block
	if err := json.Unmarshal(client.failedBlocks["run-1"], &blocks); err != nil {
		t.Fatal(err)
	}
	if client.failures["run-1"] != appservice.DeviceRunFailureRuntimeFailed || len(blocks) != 1 || len(client.completed) != 0 {
		t.Fatalf("失败上报 = %v，过程内容 = %v，收尾 = %v", client.failures, blocks, client.completed)
	}
}

// TestWorkerWiresRunDependencies 验证有效配置含知识检索时经服务端检索、附件经服务端读取，运行流增量可由本机订阅读取且运行结束后结束订阅。
func TestWorkerWiresRunDependencies(t *testing.T) {
	client := &stubRunClient{
		work:       appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1"}}},
		assignment: json.RawMessage(`{"tools":["search_knowledge"]}`),
	}
	var worker *Worker
	var (
		records  []knowledgeretrieval.Record
		content  []byte
		received []agentruntime.StreamDelta
		ended    bool
		snapshot agentruntime.StreamSnapshot
	)
	endedOnce := make(chan struct{})
	worker = newTestWorker(t, client, stubWorkspaceStore{}, stubRuntime{inspect: func(ctx context.Context, request agentruntime.RunRequest) {
		result, err := request.KnowledgeSearch(ctx, knowledgeretrieval.Request{Queries: []string{"退款"}})
		if err != nil {
			t.Errorf("knowledge search: %v", err)
		}
		records = result.Records
		if content, err = request.ReadAttachment(ctx, "message-1"); err != nil {
			t.Errorf("read attachment: %v", err)
		}
		var subscribed bool
		snapshot, _, subscribed = worker.SubscribeLocalRunStream("run-1", func(delta agentruntime.StreamDelta) {
			received = append(received, delta)
		}, func() {
			ended = true
			close(endedOnce)
		})
		if !subscribed {
			t.Error("local run stream is not available")
		}
		request.OnStream(agentruntime.StreamDelta{RunID: "run-1", StreamID: request.StreamID, Attempt: 1, BaseSequence: 0, Sequence: 1,
			Operations: []agentruntime.StreamOperation{{Kind: agentruntime.StreamOperationAppendCandidate, Text: "处理中"}}})
	}})

	worker.poll()
	worker.runs.Wait()
	<-endedOnce
	if len(records) != 1 || records[0].Content != "退款三天到账" || len(client.searches) != 1 {
		t.Fatalf("检索结果 = %#v，检索请求 = %s", records, client.searches)
	}
	if string(content) != "content:message-1" {
		t.Fatalf("附件内容 = %q", content)
	}
	if snapshot.RunID != "run-1" || snapshot.StreamID == "" || len(received) != 1 || received[0].Sequence != 1 || !ended {
		t.Fatalf("快照 = %#v，增量 = %#v，结束 = %t", snapshot, received, ended)
	}
	if _, _, ok := worker.SubscribeLocalRunStream("run-1", func(agentruntime.StreamDelta) {}, func() {}); ok {
		t.Fatal("subscribed to finished local run stream")
	}
}

// TestWorkerOmitsKnowledgeSearch 验证有效配置不含知识检索时不注入检索函数。
func TestWorkerOmitsKnowledgeSearch(t *testing.T) {
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1"}}}}
	injected := true
	worker := newTestWorker(t, client, stubWorkspaceStore{}, stubRuntime{inspect: func(_ context.Context, request agentruntime.RunRequest) {
		injected = request.KnowledgeSearch != nil
	}})

	worker.poll()
	worker.runs.Wait()
	if injected {
		t.Fatal("knowledge search injected without knowledge tool")
	}
}

// TestWorkerStreamFollowsReservation 验证运行登记后即可订阅本机过程流，释放登记时过程流随之结束。
func TestWorkerStreamFollowsReservation(t *testing.T) {
	worker := newTestWorker(t, &stubRunClient{}, stubWorkspaceStore{}, stubRuntime{})
	if !worker.reserve(appservice.DeviceWorkRun{RunID: "run-1"}) || !worker.RunsLocally("run-1") {
		t.Fatal("reserved run is not local")
	}
	ended := false
	snapshot, _, ok := worker.SubscribeLocalRunStream("run-1", func(agentruntime.StreamDelta) {}, func() { ended = true })
	if !ok || snapshot.RunID != "run-1" || snapshot.StreamID == "" || snapshot.Attempt != 1 {
		t.Fatalf("snapshot = %#v, ok = %v", snapshot, ok)
	}
	worker.release("run-1")
	if !ended || worker.RunsLocally("run-1") {
		t.Fatalf("ended = %v, local = %v", ended, worker.RunsLocally("run-1"))
	}
}
