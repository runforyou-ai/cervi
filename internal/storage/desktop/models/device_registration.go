//go:build !server && !ios && !android

package models

import "github.com/uptrace/bun"

// DeviceRegistration 表示桌面端本机设备在一个企业服务器上的注册结果。
type DeviceRegistration struct {
	bun.BaseModel `bun:"table:device_registrations,alias:device_registration"`

	ServerURL      string `bun:"server_url,pk"`
	OrganizationID string `bun:"organization_id,pk"`
	DeviceID       string `bun:"device_id"`
	RegisteredAt   string `bun:"registered_at"`
}
