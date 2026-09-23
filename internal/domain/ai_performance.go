package domain

// AIPerformanceDimension 定义 AI 表现报表的拆分维度。
type AIPerformanceDimension string

const (
	AIPerformanceDimensionChannel  AIPerformanceDimension = "channel"
	AIPerformanceDimensionCategory AIPerformanceDimension = "category"
)
