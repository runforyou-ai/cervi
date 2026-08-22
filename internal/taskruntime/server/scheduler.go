//go:build server

package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/taskruntime"
	"github.com/uptrace/bun"
)

const schedulerPollInterval = time.Second

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// syncSchedules 校验并同步由代码注册的服务端定时计划。
func (r *Runtime) syncSchedules(ctx context.Context) error {
	seen := make(map[string]struct{}, len(r.schedules))
	for _, definition := range r.schedules {
		if _, exists := seen[definition.Key]; exists {
			return fmt.Errorf("task schedule %q registered more than once", definition.Key)
		}
		seen[definition.Key] = struct{}{}
		if err := r.syncSchedule(ctx, definition); err != nil {
			return err
		}
	}
	return nil
}

// syncSchedule 同步一个代码定义的定时计划。
func (r *Runtime) syncSchedule(ctx context.Context, definition taskruntime.ScheduleDefinition) error {
	definition.Key = strings.TrimSpace(definition.Key)
	definition.ActionName = strings.TrimSpace(definition.ActionName)
	definition.Timezone = strings.TrimSpace(definition.Timezone)
	if !actionNamePattern.MatchString(definition.Key) {
		return fmt.Errorf("invalid task schedule key %q", definition.Key)
	}
	if _, exists := r.registry.lookup(definition.ActionName); !exists {
		return fmt.Errorf("task schedule %q references unregistered action %q", definition.Key, definition.ActionName)
	}
	options, err := normalizeEnqueueOptions(taskruntime.EnqueueOptions{
		Queue: definition.Queue, MaxAttempts: definition.MaxAttempts, TriggerType: taskruntime.TriggerSchedule,
	})
	if err != nil {
		return fmt.Errorf("invalid task schedule %q: %w", definition.Key, err)
	}
	definition.Queue = options.Queue
	definition.MaxAttempts = options.MaxAttempts
	if definition.Timezone == "" {
		definition.Timezone = "UTC"
	}
	schedule, err := parseSchedule(definition.CronExpression, definition.Timezone)
	if err != nil {
		return fmt.Errorf("parse task schedule %q: %w", definition.Key, err)
	}
	payload, err := encodePayload(definition.Payload)
	if err != nil {
		return fmt.Errorf("encode task schedule %q: %w", definition.Key, err)
	}

	now := time.Now().UTC()
	var existing servermodels.TaskSchedule
	readErr := r.repository.db.NewSelect().Model(&existing).Where("ts.schedule_key = ?", definition.Key).Scan(ctx)
	if readErr != nil && !errors.Is(readErr, sql.ErrNoRows) {
		return fmt.Errorf("read task schedule %q: %w", definition.Key, readErr)
	}
	exists := readErr == nil
	nextRunAt := schedule.Next(now)
	if !exists && definition.StartImmediately && definition.Enabled {
		nextRunAt = now
	}
	if exists && scheduleDefinitionUnchanged(&existing, definition, payload) {
		nextRunAt = existing.NextRunAt
	}
	record := &servermodels.TaskSchedule{
		ID: uuid.NewString(), ScheduleKey: definition.Key, ActionName: definition.ActionName,
		QueueName: definition.Queue, Payload: payload, CronExpression: definition.CronExpression,
		Timezone: definition.Timezone, Enabled: definition.Enabled, MaxAttempts: definition.MaxAttempts,
		NextRunAt: nextRunAt, CreatedAt: now, UpdatedAt: now,
	}
	_, err = r.repository.db.NewRaw(`
		INSERT INTO task_schedules (
			id, schedule_key, action_name, queue_name, payload, cron_expression,
			timezone, enabled, max_attempts, next_run_at, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (schedule_key) DO UPDATE SET
			action_name = EXCLUDED.action_name,
			queue_name = EXCLUDED.queue_name,
			payload = EXCLUDED.payload,
			cron_expression = EXCLUDED.cron_expression,
			timezone = EXCLUDED.timezone,
			enabled = EXCLUDED.enabled,
			max_attempts = EXCLUDED.max_attempts,
			next_run_at = EXCLUDED.next_run_at,
			updated_at = EXCLUDED.updated_at
	`, record.ID, record.ScheduleKey, record.ActionName, record.QueueName, string(record.Payload),
		record.CronExpression, record.Timezone, record.Enabled, record.MaxAttempts,
		record.NextRunAt, record.CreatedAt, record.UpdatedAt).Exec(ctx)
	if err != nil {
		return fmt.Errorf("sync task schedule %q: %w", definition.Key, err)
	}
	return nil
}

