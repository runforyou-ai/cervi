//go:build server

// Package mcpserver 实现企业 MCP 服务的查询与管理。
package mcpserver

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	"time"
)

// Input 定义 MCP 服务可编辑字段。
type Input struct {
	Name               string
	URL                string
	ServerType         domain.MCPServerType
	AuthorizationToken string
}

// Record 定义 MCP 服务记录。
type Record struct {
	ID                 string
	Name               string
	URL                string
	ServerType         domain.MCPServerType
	AuthorizationToken string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
