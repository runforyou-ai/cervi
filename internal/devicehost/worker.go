//go:build !server && !ios && !android

package devicehost

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/localmcp"
	"github.com/runforyou-ai/cervi/internal/integration/localskill"
	"github.com/runforyou-ai/cervi/internal/integration/localworkspace"
	"github.com/runforyou-ai/cervi/internal/integration/webfetch"
	"github.com/runforyou-ai/cervi/internal/realtime/protocol"
)

const (
	// workPollInterval 是不依赖事件流、定期比较工作水位的间隔。
	workPollInterval = 30 * time.Second
	// workRetryInterval 是请求失败后重新检查的间隔。
	workRetryInterval = 5 * time.Second
	// workRequestTimeout 是单次运行期请求的时限。
	workRequestTimeout = 30 * time.Second
	// defaultLeaseRenewInterval 是服务端未给出有效续租间隔时使用的间隔。
	defaultLeaseRenewInterval = 10 * time.Second
	// streamIdleTimeout 是设备事件流未收到任何事件即断开重连的时限，服务端每 25 秒发送心跳。
	streamIdleTimeout = 60 * time.Second
	// streamMaxRetryInterval 是设备事件流连续建立失败后的最长重试间隔。
	streamMaxRetryInterval = time.Minute
	// streamMaxLineBytes 是设备事件流单行事件的读取上限。
	streamMaxLineBytes = 1 << 20
)

// RunClient 是设备执行循环使用的企业服务端调用。
type RunClient interface {
	appservice.DeviceRunBackend
	// OpenDeviceEventStream 以本机设备身份建立成员事件流，关闭返回值即结束事件流。
	OpenDeviceEventStream(context.Context, appservice.RequestMeta) (io.ReadCloser, error)
	// DeviceModelEndpoint 返回运行的模型代理入口与附加本机设备认证的传输层。
	DeviceModelEndpoint(context.Context, appservice.RequestMeta, string) (string, http.RoundTripper, error)
	// ReadDeviceRunAttachment 读取运行所属会话中指定附件消息的文件内容。
	ReadDeviceRunAttachment(ctx context.Context, meta appservice.RequestMeta, runID, messageID string) ([]byte, error)
}

// Toolchain 是本机 Agent 命令使用的运行环境。
type Toolchain interface {
	// Ensure 在后台准备运行环境，返回设备是否可以领取运行。
	Ensure() bool
	// Environment 返回 Agent 命令叠加的环境变量。
	Environment() localworkspace.Environment
	// Close 结束后台准备并等待其退出。
	Close()
}

// Worker 领取派发给本机设备的 Agent 运行并在本机执行，每个会话以默认文件夹为本机文件的相对路径起点。
type Worker struct {
	registrar *Registrar
	client    RunClient
	runtime   agentruntime.Runtime
	toolchain Toolchain
	// localMCP 是这台电脑上主人的助理共用的本地 MCP 服务配置。
	localMCP *localmcp.Store
	// skills 是这台电脑上主人的助理共用的技能目录。
	skills *localskill.Store
	// folders 是各会话默认文件夹的上级目录。
	folders string
	pages   *webfetch.Client

	ctx     context.Context
	cancel  context.CancelFunc
	wake    chan struct{}
	session chan struct{}
	loops   sync.WaitGroup
	runs    sync.WaitGroup

	mu sync.Mutex
	// active 按运行编号保存执行中运行的默认文件夹与立即续租信号。
	active map[string]*activeRun
}

// activeRun 是本机登记执行的一次运行；过程流在登记时创建，释放登记时结束。
// 设备只领取排队中的运行，领取后不再回到排队，同一运行在设备上只执行一次，尝试序号固定为 1。
type activeRun struct {
	folder   string // 运行所属会话的默认文件夹。
	renewNow chan struct{}
	streamID string
	stream   *agentruntime.StreamHub
}

