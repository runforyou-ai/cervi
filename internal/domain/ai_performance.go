package domain

// AIPerformanceDimension 定义 AI 表现报表的拆分维度。
type AIPerformanceDimension string

const (
	AIPerformanceDimensionChannel  AIPerformanceDimension = "channel"
	AIPerformanceDimensionCategory AIPerformanceDimension = "category"
)

// AIPerformanceIssueType 定义问题会话的筛选类型。
type AIPerformanceIssueType string

const (
	// AIPerformanceIssueTypeAll 表示满意度为不满意或任一质检标记成立。
	AIPerformanceIssueTypeAll AIPerformanceIssueType = "all"
	// AIPerformanceIssueTypeDissatisfied 表示推断满意度为不满意。
	AIPerformanceIssueTypeDissatisfied AIPerformanceIssueType = "dissatisfied"
	// AIPerformanceIssueTypeAIIncorrect 表示 AI 客服答错。
	AIPerformanceIssueTypeAIIncorrect AIPerformanceIssueType = "ai_incorrect"
	// AIPerformanceIssueTypeAIMissedHandoff 表示 AI 客服应转人工未转。
	AIPerformanceIssueTypeAIMissedHandoff AIPerformanceIssueType = "ai_missed_handoff"
	// AIPerformanceIssueTypeAIPoorAttitude 表示 AI 客服态度问题。
	AIPerformanceIssueTypeAIPoorAttitude AIPerformanceIssueType = "ai_poor_attitude"
)
