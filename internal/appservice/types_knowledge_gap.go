package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// KnowledgeGapSource 定义待补知识的来源。
type KnowledgeGapSource string

const (
	KnowledgeGapSourceKnowledgeGap         KnowledgeGapSource = KnowledgeGapSource(domain.KnowledgeGapSourceKnowledgeGap)
	KnowledgeGapSourceInsufficientEvidence KnowledgeGapSource = KnowledgeGapSource(domain.KnowledgeGapSourceInsufficientEvidence)
	KnowledgeGapSourceRatedUnresolved      KnowledgeGapSource = KnowledgeGapSource(domain.KnowledgeGapSourceRatedUnresolved)
	KnowledgeGapSourcePossiblyWrong        KnowledgeGapSource = KnowledgeGapSource(domain.KnowledgeGapSourcePossiblyWrong)
)

// KnowledgeGapStatus 定义待补知识的处理状态。
type KnowledgeGapStatus string

const (
	KnowledgeGapStatusPending   KnowledgeGapStatus = KnowledgeGapStatus(domain.KnowledgeGapStatusPending)
	KnowledgeGapStatusAccepted  KnowledgeGapStatus = KnowledgeGapStatus(domain.KnowledgeGapStatusAccepted)
	KnowledgeGapStatusDismissed KnowledgeGapStatus = KnowledgeGapStatus(domain.KnowledgeGapStatusDismissed)
)

// KnowledgeGapDraftStatus 定义待补知识问答草稿的状态。
type KnowledgeGapDraftStatus string

const (
	KnowledgeGapDraftStatusPending     KnowledgeGapDraftStatus = KnowledgeGapDraftStatus(domain.KnowledgeGapDraftStatusPending)
	KnowledgeGapDraftStatusReady       KnowledgeGapDraftStatus = KnowledgeGapDraftStatus(domain.KnowledgeGapDraftStatusReady)
	KnowledgeGapDraftStatusFailed      KnowledgeGapDraftStatus = KnowledgeGapDraftStatus(domain.KnowledgeGapDraftStatusFailed)
	KnowledgeGapDraftStatusUnavailable KnowledgeGapDraftStatus = KnowledgeGapDraftStatus(domain.KnowledgeGapDraftStatusUnavailable)
)

// KnowledgeGapMessageSender 定义待补知识沟通记录中的发送方。
type KnowledgeGapMessageSender string

const (
	KnowledgeGapMessageSenderCustomer KnowledgeGapMessageSender = "customer"
	KnowledgeGapMessageSenderAI       KnowledgeGapMessageSender = "ai"
	KnowledgeGapMessageSenderStaff    KnowledgeGapMessageSender = "staff"
)

// KnowledgeGapListInput 定义待补知识清单的渠道、处理状态与分页，ChannelID 为空表示全部渠道。
type KnowledgeGapListInput struct {
	ChannelID string             `json:"channelId" query:"channelId"`
	Status    KnowledgeGapStatus `json:"status" query:"status,default=pending"`
	Page      int                `json:"page" query:"page,default=1"`
	PageSize  int                `json:"pageSize" query:"pageSize,default=50"`
}

// KnowledgeGapSummary 定义清单中的一条待补知识；Question 优先取 AI 起草的问题，其次为客户提问原文，HasDraft 表示 AI 已起草问答。
type KnowledgeGapSummary struct {
	ID                string             `json:"id"`
	ConversationID    string             `json:"conversationId"`
	QuestionMessageID string             `json:"questionMessageId"`
	Question          string             `json:"question"`
	Source            KnowledgeGapSource `json:"source"`
	Status            KnowledgeGapStatus `json:"status"`
	CategoryName      string             `json:"categoryName"`
	HasDraft          bool               `json:"hasDraft"`
	OccurredAt        time.Time          `json:"occurredAt"`
}

// KnowledgeGapList 定义一页待补知识，按来源发生时间倒序排列。
type KnowledgeGapList struct {
	Gaps []KnowledgeGapSummary `json:"gaps"`
	Page PageInfo              `json:"page"`
}

// KnowledgeGapMessage 定义待补知识来源周期中的一条对客消息，客户的 SenderName 为空。
type KnowledgeGapMessage struct {
	ID         string                    `json:"id"`
	Sender     KnowledgeGapMessageSender `json:"sender"`
	SenderName string                    `json:"senderName"`
	Body       string                    `json:"body"`
	CreatedAt  time.Time                 `json:"createdAt"`
}

// KnowledgeGapDraft 定义 AI 起草的问答；真人客服没有实质答复时 Answer 为空。
type KnowledgeGapDraft struct {
	Question         string   `json:"question"`
	SimilarQuestions []string `json:"similarQuestions"`
	Answer           string   `json:"answer"`
}

// KnowledgeGap 定义待补知识详情：Question 为客户提问原文，DraftStatus 为草稿状态，Draft 只在已起草时给出；DefaultKnowledgeBaseID 为接待 AI 员工绑定的问答知识库，KnowledgeBaseID 与 QAEntryID 为加入的知识库与问答。
type KnowledgeGap struct {
	ID                     string                  `json:"id"`
	ConversationID         string                  `json:"conversationId"`
	QuestionMessageID      string                  `json:"questionMessageId"`
	Question               string                  `json:"question"`
	Source                 KnowledgeGapSource      `json:"source"`
	Status                 KnowledgeGapStatus      `json:"status"`
	CategoryName           string                  `json:"categoryName"`
	OccurredAt             time.Time               `json:"occurredAt"`
	DraftStatus            KnowledgeGapDraftStatus `json:"draftStatus"`
	Draft                  *KnowledgeGapDraft      `json:"draft"`
	DefaultKnowledgeBaseID string                  `json:"defaultKnowledgeBaseId"`
	KnowledgeBaseID        string                  `json:"knowledgeBaseId"`
	QAEntryID              string                  `json:"qaEntryId"`
	Messages               []KnowledgeGapMessage   `json:"messages"`
}

// KnowledgeGapAcceptInput 定义加入知识库的问答：EntryID 为空时新建问答，否则更新该问答。
type KnowledgeGapAcceptInput struct {
	KnowledgeBaseID string           `json:"knowledgeBaseId"`
	EntryID         string           `json:"entryId"`
	Entry           KnowledgeQAInput `json:"entry"`
}
