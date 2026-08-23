//go:build server

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	statusQueued    = "queued"
	statusPublished = "published"
	statusRunning   = "running"
	statusRetrying  = "retrying"
	statusSucceeded = "succeeded"
	statusFailed    = "failed"

	defaultQueue       = "default"
	defaultMaxAttempts = 5
	leaseDuration      = 2 * time.Minute
	outboxLease        = 30 * time.Second
)

var (
	actionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
	queueNamePattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// repository 持久化任务、计划和发件箱状态。
type repository struct {
	db *bun.DB
}

// newRepository 创建任务仓储。
func newRepository(db *bun.DB) *repository {
	return &repository{db: db}
}

// Enqueue 在同一事务内创建任务运行记录和发件箱消息。
func (r *repository) Enqueue(ctx context.Context, actionName string, payload any, options EnqueueOptions) (string, error) {
	encoded, err := encodePayload(payload)
	if err != nil {
		return "", err
	}
	options, err = normalizeEnqueueOptions(options)
	if err != nil {
		return "", err
	}
	if !actionNamePattern.MatchString(actionName) {
		return "", fmt.Errorf("invalid task action name %q", actionName)
	}
	var runID string
	err = r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var enqueueErr error
		runID, enqueueErr = enqueueIn(ctx, tx, actionName, encoded, options, "")
		return enqueueErr
	})
	return runID, err
}

// enqueueIn 在已有事务内创建任务运行记录和发件箱消息。
func enqueueIn(ctx context.Context, db bun.IDB, actionName string, payload json.RawMessage, options EnqueueOptions, scheduleKey string) (string, error) {
	now := time.Now().UTC()
	availableAt := options.AvailableAt.UTC()
	if options.AvailableAt.IsZero() {
		availableAt = now
	}
	runID := uuid.NewString()
	var insertedID string
	err := db.NewRaw(`
		INSERT INTO task_runs (
			id, action_name, queue_name, payload, trigger_type, schedule_key,
			status, max_attempts, available_at, idempotency_key, created_at, updated_at
		)
		VALUES (?, ?, ?, ?::jsonb, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?)
		ON CONFLICT DO NOTHING
		RETURNING id
	`, runID, actionName, options.Queue, string(payload), options.TriggerType, scheduleKey,
		statusQueued, options.MaxAttempts, availableAt, options.IdempotencyKey, now, now).Scan(ctx, &insertedID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("insert task run: %w", err)
	}
	if insertedID == "" {
		if options.IdempotencyKey == "" {
			return "", errors.New("insert task run returned no record")
		}
		if scheduleKey != "" {
			err = db.NewRaw(`
				SELECT id
				FROM task_runs
				WHERE schedule_key = ? AND idempotency_key = ?
				LIMIT 1
			`, scheduleKey, options.IdempotencyKey).Scan(ctx, &insertedID)
		} else {
			err = db.NewRaw(`
				SELECT id
				FROM task_runs
				WHERE action_name = ?
					AND idempotency_key = ?
					AND status IN (?, ?, ?, ?)
				ORDER BY created_at DESC
				LIMIT 1
			`, actionName, options.IdempotencyKey, statusQueued, statusPublished, statusRunning, statusRetrying).Scan(ctx, &insertedID)
		}
		if err != nil {
			return "", fmt.Errorf("find idempotent task run: %w", err)
		}
		return insertedID, nil
	}
	outbox := &servermodels.TaskOutbox{
		TaskRunID: insertedID, MessageID: uuid.NewString(), QueueName: options.Queue,
		AvailableAt: availableAt, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := db.NewInsert().Model(outbox).Exec(ctx); err != nil {
		return "", fmt.Errorf("insert task outbox: %w", err)
	}
	return insertedID, nil
}

// claimOutbox 认领一条待发布消息并设置短租约。
func (r *repository) claimOutbox(ctx context.Context) (*servermodels.TaskOutbox, error) {
	now := time.Now().UTC()
	var record servermodels.TaskOutbox
	err := r.db.NewRaw(`
		WITH candidate AS (
			SELECT task_run_id
			FROM task_outbox
			WHERE available_at <= ?
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE task_outbox AS target
		SET attempts = target.attempts + 1,
			available_at = ?,
			updated_at = ?
		FROM candidate
		WHERE target.task_run_id = candidate.task_run_id
		RETURNING target.*
	`, now, now.Add(outboxLease), now).Scan(ctx, &record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim task outbox: %w", err)
	}
	return &record, nil
}

// markPublished 提交消息发布结果并删除发件箱记录。
func (r *repository) markPublished(ctx context.Context, runID, messageID string) error {
	now := time.Now().UTC()
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*servermodels.TaskOutbox)(nil)).
			Where("task_run_id = ?", runID).
			Where("message_id = ?", messageID).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete task outbox: %w", err)
		}
		if _, err := tx.NewRaw(`
			UPDATE task_runs
			SET status = CASE WHEN status = ? THEN ? ELSE status END,
				published_at = ?,
				updated_at = ?
			WHERE id = ?
		`, statusQueued, statusPublished, now, now, runID).Exec(ctx); err != nil {
			return fmt.Errorf("mark task published: %w", err)
		}
		return nil
	})
}

