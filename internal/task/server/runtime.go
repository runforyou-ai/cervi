//go:build server

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"uuid"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/uptrace/bun"
)

var (
	_ Enqueuer   = (*Runtime)(nil)
	_ TxEnqueuer = (*Runtime)(nil)
)

// workerPoolRuntime 保存一个 Worker Pool 的 Broker 与消费状态。
type workerPoolRuntime struct {
	config         workerPoolConfig
	consumer       jetstream.Consumer
	consumeContext jetstream.ConsumeContext
}

// Runtime 运行服务端异步 Action、定时计划和可靠消息投递。
type Runtime struct {
	config     runtimeConfig
	repository *repository
	registry   *Registry
	schedules  []ScheduleDefinition
	instanceID string

	cancel      context.CancelFunc
	connection  *nats.Conn
	jetstream   jetstream.JetStream
	workerPools []workerPoolRuntime
	waitGroup   sync.WaitGroup
}

// New 创建服务端任务运行时。
func New(db *bun.DB, natsConfig serverconfig.NATSConfig) *Runtime {
	return &Runtime{
		config: newConfig(natsConfig),
		// 创建任务仓储。
		repository: &repository{db: db},
		registry:   NewRegistry(),
		instanceID: uuid.New().String(),
	}
}

// Registry 返回 Action 注册表。
func (r *Runtime) Registry() *Registry {
	return r.registry
}

// RegisterSchedule 注册一个由代码管理的定时 Action。
func (r *Runtime) RegisterSchedule(definition ScheduleDefinition) {
	r.schedules = append(r.schedules, definition)
}

// Enqueue 将 Action 输入持久化并等待可靠发布。
func (r *Runtime) Enqueue(ctx context.Context, actionName string, payload any, options EnqueueOptions) (string, error) {
	encoded, normalized, err := r.prepareEnqueue(actionName, payload, options)
	if err != nil {
		return "", err
	}
	// 在独立事务内创建任务运行记录和发件箱消息。
	var runID string
	err = r.repository.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var enqueueErr error
		runID, enqueueErr = enqueueIn(ctx, tx, actionName, encoded, normalized, "")
		return enqueueErr
	})
	return runID, err
}

// EnqueueIn 将 Action 输入加入调用方已经开启的业务事务。
func (r *Runtime) EnqueueIn(ctx context.Context, tx bun.IDB, actionName string, payload any, options EnqueueOptions) (string, error) {
	encoded, normalized, err := r.prepareEnqueue(actionName, payload, options)
	if err != nil {
		return "", err
	}
	return enqueueIn(ctx, tx, actionName, encoded, normalized, "")
}

// prepareEnqueue 校验并编码一次任务投递输入。
func (r *Runtime) prepareEnqueue(actionName string, payload any, options EnqueueOptions) (json.RawMessage, EnqueueOptions, error) {
	if _, exists := r.registry.lookup(actionName); !exists {
		return nil, options, fmt.Errorf("task action %q is not registered", actionName)
	}
	encoded, err := encodePayload(payload)
	if err != nil {
		return nil, options, err
	}
	normalized, err := normalizeEnqueueOptions(options)
	if err != nil {
		return nil, options, err
	}
	return encoded, normalized, nil
}

// Start 校验计划、连接 NATS 并启动服务端任务循环。
func (r *Runtime) Start(parent context.Context) error {
	startupCtx, startupCancel := context.WithTimeout(parent, r.config.StartupTimeout)
	defer startupCancel()
	if err := r.syncSchedules(startupCtx); err != nil {
		return err
	}
	if err := r.connectBroker(startupCtx); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	if err := r.startConsumers(ctx); err != nil {
		cancel()
		r.stopConsumers()
		r.waitGroup.Wait()
		r.connection.Close()
		r.resetBroker()
		return err
	}
	r.waitGroup.Add(3)
	go r.runOutbox(ctx)
	go r.runExpiringMessageRecovery(ctx)
	go r.runScheduler(ctx)
	slog.Info("服务端任务运行时已启动",
		"namespace", r.config.Namespace,
		"stream", r.config.streamName(),
		"standard_consumer", r.config.consumerName(workerPoolStandard),
		"standard_workers", r.workerCount(workerPoolStandard),
		"agent_consumer", r.config.consumerName(workerPoolAgent),
		"agent_workers", r.workerCount(workerPoolAgent),
		"schedules", len(r.schedules),
	)
	return nil
}

// Stop 停止消费和调度，并关闭 NATS 连接。
func (r *Runtime) Stop() error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()
	r.stopConsumers()
	r.waitGroup.Wait()
	if r.connection != nil {
		if err := r.connection.Drain(); err != nil {
			r.connection.Close()
			r.resetBroker()
			return fmt.Errorf("drain NATS connection: %w", err)
		}
		r.connection.Close()
	}
	r.resetBroker()
	slog.Info("服务端任务运行时已停止", "namespace", r.config.Namespace)
	return nil
}

// workerCount 返回指定 Worker Pool 的并发数。
func (r *Runtime) workerCount(pool string) int {
	for _, item := range r.config.WorkerPools {
		if item.Name == pool {
			return item.Workers
		}
	}
	return 0
}

// resetBroker 清理已经停止的 Broker 生命周期状态。
func (r *Runtime) resetBroker() {
	r.cancel = nil
	r.connection = nil
	r.jetstream = nil
	r.workerPools = nil
}