// NewWorker 创建设备执行循环，folders 是各会话默认文件夹的上级目录；当前平台不注册本机设备时返回 nil。
func NewWorker(registrar *Registrar, client RunClient, runtime agentruntime.Runtime, runEnvironment Toolchain, localMCP *localmcp.Store, skills *localskill.Store, folders string) *Worker {
	if registrar == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		registrar: registrar,
		client:    client,
		runtime:   runtime,
		toolchain: runEnvironment,
		localMCP:  localMCP,
		skills:    skills,
		folders:   folders,
		pages:     webfetch.NewClient(),
		ctx:       ctx,
		cancel:    cancel,
		wake:      make(chan struct{}, 1),
		session:   make(chan struct{}, 1),
		active:    map[string]*activeRun{},
	}
}

// Start 开始准备运行环境，订阅登录凭据变化，开始领取循环与设备事件流。
func (w *Worker) Start() {
	if w == nil {
		return
	}
	w.toolchain.Ensure()
	w.registrar.sessions.Subscribe(func() {
		w.Wake()
		signal(w.session)
	})
	w.loops.Add(2)
	go w.loop()
	go w.listen()
}

// Stop 结束领取循环与设备事件流，取消本机执行中的运行与运行环境准备并等待其退出。
func (w *Worker) Stop() {
	if w == nil {
		return
	}
	w.cancel()
	w.loops.Wait()
	w.runs.Wait()
	w.toolchain.Close()
}

// Wake 请求立即比较一次工作水位。
func (w *Worker) Wake() {
	if w == nil {
		return
	}
	signal(w.wake)
}

// loop 在唤醒信号和定时器上比较工作水位并领取运行，直到执行循环停止。
func (w *Worker) loop() {
	defer w.loops.Done()
	for {
		wait := workPollInterval
		if w.poll() {
			wait = workRetryInterval
		}
		select {
		case <-w.ctx.Done():
			return
		case <-w.wake:
		case <-time.After(wait):
		}
	}
}

// poll 读取待领取运行并逐个领取，返回是否需要尽快重新检查；运行环境首次就绪前不领取（用户已卸载时照常领取），准备结束后经 Wake 重新检查。
func (w *Worker) poll() bool {
	ctx, cancel := context.WithTimeout(w.ctx, workRequestTimeout)
	defer cancel()
	session, found, err := w.registrar.currentDeviceSession(ctx, appservice.RequestMeta{})
	if err != nil {
		slog.Warn("读取本机设备注册状态失败", "error", err)
		return true
	}
	if !found {
		return false
	}
	meta := appservice.RequestMeta{DeviceID: session.deviceID}
	work, err := w.client.GetDeviceWork(ctx, meta)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("读取设备待领取运行失败", "device_id", session.deviceID, "error", err)
		}
		return true
	}
	if !w.toolchain.Ensure() {
		return false
	}
	retry := false
	for _, run := range work.Runs {
		if !w.reserve(run) {
			continue
		}
		if w.start(ctx, meta, run) {
			retry = true
		}
	}
	return retry
}

// reserve 在本机登记运行及其会话的默认文件夹，运行已在执行时返回 false。
func (w *Worker) reserve(run appservice.DeviceWorkRun) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active[run.RunID] != nil {
		return false
	}
	streamID := uuid.NewV7().String()
	w.active[run.RunID] = &activeRun{
		folder: filepath.Join(w.folders, run.ConversationID), renewNow: make(chan struct{}, 1), streamID: streamID,
		stream: agentruntime.NewStreamHub(agentruntime.StreamSnapshot{RunID: run.RunID, StreamID: streamID, Attempt: 1}),
	}
	return true
}

// release 移除本机对运行的登记并结束运行的过程流。
func (w *Worker) release(runID string) {
	w.mu.Lock()
	current := w.active[runID]
	if current != nil {
		delete(w.active, runID)
	}
	w.mu.Unlock()
	if current != nil {
		current.stream.End()
	}
}

// RunsLocally 判断运行是否已在本机登记执行。
func (w *Worker) RunsLocally(runID string) bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active[runID] != nil
}

