//go:build !server && !ios && !android

// Package models 定义桌面端 SQLite 使用的 Bun 数据模型。
package models

import "github.com/uptrace/bun"

// AppSetting 表示桌面端 SQLite 中的一项应用配置。
type AppSetting struct {
	bun.BaseModel `bun:"table:app_settings,alias:setting"`

	Key   string `bun:"key,pk"`
	Value string `bun:"value"`
}
