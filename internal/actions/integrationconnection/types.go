//go:build server

// Package integrationconnection 实现外部系统连接器的查询与操作。
package integrationconnection

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// Configuration 定义连接器当前支持的通用认证配置。
type Configuration struct {
	APIURL string
	APIKey string
}

// Input 定义连接器可编辑字段。
type Input struct {
	Type          domain.IntegrationConnectionType
	Name          string
	Description   string
	Configuration Configuration
}

// ConnectionInput 定义连接测试需要的草稿配置。
type ConnectionInput struct {
	Type          domain.IntegrationConnectionType
	Configuration Configuration
}

// TestResult 记录连接器探测后的状态和时间。
type TestResult struct {
	Status   domain.IntegrationConnectionStatus
	TestedAt time.Time
}

// Record 定义连接器完整信息。
type Record struct {
	ID            string
	Type          domain.IntegrationConnectionType
	Name          string
	Description   string
	Configuration Configuration
	Status        domain.IntegrationConnectionStatus
	LastTestedAt  *time.Time
}

// Summary 定义连接器列表项。
type Summary struct {
	ID           string
	Type         domain.IntegrationConnectionType
	Name         string
	Description  string
	Status       domain.IntegrationConnectionStatus
	LastTestedAt *time.Time
}
