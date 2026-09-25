//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	"github.com/runforyou-ai/cervi/internal/integration/localmcp"
	"github.com/runforyou-ai/cervi/internal/integration/localworkspace"
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

// ClaimDeviceRun 记录领取并返回预设的有效配置。
func (c *stubRunClient) ClaimDeviceRun(_ context.Context, _ appservice.RequestMeta, runID string) (appservice.DeviceRunClaim, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
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

// SearchDeviceRunWeb 返回一条固定的搜索结果。
func (c *stubRunClient) SearchDeviceRunWeb(_ context.Context, _ appservice.RequestMeta, _ string, _ appservice.DeviceRunWebSearchInput) (appservice.DeviceRunWebSearchResult, error) {
	return appservice.DeviceRunWebSearchResult{Result: json.RawMessage(`{"items":[{"title":"退款政策","url":"https://example.com/refund"}]}`)}, nil
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

// stubToolchain 按预设返回是否可以领取运行并记录检查次数，不改动命令环境变量。
type stubToolchain struct {
	ready  bool
	checks int
}

// Ensure 记录检查并返回预设结果。
func (s *stubToolchain) Ensure() bool {
	s.checks++
	return s.ready
}

// Environment 返回不改动命令环境变量的设置。
func (s *stubToolchain) Environment() localworkspace.Environment {
	return localworkspace.Environment{}
}

// Close 不做任何事。
func (s *stubToolchain) Close() {}

// newTestWorker 创建已登录并已注册设备的执行循环，不启动后台循环。
func newTestWorker(t *testing.T, client *stubRunClient, runtime stubRuntime) *Worker {
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
	worker := NewWorker(registrar, client, runtime, &stubToolchain{ready: true}, localmcp.NewStore(filepath.Join(t.TempDir(), "mcp.json"), func() {}), t.TempDir())
	t.Cleanup(worker.Stop)
	return worker
}

// TestWorkerRunsInConversationFolder 验证各会话的运行都被领取执行并回报回复，本机文件以会话默认文件夹为起点且文件夹自动创建。
func TestWorkerRunsInConversationFolder(t *testing.T) {
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{
		{RunID: "run-1", ConversationID: "conversation-1"}, {RunID: "run-2", ConversationID: "conversation-2"},
	}}}
	var mu sync.Mutex
	listed := map[string]bool{}
	worker := newTestWorker(t, client, stubRuntime{inspect: func(ctx context.Context, request agentruntime.RunRequest) {
		_, err := request.Workspace.LsInfo(ctx, &filesystem.LsInfoRequest{})
		mu.Lock()
		listed[request.RunID] = err == nil
		mu.Unlock()
	}})

	worker.poll()
	worker.runs.Wait()
	if len(client.claims) != 2 || client.completed["run-1"] == "" || client.completed["run-2"] == "" || !listed["run-1"] || !listed["run-2"] {
		t.Fatalf("领取 = %v，收尾 = %v，读取默认文件夹 = %v", client.claims, client.completed, listed)
	}
	for _, conversationID := range []string{"conversation-1", "conversation-2"} {
		if info, err := os.Stat(filepath.Join(worker.folders, conversationID)); err != nil || !info.IsDir() {
			t.Fatalf("默认文件夹 %s 未创建：%v", conversationID, err)
		}
	}
}

// TestWorkerWaitsForToolchain 验证运行环境不可领取时不领取运行，可领取后照常领取。
func TestWorkerWaitsForToolchain(t *testing.T) {
	client := &stubRunClient{work: appservice.DeviceWork{
		Runs: []appservice.DeviceWorkRun{{RunID: "run-1", ConversationID: "conversation-1"}},
	}}
	worker := newTestWorker(t, client, stubRuntime{})
	pending := &stubToolchain{}
	worker.toolchain = pending

	worker.poll()
	worker.runs.Wait()
	if len(client.claims) != 0 || pending.checks != 1 {
		t.Fatalf("领取 = %v，检查次数 = %d", client.claims, pending.checks)
	}
	pending.ready = true
	worker.poll()
	worker.runs.Wait()
	if len(client.claims) != 1 {
		t.Fatalf("运行环境就绪后未领取：%v", client.claims)
	}
}

// TestWorkerStopsEndedRun 验证续租得知运行已结束后中断本机执行，且不上报失败。
func TestWorkerStopsEndedRun(t *testing.T) {
	client := &stubRunClient{
		work:      appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1", ConversationID: "conversation-1"}}},
		blockPeek: true,
		leaseEnd:  true,
		peeked:    make(chan string, 1),
	}
	worker := newTestWorker(t, client, stubRuntime{})

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
	client := &stubRunClient{work: appservice.DeviceWork{Runs: []appservice.DeviceWorkRun{{RunID: "run-1", ConversationID: "conversation-1"}}}}
	worker := newTestWorker(t, client, stubRuntime{failure: errors.New("model unavailable")})

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
	worker = newTestWorker(t, client, stubRuntime{inspect: func(ctx context.Context, request agentruntime.RunRequest) {
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
	worker := newTestWorker(t, client, stubRuntime{inspect: func(_ context.Context, request agentruntime.RunRequest) {
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
	worker := newTestWorker(t, &stubRunClient{}, stubRuntime{})
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
