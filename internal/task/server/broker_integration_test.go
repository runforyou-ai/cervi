//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
)

// TestWorkerPoolsIsolateAgentTasks 验证 Agent 阻塞不会占用标准任务的 Worker 配额。
func TestWorkerPoolsIsolateAgentTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := serverstorage.Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	natsConfig := testNATSConfig(t)
	config := newConfig(natsConfig)
	createLegacyTaskConsumer(t, ctx, config)

	runtime := New(store.DB(), natsConfig)
	agentProbe := newWorkerPoolProbe(3)
	standardProbe := newWorkerPoolProbe(5)
	agentAction := "test.pool.agent." + uuid.NewString()
	standardAction := "test.pool.standard." + uuid.NewString()
	if err := runtime.Registry().Register(agentAction, agentProbe.handle); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Registry().Register(standardAction, standardProbe.handle); err != nil {
		t.Fatal(err)
	}
	tasks := make([]testBrokerTask, 0, 8)
	for range 3 {
		runID, enqueueErr := runtime.Enqueue(ctx, agentAction, struct{}{}, EnqueueOptions{Queue: QueueAgent, MaxAttempts: 1})
		if enqueueErr != nil {
			t.Fatal(enqueueErr)
		}
		cleanupEnqueuedTask(t, store.DB(), runID)
		tasks = append(tasks, testBrokerTask{runID: runID, queue: QueueAgent})
	}
	for range 5 {
		runID, enqueueErr := runtime.Enqueue(ctx, standardAction, struct{}{}, EnqueueOptions{Queue: "future_queue", MaxAttempts: 1})
		if enqueueErr != nil {
			t.Fatal(enqueueErr)
		}
		cleanupEnqueuedTask(t, store.DB(), runID)
		tasks = append(tasks, testBrokerTask{runID: runID, queue: "future_queue"})
	}

	runtimeStopped := false
	t.Cleanup(func() {
		if !runtimeStopped {
			_ = runtime.Stop()
		}
	})
	if err := startTestConsumers(ctx, runtime); err != nil {
		t.Fatal(err)
	}
	assertWorkerConsumers(t, ctx, runtime)
	for _, task := range tasks {
		if err := publishTestTask(ctx, runtime, task); err != nil {
			t.Fatal(err)
		}
	}

	waitForSignals(t, agentProbe.started, agentTaskWorkers, "Agent 任务启动")
	waitForSignals(t, standardProbe.started, standardTaskWorkers, "标准任务启动")
	assertNoSignal(t, agentProbe.started, "Agent Worker 超出配额")
	assertNoSignal(t, standardProbe.started, "标准 Worker 超出配额")

	close(standardProbe.release)
	waitForSignals(t, standardProbe.completed, standardTaskWorkers, "已启动的标准任务完成")
	assertNoSignal(t, agentProbe.started, "标准任务释放后 Agent Worker 超出配额")

	close(agentProbe.release)
	waitForSignals(t, agentProbe.completed, agentTaskWorkers, "已启动的 Agent 任务完成")
	if err := runtime.Stop(); err != nil {
		t.Fatal(err)
	}
	runtimeStopped = true

	restarted := New(store.DB(), natsConfig)
	restartStopped := false
	t.Cleanup(func() {
		if !restartStopped {
			_ = restarted.Stop()
		}
	})
	if err := startTestConsumers(ctx, restarted); err != nil {
		t.Fatalf("使用现有 Worker Consumer 重启失败: %v", err)
	}
	if err := restarted.Stop(); err != nil {
		t.Fatal(err)
	}
	restartStopped = true
}

type testBrokerTask struct {
	runID string
	queue string
}

type workerPoolProbe struct {
	started   chan struct{}
	release   chan struct{}
	completed chan struct{}
}

// newWorkerPoolProbe 创建阻塞指定数量任务的并发探针。
func newWorkerPoolProbe(tasks int) *workerPoolProbe {
	return &workerPoolProbe{
		started:   make(chan struct{}, tasks),
		release:   make(chan struct{}),
		completed: make(chan struct{}, tasks),
	}
}

