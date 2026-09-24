package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

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

// AIPerformanceSummary 定义统计范围内已关闭周期的计数，排除小结状态为无实质诉求的周期：Resolved 与 Unresolved 按小结的是否解决计数，其余为未判定；AIOnly 为 AI 员工独立处理并关闭的周期数，AIResolved 与 AIUnresolved 为其中的已解决与未解决数；HandedOff 为发生过转人工的周期数，CloseAIResolved 等为按结束方式的周期数。
type AIPerformanceSummary struct {
	Closed               int `json:"closed"`
	Resolved             int `json:"resolved"`
	Unresolved           int `json:"unresolved"`
	AIOnly               int `json:"aiOnly"`
	AIResolved           int `json:"aiResolved"`
	AIUnresolved         int `json:"aiUnresolved"`
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

// AIPerformanceReport 定义 AI 表现报表概览：整体计数、转人工原因分布，以及所选渠道下全部待处理的待补知识条数，该条数不受统计天数限制。
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

// AIPerformanceBreakdown 定义按渠道或咨询分类拆分的已关闭周期数、已解决数与 AI 独立解决数；ID 为空表示未分类。
type AIPerformanceBreakdown struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Closed     int    `json:"closed"`
	Resolved   int    `json:"resolved"`
	AIResolved int    `json:"aiResolved"`
}

// AIPerformanceBreakdownList 定义一页拆分结果。
type AIPerformanceBreakdownList struct {
	Rows []AIPerformanceBreakdown `json:"rows"`
	Page PageInfo                 `json:"page"`
}