// scheduleDefinitionUnchanged 判断是否应保留数据库中的下一执行时间。
func scheduleDefinitionUnchanged(existing *servermodels.TaskSchedule, definition taskruntime.ScheduleDefinition, payload json.RawMessage) bool {
	return existing.ActionName == definition.ActionName &&
		existing.QueueName == definition.Queue &&
		payloadsEqual(existing.Payload, payload) &&
		existing.CronExpression == definition.CronExpression &&
		existing.Timezone == definition.Timezone &&
		existing.Enabled == definition.Enabled &&
		existing.MaxAttempts == definition.MaxAttempts
}

// payloadsEqual 比较两个 JSON Action 输入。
func payloadsEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

// parseSchedule 使用指定时区解析五段 Cron 或描述符表达式。
func parseSchedule(expression, timezone string) (cron.Schedule, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, errors.New("cron expression is required")
	}
	return cronParser.Parse("CRON_TZ=" + location.String() + " " + expression)
}

// runScheduler 持续认领到期计划并创建异步任务。
func (r *Runtime) runScheduler(ctx context.Context) {
	defer r.waitGroup.Done()
	ticker := time.NewTicker(schedulerPollInterval)
	defer ticker.Stop()
	for {
		triggered, err := r.triggerOneSchedule(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Warn("触发定时任务失败", "namespace", r.config.Namespace, "error", err)
		}
		if triggered {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// triggerOneSchedule 在同一事务内推进计划并创建一次任务。
func (r *Runtime) triggerOneSchedule(ctx context.Context) (bool, error) {
	triggered := false
	err := r.repository.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		now := time.Now().UTC()
		var record servermodels.TaskSchedule
		err := tx.NewSelect().Model(&record).
			Where("ts.enabled = TRUE").
			Where("ts.next_run_at <= ?", now).
			OrderExpr("ts.next_run_at ASC, ts.schedule_key ASC").
			For("UPDATE SKIP LOCKED").
			Limit(1).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim due task schedule: %w", err)
		}
		schedule, err := parseSchedule(record.CronExpression, record.Timezone)
		if err != nil {
			return fmt.Errorf("parse stored task schedule %q: %w", record.ScheduleKey, err)
		}
		dueAt := record.NextRunAt.UTC()
		options := taskruntime.EnqueueOptions{
			Queue: record.QueueName, MaxAttempts: record.MaxAttempts,
			TriggerType:    taskruntime.TriggerSchedule,
			IdempotencyKey: record.ScheduleKey + ":" + dueAt.Format(time.RFC3339Nano),
		}
		if _, err := enqueueIn(ctx, tx, record.ActionName, record.Payload, options, record.ScheduleKey); err != nil {
			return err
		}
		nextRunAt := schedule.Next(now)
		if _, err := tx.NewRaw(`
			UPDATE task_schedules
			SET next_run_at = ?, last_enqueued_at = ?, updated_at = ?
			WHERE id = ?
		`, nextRunAt, dueAt, now, record.ID).Exec(ctx); err != nil {
			return fmt.Errorf("advance task schedule %q: %w", record.ScheduleKey, err)
		}
		triggered = true
		return nil
	})
	return triggered, err
}
