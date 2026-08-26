package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// IntegrationConnectionType 表示外部系统连接器类型。
type IntegrationConnectionType string

const (
	IntegrationConnectionTypeDify IntegrationConnectionType = IntegrationConnectionType(domain.IntegrationConnectionTypeDify)
	IntegrationConnectionTypeN8N  IntegrationConnectionType = IntegrationConnectionType(domain.IntegrationConnectionTypeN8N)
)

// IntegrationConnectionStatus 表示连接器最近一次测试状态。
type IntegrationConnectionStatus string

const (
	IntegrationConnectionStatusUntested    IntegrationConnectionStatus = IntegrationConnectionStatus(domain.IntegrationConnectionStatusUntested)
	IntegrationConnectionStatusAvailable   IntegrationConnectionStatus = IntegrationConnectionStatus(domain.IntegrationConnectionStatusAvailable)
	IntegrationConnectionStatusUnavailable IntegrationConnectionStatus = IntegrationConnectionStatus(domain.IntegrationConnectionStatusUnavailable)
)

// IntegrationConnectionConfiguration 定义连接器认证配置。
type IntegrationConnectionConfiguration struct {
	APIURL string `json:"apiUrl"`
	APIKey string `json:"apiKey"`
}

// IntegrationConnectionInput 定义连接器可编辑字段。
type IntegrationConnectionInput struct {
	Type          IntegrationConnectionType          `json:"type"`
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	Configuration IntegrationConnectionConfiguration `json:"configuration"`
}

// IntegrationConnectionTestInput 定义连接器草稿测试字段。
type IntegrationConnectionTestInput struct {
	Type          IntegrationConnectionType          `json:"type"`
	Configuration IntegrationConnectionConfiguration `json:"configuration"`
}

// IntegrationConnection 定义连接器详情。
type IntegrationConnection struct {
	ID            string                             `json:"id"`
	Type          IntegrationConnectionType          `json:"type"`
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	Configuration IntegrationConnectionConfiguration `json:"configuration"`
	Status        IntegrationConnectionStatus        `json:"status"`
	LastTestedAt  *time.Time                         `json:"lastTestedAt"`
}

// IntegrationConnectionSummary 定义连接器列表项。
type IntegrationConnectionSummary struct {
	ID           string                      `json:"id"`
	Type         IntegrationConnectionType   `json:"type"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Status       IntegrationConnectionStatus `json:"status"`
	LastTestedAt *time.Time                  `json:"lastTestedAt"`
}

// IntegrationConnectionList 定义连接器列表。
type IntegrationConnectionList struct {
	Connections []IntegrationConnectionSummary `json:"connections"`
}
