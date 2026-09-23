package appservice

import "time"

// AIPerformanceReportInput 定义 AI 表现报表的统计范围：最近 Days 天内结束的会话，ChannelID 为空表示全部渠道。
type AIPerformanceReportInput struct {
	Days      int    `json:"days" query:"days,default=30"`
	ChannelID string `json:"channelId" query:"channelId"`
}

// AIPerformanceSummary 定义统计范围内已关闭周期的整体计数；AIResolved 为 AI 独立解决的周期数，HandedOff 为发生过转人工的周期数，CloseAIResolved 等为按结束方式的周期数。
type AIPerformanceSummary struct {
	Closed               int `json:"closed"`
	AIResolved           int `json:"aiResolved"`
	HandedOff            int `json:"handedOff"`
	CloseAIResolved      int `json:"closeAiResolved"`
	CustomerUnresponsive int `json:"customerUnresponsive"`
	Manual               int `json:"manual"`
	Rated                int `json:"rated"`
	RatedResolved        int `json:"ratedResolved"`
}

// AIPerformanceBreakdown 定义按渠道或咨询分类拆分的已关闭周期数与 AI 独立解决数；ID 为空表示未分类。
type AIPerformanceBreakdown struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Closed     int    `json:"closed"`
	AIResolved int    `json:"aiResolved"`
}

// AIHandoffReasonCount 定义一种转人工原因在统计范围内出现的次数。
type AIHandoffReasonCount struct {
	Reason AgentHandoffReason `json:"reason"`
	Count  int                `json:"count"`
}

// AIKnowledgeGap 定义一次因知识不足或缺少依据的转人工；EventID 为转人工事件编号，MessageID 为客户提问消息编号，客户没有文本提问时为空。
type AIKnowledgeGap struct {
	EventID        string             `json:"eventId"`
	ConversationID string             `json:"conversationId"`
	MessageID      string             `json:"messageId"`
	Question       string             `json:"question"`
	Reason         AgentHandoffReason `json:"reason"`
	CategoryName   string             `json:"categoryName"`
	OccurredAt     time.Time          `json:"occurredAt"`
}

// AIPerformanceReport 定义 AI 表现报表；KnowledgeGaps 按时间倒序只含最近的部分，KnowledgeGapTotal 为范围内总数。
type AIPerformanceReport struct {
	Summary           AIPerformanceSummary     `json:"summary"`
	Channels          []AIPerformanceBreakdown `json:"channels"`
	Categories        []AIPerformanceBreakdown `json:"categories"`
	HandoffReasons    []AIHandoffReasonCount   `json:"handoffReasons"`
	KnowledgeGaps     []AIKnowledgeGap         `json:"knowledgeGaps"`
	KnowledgeGapTotal int                      `json:"knowledgeGapTotal"`
}
