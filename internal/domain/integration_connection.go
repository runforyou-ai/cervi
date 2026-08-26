package domain

// IntegrationConnectionType 定义外部系统连接器类型。
type IntegrationConnectionType string

const (
	IntegrationConnectionTypeDify IntegrationConnectionType = "dify"
	IntegrationConnectionTypeN8N  IntegrationConnectionType = "n8n"
)

// IntegrationConnectionStatus 定义连接器最近一次测试状态。
type IntegrationConnectionStatus string

const (
	IntegrationConnectionStatusUntested    IntegrationConnectionStatus = "untested"
	IntegrationConnectionStatusAvailable   IntegrationConnectionStatus = "available"
	IntegrationConnectionStatusUnavailable IntegrationConnectionStatus = "unavailable"
)