// handle 在测试释放前保持任务运行。
func (p *workerPoolProbe) handle(ctx context.Context, _ json.RawMessage) error {
	select {
	case p.started <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	p.completed <- struct{}{}
	return nil
}

// startTestConsumers 只启动当前测试需要的 Broker 和 Worker，不扫描共享任务表。
func startTestConsumers(parent context.Context, runtime *Runtime) error {
	if err := runtime.connectBroker(parent); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	runtime.cancel = cancel
	if err := runtime.startConsumers(ctx); err != nil {
		cancel()
		runtime.stopConsumers()
		runtime.waitGroup.Wait()
		runtime.connection.Close()
		runtime.resetBroker()
		return err
	}
	return nil
}

// publishTestTask 直接发布本测试创建的 Run，避免领取共享 Outbox。
func publishTestTask(ctx context.Context, runtime *Runtime, task testBrokerTask) error {
	payload, err := json.Marshal(taskMessage{RunID: task.runID})
	if err != nil {
		return err
	}
	_, err = runtime.jetstream.Publish(
		ctx,
		runtime.config.taskSubject(task.queue),
		payload,
		jetstream.WithMsgID(uuid.NewString()),
	)
	return err
}

// testNATSConfig 创建不会与其他测试共享 JetStream 资源的 NATS 配置。
func testNATSConfig(t *testing.T) serverconfig.NATSConfig {
	t.Helper()
	url := os.Getenv("TEST_NATS_URL")
	if url == "" {
		t.Skip("TEST_NATS_URL is not set")
	}
	namespace := "test_tasks_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	return serverconfig.NATSConfig{URL: url, Namespace: namespace}
}

// createLegacyTaskConsumer 创建旧版 Catch-all Consumer 并在测试后删除 Stream。
func createLegacyTaskConsumer(t *testing.T, ctx context.Context, config runtimeConfig) {
	t.Helper()
	connection, err := nats.Connect(config.URL, nats.Timeout(config.StartupTimeout))
	if err != nil {
		t.Fatal(err)
	}
	js, err := jetstream.New(connection)
	if err != nil {
		connection.Close()
		t.Fatal(err)
	}
	if _, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: config.streamName(), Subjects: []string{config.subjectPrefix() + ".>"},
		Retention: jetstream.WorkQueuePolicy, MaxBytes: config.MaxBytes, MaxAge: config.MaxAge,
		Discard: jetstream.DiscardNew, Storage: jetstream.FileStorage, Replicas: config.Replicas,
		Duplicates: taskMessageDuplicateWindow,
	}); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	if _, err := js.CreateOrUpdateConsumer(ctx, config.streamName(), jetstream.ConsumerConfig{
		Name: config.legacyConsumerName(), Durable: config.legacyConsumerName(),
		AckPolicy: jetstream.AckExplicitPolicy, AckWait: leaseDuration, MaxDeliver: -1,
		FilterSubject: config.subjectPrefix() + ".>", MaxAckPending: taskPoolMaxAckPending, Replicas: config.Replicas,
	}); err != nil {
		connection.Close()
		t.Fatal(err)
	}
	connection.Close()
	t.Cleanup(func() {
		cleanupConnection, connectErr := nats.Connect(config.URL, nats.Timeout(config.StartupTimeout))
		if connectErr != nil {
			t.Errorf("连接 NATS 清理测试 Stream 失败: %v", connectErr)
			return
		}
		defer cleanupConnection.Close()
		cleanupJS, jsErr := jetstream.New(cleanupConnection)
		if jsErr != nil {
			t.Errorf("创建 JetStream 客户端清理测试 Stream 失败: %v", jsErr)
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), taskBrokerOperationTimeout)
		defer cleanupCancel()
		if deleteErr := cleanupJS.DeleteStream(cleanupCtx, config.streamName()); deleteErr != nil {
			t.Errorf("删除测试 Stream 失败: %v", deleteErr)
		}
	})
}

// assertWorkerConsumers 验证旧 Consumer 已被两个互不重叠的 Worker Pool 替代。
func assertWorkerConsumers(t *testing.T, ctx context.Context, runtime *Runtime) {
	t.Helper()
	if _, err := runtime.jetstream.Consumer(ctx, runtime.config.streamName(), runtime.config.legacyConsumerName()); !errors.Is(err, jetstream.ErrConsumerNotFound) {
		t.Fatalf("旧任务 Consumer 仍然存在: %v", err)
	}
	for _, pool := range runtime.config.WorkerPools {
		consumer, err := runtime.jetstream.Consumer(ctx, runtime.config.streamName(), runtime.config.consumerName(pool.Name))
		if err != nil {
			t.Fatalf("读取 %s Consumer 失败: %v", pool.Name, err)
		}
		if filter := consumer.CachedInfo().Config.FilterSubject; filter != runtime.config.filterSubject(pool.Name) {
			t.Fatalf("%s Consumer Filter = %q", pool.Name, filter)
		}
	}
}

// waitForSignals 等待指定数量的任务事件。
func waitForSignals(t *testing.T, signals <-chan struct{}, count int, description string) {
	t.Helper()
	for range count {
		select {
		case <-signals:
		case <-time.After(10 * time.Second):
			t.Fatalf("等待%s超时", description)
		}
	}
}

// assertNoSignal 验证当前 Worker 配额内没有额外任务开始。
func assertNoSignal(t *testing.T, signals <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signals:
		t.Fatal(description)
	case <-time.After(200 * time.Millisecond):
	}
}
