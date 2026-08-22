// Package taskruntime 定义各平台异步任务运行时共享的最小契约。
package taskruntime

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const (
	TriggerBusiness = "business"
	TriggerManual   = "manual"
	TriggerSchedule = "schedule"
)

// EnqueueOptions 定义一次异步 Action 投递参数。
type EnqueueOptions struct {
	Queue          string
	MaxAttempts    int
	IdempotencyKey string
	TriggerType    string
	AvailableAt    time.Time
}

// Enqueuer 将一个 Action 输入提交给当前平台的任务运行时。
type Enqueuer interface {
	Enqueue(ctx context.Context, actionName string, payload any, options EnqueueOptions) (string, error)
}

// Handler 执行已经序列化的 Action 输入。
type Handler func(context.Context, json.RawMessage) error

// PermanentError 表示重试无法修复的 Action 错误。
type PermanentError struct {
	err error
}

// Error 返回 Action 错误。
func (e *PermanentError) Error() string { return e.err.Error() }

// Unwrap 返回原始错误。
func (e *PermanentError) Unwrap() error { return e.err }

// Permanent 将 Action 错误标记为无需重试。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{err: err}
}

// IsPermanent 判断 Action 错误是否无需重试。
func IsPermanent(err error) bool {
	var target *PermanentError
	return errors.As(err, &target)
}

// ScheduleDefinition 定义一个由代码管理的定时 Action。
type ScheduleDefinition struct {
	Key              string
	ActionName       string
	Queue            string
	Payload          any
	CronExpression   string
	Timezone         string
	Enabled          bool
	MaxAttempts      int
	StartImmediately bool
}
