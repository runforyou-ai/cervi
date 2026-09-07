//go:build server

// Package channelstate 串行化 Telegram 渠道配置与外发操作。
package channelstate

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/uptrace/bun"
	"log/slog"
	"time"
)

// WithTelegramLock 在专用数据库连接上串行执行单个渠道的完整远端生命周期。
func WithTelegramLock(ctx context.Context, db *bun.DB, channelID string, execute func(bun.Conn) error) error {
	return withTelegramLock(ctx, db, channelID, true, execute)
}

// TryTelegramLock 跳过正在配置或发送的渠道，避免占满任务 Worker。
func TryTelegramLock(ctx context.Context, db *bun.DB, channelID string, execute func(bun.Conn) error) error {
	return withTelegramLock(ctx, db, channelID, false, execute)
}

// withTelegramLock 在独立连接上执行渠道操作并可靠释放会话锁。
func withTelegramLock(ctx context.Context, db *bun.DB, channelID string, wait bool, execute func(bun.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire Telegram channel connection: %w", err)
	}
	defer conn.Close()
	if wait {
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtextextended(?, 0))", channelID); err != nil {
			return fmt.Errorf("lock Telegram channel: %w", err)
		}
	} else {
		var acquired bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended(?, 0))", channelID).Scan(&acquired); err != nil {
			return err
		}
		if !acquired {
			return nil
		}
	}
	defer func(conn bun.Conn, channelID string) {
		// 释放会话锁，失败时丢弃底层连接避免锁泄漏进连接池。
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(releaseCtx, "SELECT pg_advisory_unlock(hashtextextended(?, 0))", channelID); err == nil {
			return
		}
		slog.Error("释放 Telegram 渠道锁失败", "channel_id", channelID)
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}(conn, channelID)
	return execute(conn)
}
