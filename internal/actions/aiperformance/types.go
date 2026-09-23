//go:build server

// Package aiperformance 汇总 AI 客服表现：解决率、结束方式、客户评价、转人工原因、按渠道与咨询分类的拆分和知识缺口。
package aiperformance

import (
	"errors"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

var (
	// ErrPageSizeInvalid 表示每页数量超出上限。
	ErrPageSizeInvalid = errors.New("ai performance page size invalid")
	// ErrDimensionInvalid 表示拆分维度不是渠道或咨询分类。
	ErrDimensionInvalid = errors.New("ai performance dimension invalid")
)

// Input 定义报表统计范围：最近 Days 天内关闭的周期，ChannelID 为空表示全部渠道。
type Input struct {
	Days      int
	ChannelID string
}

// Summary 定义统计范围内已关闭周期的整体计数，排除小结状态为无实质诉求的周期：Resolved 与 Unresolved 按小结的是否解决计数，其余为未判定；AIOnly 为 AI 员工独立处理并关闭的周期数，AIResolved 与 AIUnresolved 为其中的已解决与未解决数，HandedOff 为发生过转人工的周期数。
type Summary struct {
	Closed               int `bun:"closed"`
	Resolved             int `bun:"resolved"`
	Unresolved           int `bun:"unresolved"`
	AIOnly               int `bun:"ai_only"`
	AIResolved           int `bun:"ai_resolved"`
	AIUnresolved         int `bun:"ai_unresolved"`
	HandedOff            int `bun:"handed_off"`
	CloseAIResolved      int `bun:"close_ai_resolved"`
	CustomerUnresponsive int `bun:"customer_unresponsive"`
	Manual               int `bun:"manual"`
	Rated                int `bun:"rated"`
	RatedResolved        int `bun:"rated_resolved"`
}

// ReasonCount 定义一种转人工原因的出现次数。
type ReasonCount struct {
	Reason string `bun:"reason"`
	Count  int    `bun:"count"`
}

// Overview 定义报表概览：整体计数、转人工原因分布与知识缺口总数。
type Overview struct {
	Summary           Summary
	HandoffReasons    []ReasonCount
	KnowledgeGapTotal int
}

// BreakdownInput 定义按维度拆分的统计范围与分页。
type BreakdownInput struct {
	Input
	Dimension domain.AIPerformanceDimension
	Page      int
	PageSize  int
}

// Breakdown 定义按渠道或咨询分类拆分的已关闭周期数、已解决数与 AI 独立解决数；ID 为空表示未分类。
type Breakdown struct {
	ID         *string `bun:"id"`
	Name       string  `bun:"name"`
	Closed     int     `bun:"closed"`
	Resolved   int     `bun:"resolved"`
	AIResolved int     `bun:"ai_resolved"`
}

// BreakdownList 定义一页拆分结果与总行数。
type BreakdownList struct {
	Rows     []Breakdown
	Page     int
	PageSize int
	Total    int
}

// KnowledgeGapInput 定义知识缺口清单的统计范围与分页。
type KnowledgeGapInput struct {
	Input
	Page     int
	PageSize int
}

// KnowledgeGap 定义一次因知识不足或缺少依据的转人工，EventID 为转人工事件消息编号，Question 为事件之前客户发出的最后一条文本消息。
type KnowledgeGap struct {
	EventID        string    `bun:"event_id"`
	ConversationID string    `bun:"conversation_id"`
	MessageID      *string   `bun:"message_id"`
	Question       string    `bun:"question"`
	Reason         string    `bun:"reason"`
	CategoryName   *string   `bun:"category_name"`
	OccurredAt     time.Time `bun:"occurred_at"`
}

// KnowledgeGapList 定义一页知识缺口与总条数。
type KnowledgeGapList struct {
	Gaps     []KnowledgeGap
	Page     int
	PageSize int
	Total    int
}
