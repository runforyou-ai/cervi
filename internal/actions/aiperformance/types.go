//go:build server

// Package aiperformance 汇总 AI 客服表现：解决率、结束方式、客户评价、转人工原因与知识缺口。
package aiperformance

import "time"

// KnowledgeGapLimit 是知识缺口清单返回的最多条数。
const KnowledgeGapLimit = 50

// Input 定义报表统计范围：最近 Days 天内关闭的周期，ChannelID 为空表示全部渠道。
type Input struct {
	Days      int
	ChannelID string
}

// Summary 定义统计范围内已关闭周期的整体计数；HandedOff 为发生过转人工的周期数。
type Summary struct {
	Closed               int `bun:"closed"`
	AIResolved           int `bun:"ai_resolved"`
	HandedOff            int `bun:"handed_off"`
	CloseAIResolved      int `bun:"close_ai_resolved"`
	CustomerUnresponsive int `bun:"customer_unresponsive"`
	Manual               int `bun:"manual"`
	Rated                int `bun:"rated"`
	RatedResolved        int `bun:"rated_resolved"`
}

// Breakdown 定义按渠道或咨询分类拆分的已关闭周期数与 AI 解决数；ID 为空表示未分类。
type Breakdown struct {
	ID         *string `bun:"id"`
	Name       string  `bun:"name"`
	Closed     int     `bun:"closed"`
	AIResolved int     `bun:"ai_resolved"`
}

// ReasonCount 定义一种转人工原因的出现次数。
type ReasonCount struct {
	Reason string `bun:"reason"`
	Count  int    `bun:"count"`
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

// Report 定义 AI 表现报表。
type Report struct {
	Summary           Summary
	Channels          []Breakdown
	Categories        []Breakdown
	HandoffReasons    []ReasonCount
	KnowledgeGaps     []KnowledgeGap
	KnowledgeGapTotal int
}