// SubscribeLocalRunStream 订阅本机登记执行的运行的过程流，返回订阅时的快照与取消订阅函数，回调在运行流锁内执行；运行不在本机或过程流已结束时返回 false。
func (w *Worker) SubscribeLocalRunStream(runID string, onDelta func(agentruntime.StreamDelta), onEnd func()) (agentruntime.StreamSnapshot, func(), bool) {
	if w == nil {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	w.mu.Lock()
	current := w.active[runID]
	w.mu.Unlock()
	if current == nil {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	snapshot, subscription, ok := current.stream.Subscribe(onDelta, onEnd)
	if !ok {
		return agentruntime.StreamSnapshot{}, nil, false
	}
	return snapshot, subscription.Close, true
}

// start 领取运行，领取成功后启动执行与续租，返回是否需要尽快重新检查。
func (w *Worker) start(ctx context.Context, meta appservice.RequestMeta, run appservice.DeviceWorkRun) bool {
	claim, err := w.client.ClaimDeviceRun(ctx, meta, run.RunID)
	if err != nil {
		w.release(run.RunID)
		if ctx.Err() == nil {
			slog.Warn("领取设备运行失败", "agent_run_id", run.RunID, "error", err)
		}
		return errorReason(err) != "run_unavailable"
	}
	w.mu.Lock()
	local := w.active[run.RunID]
	w.mu.Unlock()
	runCtx, cancelRun := context.WithCancel(w.ctx)
	// 服务端给出的续租间隔无效时按默认间隔续租。
	interval := time.Duration(claim.LeaseRenewIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultLeaseRenewInterval
	}
	w.runs.Add(2)
	go w.renew(runCtx, cancelRun, meta, run.RunID, interval, local.renewNow)
	go w.execute(runCtx, cancelRun, meta, run.RunID, claim, local)
	slog.Info("设备运行已领取", "agent_run_id", run.RunID, "lease_expires_at", claim.LeaseExpiresAt)
	return false
}

// execute 在本机执行一次已领取的运行，出错时上报失败与已产生的过程内容。
func (w *Worker) execute(runCtx context.Context, cancelRun context.CancelFunc, meta appservice.RequestMeta, runID string, claim appservice.DeviceRunClaim, local *activeRun) {
	defer w.runs.Done()
	defer w.Wake()
	defer w.release(runID)
	defer cancelRun()
	result, err := w.runAgent(runCtx, meta, runID, claim, local)
	// 本机执行循环停止时不再上报；运行已在服务端结束或失效且没有过程内容时无需上报。
	ended := runCtx.Err() != nil || errors.Is(err, errRunSuppressed)
	if err == nil || w.ctx.Err() != nil || (ended && len(result.Blocks) == 0) {
		return
	}
	if !ended {
		slog.Warn("设备运行执行失败", "agent_run_id", runID, "error", err)
	}
	input := appservice.DeviceRunFailureInput{ErrorCode: appservice.DeviceRunFailureRuntimeFailed, Message: err.Error()}
	if input.Usage, input.Blocks, input.Plan, err = encodeProcess(result); err != nil {
		slog.Warn("编码设备运行过程内容失败", "agent_run_id", runID, "error", err)
	}
	ctx, cancel := context.WithTimeout(w.ctx, workRequestTimeout)
	defer cancel()
	if failErr := w.client.FailDeviceRun(ctx, meta, runID, input); failErr != nil {
		slog.Warn("上报设备运行失败未成功", "agent_run_id", runID, "error", failErr)
	}
}

// renew 按间隔续租，运行已结束或租约失效时取消本机执行；收到立即续租信号时提前续租。
func (w *Worker) renew(runCtx context.Context, cancelRun context.CancelFunc, meta appservice.RequestMeta, runID string, interval time.Duration, renewNow <-chan struct{}) {
	defer w.runs.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return
		case <-ticker.C:
		case <-renewNow:
		}
		ctx, cancel := context.WithTimeout(runCtx, workRequestTimeout)
		lease, err := w.client.RenewDeviceRunLease(ctx, meta, runID)
		cancel()
		if runCtx.Err() != nil {
			return
		}
		if err != nil {
			if apiError, ok := errors.AsType[*appservice.Error](err); ok && (apiError.Reason == "lease_lost" || apiError.Kind == appservice.ErrorKindNotFound) {
				slog.Info("设备运行租约已失效，停止本机执行", "agent_run_id", runID)
				cancelRun()
				return
			}
			slog.Warn("设备运行续租失败", "agent_run_id", runID, "error", err)
			continue
		}
		if lease.Ended {
			slog.Info("设备运行已结束，停止本机执行", "agent_run_id", runID)
			cancelRun()
			return
		}
	}
}