// releaseOutbox 在发布失败后安排指数退避重试。
func (r *repository) releaseOutbox(ctx context.Context, record *servermodels.TaskOutbox, publishErr error) error {
	now := time.Now().UTC()
	delay := retryDelay(record.Attempts)
	message := truncateError(publishErr)
	_, err := r.db.NewRaw(`
		UPDATE task_outbox
		SET available_at = ?, last_error = ?, updated_at = ?
		WHERE task_run_id = ? AND message_id = ?
	`, now.Add(delay), message, now, record.TaskRunID, record.MessageID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("release task outbox: %w", err)
	}
	return nil
}

// claimRun 以数据库租约认领任务，确保重复消息不会并发执行同一次运行。
func (r *repository) claimRun(ctx context.Context, runID, workerID string) (*servermodels.TaskRun, error) {
	now := time.Now().UTC()
	var record servermodels.TaskRun
	err := r.db.NewRaw(`
		UPDATE task_runs
		SET status = ?,
			attempt = attempt + 1,
			lease_expires_at = ?,
			worker_id = ?,
			started_at = COALESCE(started_at, ?),
			updated_at = ?
		WHERE id = ?
			AND attempt < max_attempts
			AND available_at <= ?
			AND (
				status IN (?, ?, ?)
				OR (status = ? AND lease_expires_at <= ?)
			)
		RETURNING *
	`, statusRunning, now.Add(leaseDuration), workerID, now, now, runID, now,
		statusQueued, statusPublished, statusRetrying, statusRunning, now).Scan(ctx, &record)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim task run: %w", err)
	}
	return &record, nil
}

// getRun 读取任务当前状态。
func (r *repository) getRun(ctx context.Context, runID string) (*servermodels.TaskRun, error) {
	var record servermodels.TaskRun
	err := r.db.NewSelect().Model(&record).Where("tr.id = ?", runID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task run: %w", err)
	}
	return &record, nil
}

// extendLease 延长正在运行任务的数据库租约并返回是否仍持有租约。
func (r *repository) extendLease(ctx context.Context, runID, workerID string) (bool, error) {
	now := time.Now().UTC()
	result, err := r.db.NewRaw(`
		UPDATE task_runs
		SET lease_expires_at = ?, updated_at = ?
		WHERE id = ? AND status = ? AND worker_id = ?
	`, now.Add(leaseDuration), now, runID, statusRunning, workerID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("extend task lease: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read extended task lease count: %w", err)
	}
	return count == 1, nil
}

// recoverExpiringMessages 为接近 JetStream 保留期限的非终态任务重建发件箱消息。
func (r *repository) recoverExpiringMessages(ctx context.Context, publishedBefore time.Time, limit int) (int64, error) {
	now := time.Now().UTC()
	result, err := r.db.NewRaw(`
		WITH candidates AS (
			SELECT tr.id, tr.queue_name, tr.available_at
			FROM task_runs AS tr
			WHERE tr.published_at <= ?
				AND (
					tr.status IN (?, ?)
					OR (tr.status = ? AND tr.lease_expires_at <= ?)
				)
				AND NOT EXISTS (
					SELECT 1
					FROM task_outbox AS task_outbox
					WHERE task_outbox.task_run_id = tr.id
				)
			ORDER BY tr.published_at, tr.created_at
			LIMIT ?
		)
		INSERT INTO task_outbox (
			task_run_id, message_id, queue_name, attempts, available_at, created_at, updated_at
		)
		SELECT id, uuidv7(), queue_name, 0, GREATEST(available_at, ?), ?, ?
		FROM candidates
		ON CONFLICT (task_run_id) DO NOTHING
	`, publishedBefore, statusPublished, statusRetrying, statusRunning, now, limit, now, now, now).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("recover expiring task messages: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read recovered expiring task message count: %w", err)
	}
	return count, nil
}

