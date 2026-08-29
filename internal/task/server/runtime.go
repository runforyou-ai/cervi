//go:build server

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/uptrace/bun"
)

var (
	_ Enqueuer   = (*Runtime)(nil)
	_ TxEnqueuer = (*Runtime)(nil)
)

// Runtime 运行服务端异步 Action、定时计划和可靠消息投递。
type Runtime struct {
	config     runtimeConfig
	repository *repository
	registry   *Registry
	schedules  []ScheduleDefinition
	instanceID string

	cancel         context.CancelFunc
	connection     *nats.Conn
	jetstream      jetstream.JetStream
	consumer       jetstream.Consumer
	consumeContext jetstream.ConsumeContext
	waitGroup      sync.WaitGroup
}

// New 创建服务端任务运行时。
func New(db *bun.DB, natsConfig serverconfig.NATSConfig) *Runtime {
	return &Runtime{
		config: newConfig(natsConfig), repository: newRepository(db), registry: NewRegistry(),
		instanceID: uuid.NewString(),
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
	return r.repository.enqueue(ctx, actionName, encoded, normalized)
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
	if err := r.startConsumer(ctx); err != nil {
		cancel()
		r.connection.Close()
		return err
	}
	r.waitGroup.Add(3)
	go r.runOutbox(ctx)
	go r.runExpiringMessageRecovery(ctx)
	go r.runScheduler(ctx)
	slog.Info("服务端任务运行时已启动",
		"namespace", r.config.Namespace,
		"stream", r.config.streamName(),
		"consumer", r.config.consumerName(),
		"workers", r.config.Workers,
		"max_ack_pending", r.config.MaxAckPending,
		"schedules", len(r.schedules),
	)
	return nil
}

// Stop 停止消费和调度，并关闭 NATS 连接。
func (r *Runtime) Stop() error {
	if r.cancel == nil {
		return nil
	}
	if r.consumeContext != nil {
		r.consumeContext.Stop()
	}
	r.cancel()
	r.waitGroup.Wait()
	if r.connection != nil {
		if err := r.connection.Drain(); err != nil {
			r.connection.Close()
			return fmt.Errorf("drain NATS connection: %w", err)
		}
		r.connection.Close()
	}
	slog.Info("服务端任务运行时已停止", "namespace", r.config.Namespace)
	return nil
}
