//go:build !server && !ios && !android

package models

import "github.com/uptrace/bun"

// DeviceRegistration 表示桌面端本机设备为一个账号在一个工作区中的注册结果。
type DeviceRegistration struct {
	bun.BaseModel `bun:"table:device_registrations,alias:device_registration"`

	ServerURL      string `bun:"server_url,pk"`
	AccountID      string `bun:"account_id,pk"`
	OrganizationID string `bun:"organization_id,pk"`
	DeviceID       string `bun:"device_id"`
	RegisteredAt   string `bun:"registered_at"`
}