// completeRun 将任务标记为执行成功。
func (r *repository) completeRun(ctx context.Context, runID, workerID string) error {
	now := time.Now().UTC()
	result, err := r.db.NewRaw(`
		UPDATE task_runs
		SET status = ?, completed_at = ?, lease_expires_at = NULL,
			worker_id = NULL, last_error = NULL, updated_at = ?
		WHERE id = ? AND status = ? AND worker_id = ?
	`, statusSucceeded, now, now, runID, statusRunning, workerID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("complete task run: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return errors.New("task run lease lost before completion")
	}
	return nil
}

// failRun 记录失败，并在需要重试时原子创建下一次发件箱消息。
func (r *repository) failRun(ctx context.Context, run *servermodels.TaskRun, workerID string, runErr error, permanent bool) (bool, error) {
	now := time.Now().UTC()
	retry := !permanent && run.Attempt < run.MaxAttempts
	delay := retryDelay(run.Attempt)
	nextStatus := statusFailed
	var completedAt any = now
	availableAt := now
	if retry {
		nextStatus = statusRetrying
		completedAt = nil
		availableAt = now.Add(delay)
	}
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.NewRaw(`
			UPDATE task_runs
			SET status = ?, available_at = ?, completed_at = ?, last_error = ?,
				lease_expires_at = NULL, worker_id = NULL, updated_at = ?
			WHERE id = ? AND status = ? AND worker_id = ?
		`, nextStatus, availableAt, completedAt, truncateError(runErr), now, run.ID, statusRunning, workerID).Exec(ctx)
		if err != nil {
			return fmt.Errorf("fail task run: %w", err)
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return errors.New("task run lease lost before failure update")
		}
		if !retry {
			return nil
		}
		if _, err := tx.NewRaw(`
			INSERT INTO task_outbox (
				task_run_id, message_id, queue_name, attempts, available_at, created_at, updated_at
			)
			VALUES (?, uuidv7(), ?, 0, ?, ?, ?)
			ON CONFLICT (task_run_id) DO UPDATE SET
				message_id = EXCLUDED.message_id,
				queue_name = EXCLUDED.queue_name,
				attempts = 0,
				available_at = EXCLUDED.available_at,
				last_error = NULL,
				updated_at = EXCLUDED.updated_at
		`, run.ID, run.QueueName, availableAt, now, now).Exec(ctx); err != nil {
			return fmt.Errorf("enqueue task retry: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return retry, nil
}

// failExhaustedRun 终结已经耗尽尝试次数且租约过期的任务。
func (r *repository) failExhaustedRun(ctx context.Context, runID string) (bool, error) {
	now := time.Now().UTC()
	result, err := r.db.NewRaw(`
		UPDATE task_runs
		SET status = ?, completed_at = ?, lease_expires_at = NULL,
			worker_id = NULL, last_error = ?, updated_at = ?
		WHERE id = ?
			AND status = ?
			AND attempt >= max_attempts
			AND lease_expires_at <= ?
	`, statusFailed, now, "task worker lease expired after final attempt", now,
		runID, statusRunning, now).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("fail exhausted task run: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read exhausted task run count: %w", err)
	}
	return count == 1, nil
}

// normalizeEnqueueOptions 补齐并校验任务投递参数。
func normalizeEnqueueOptions(options EnqueueOptions) (EnqueueOptions, error) {
	options.Queue = strings.TrimSpace(options.Queue)
	if options.Queue == "" {
		options.Queue = defaultQueue
	}
	if !queueNamePattern.MatchString(options.Queue) {
		return options, fmt.Errorf("invalid task queue name %q", options.Queue)
	}
	if options.MaxAttempts == 0 {
		options.MaxAttempts = defaultMaxAttempts
	}
	if options.MaxAttempts < 1 {
		return options, errors.New("task max attempts must be positive")
	}
	options.TriggerType = strings.TrimSpace(options.TriggerType)
	if options.TriggerType == "" {
		options.TriggerType = TriggerBusiness
	}
	switch options.TriggerType {
	case TriggerBusiness, TriggerManual, TriggerSchedule:
	default:
		return options, fmt.Errorf("invalid task trigger type %q", options.TriggerType)
	}
	options.IdempotencyKey = strings.TrimSpace(options.IdempotencyKey)
	return options, nil
}

// encodePayload 将 Action 输入序列化为 JSON。
func encodePayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		return json.RawMessage(`{}`), nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode task payload: %w", err)
	}
	return encoded, nil
}

// retryDelay 返回有上限的指数退避时间。
func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 15 * time.Second * time.Duration(1<<min(attempt-1, 8))
	return min(delay, time.Hour)
}

// truncateError 限制持久化错误长度。
func truncateError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 4000 {
		return message[:4000]
	}
	return message
}