// nudgeLeases 让本机执行中的全部运行立即续租，据此尽快得知停止。
func (w *Worker) nudgeLeases() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, run := range w.active {
		signal(run.renewNow)
	}
}

// listen 维持本机设备事件流，收到工作水位推进或重新连接时唤醒领取循环并立即续租。
func (w *Worker) listen() {
	defer w.loops.Done()
	backoff := workRetryInterval
	for {
		connected, registered := w.stream()
		wait := workRetryInterval
		switch {
		case connected:
			backoff = workRetryInterval
		case registered:
			// 连接失败按退避重连，连续失败时逐步拉长间隔。
			wait = backoff
			backoff = min(backoff*2, streamMaxRetryInterval)
		}
		select {
		case <-w.ctx.Done():
			return
		case <-w.session:
		case <-time.After(wait):
		}
	}
}

// stream 建立一次设备事件流并读取到结束，返回是否曾连接成功以及本机是否已注册设备。
func (w *Worker) stream() (bool, bool) {
	ctx, cancel := context.WithTimeout(w.ctx, workRequestTimeout)
	session, found, err := w.registrar.currentDeviceSession(ctx, appservice.RequestMeta{})
	cancel()
	if err != nil || !found {
		return false, false
	}
	body, err := w.client.OpenDeviceEventStream(w.ctx, appservice.RequestMeta{DeviceID: session.deviceID})
	if err != nil {
		if w.ctx.Err() == nil {
			slog.Warn("建立设备事件流失败", "device_id", session.deviceID, "error", err)
		}
		return false, true
	}
	defer body.Close()
	// 空闲超时、登录会话变化或执行循环停止时关闭事件流，读取随之结束。
	idle := time.AfterFunc(streamIdleTimeout, func() { body.Close() })
	defer idle.Stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-w.ctx.Done():
				body.Close()
				return
			case <-w.session:
				current, found, err := w.registrar.currentDeviceSession(w.ctx, appservice.RequestMeta{})
				if err != nil || !found || current.key() != session.key() {
					body.Close()
					signal(w.session)
					return
				}
			}
		}
	}()
	slog.Info("设备事件流已建立", "device_id", session.deviceID)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), streamMaxLineBytes)
	for scanner.Scan() {
		idle.Reset(streamIdleTimeout)
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		frame, err := protocol.Decode([]byte(data))
		if err != nil {
			continue
		}
		switch value := frame.(type) {
		case protocol.ServerHello:
			// 重新连接期间可能错过工作水位通知，连接建立后立即比较一次。
			w.Wake()
		case protocol.DeviceWorkAdvanced:
			if value.DeviceID == session.deviceID {
				w.Wake()
				w.nudgeLeases()
			}
		}
	}
	slog.Info("设备事件流已结束", "device_id", session.deviceID)
	return true, true
}

// errorReason 返回业务错误的稳定原因码，其他错误返回空串。
func errorReason(err error) string {
	if apiError, ok := errors.AsType[*appservice.Error](err); ok {
		return apiError.Reason
	}
	return ""
}

// signal 向容量为 1 的通道投递一次信号，已有待处理信号时直接返回。
func signal(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}
