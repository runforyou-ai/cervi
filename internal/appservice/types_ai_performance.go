package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// AIPerformanceDimension 定义 AI 表现报表的拆分维度。
type AIPerformanceDimension string

const (
	AIPerformanceDimensionChannel  AIPerformanceDimension = AIPerformanceDimension(domain.AIPerformanceDimensionChannel)
	AIPerformanceDimensionCategory AIPerformanceDimension = AIPerformanceDimension(domain.AIPerformanceDimensionCategory)
)

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

// AIHandoffReasonCount 定义一种转人工原因在统计范围内出现的次数。
type AIHandoffReasonCount struct {
	Reason AgentHandoffReason `json:"reason"`
	Count  int                `json:"count"`
}

// AIPerformanceReport 定义 AI 表现报表概览：整体计数、转人工原因分布与知识缺口总数。
type AIPerformanceReport struct {
	Summary           AIPerformanceSummary   `json:"summary"`
	HandoffReasons    []AIHandoffReasonCount `json:"handoffReasons"`
	KnowledgeGapTotal int                    `json:"knowledgeGapTotal"`
}

// AIPerformanceBreakdownInput 定义按维度拆分的统计范围与分页。
type AIPerformanceBreakdownInput struct {
	Days      int                    `json:"days" query:"days,default=30"`
	ChannelID string                 `json:"channelId" query:"channelId"`
	Dimension AIPerformanceDimension `json:"dimension" query:"dimension"`
	Page      int                    `json:"page" query:"page,default=1"`
	PageSize  int                    `json:"pageSize" query:"pageSize,default=50"`
}

// AIPerformanceBreakdown 定义按渠道或咨询分类拆分的已关闭周期数与 AI 独立解决数；ID 为空表示未分类。
type AIPerformanceBreakdown struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Closed     int    `json:"closed"`
	AIResolved int    `json:"aiResolved"`
}

// AIPerformanceBreakdownList 定义一页拆分结果。
type AIPerformanceBreakdownList struct {
	Rows []AIPerformanceBreakdown `json:"rows"`
	Page PageInfo                 `json:"page"`
}

// AIKnowledgeGapInput 定义知识缺口清单的统计范围与分页。
type AIKnowledgeGapInput struct {
	Days      int    `json:"days" query:"days,default=30"`
	ChannelID string `json:"channelId" query:"channelId"`
	Page      int    `json:"page" query:"page,default=1"`
	PageSize  int    `json:"pageSize" query:"pageSize,default=50"`
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

// AIKnowledgeGapList 定义一页知识缺口，按转人工时间倒序排列。
type AIKnowledgeGapList struct {
	Gaps []AIKnowledgeGap `json:"gaps"`
	Page PageInfo         `json:"page"`
}
