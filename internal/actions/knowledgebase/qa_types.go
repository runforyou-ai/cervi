//go:build server

package knowledgebase

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// QASimilarQuestion 定义带稳定编号的相似问题，新问题的编号为空。
type QASimilarQuestion struct {
	ID      string
	Content string
}

// QAInput 定义整条问答的可编辑内容。
type QAInput struct {
	GroupID          string
	Question         string
	SimilarQuestions []QASimilarQuestion
	Answer           string
}

// QAListInput 定义分组中的问答查询条件。
type QAListInput struct {
	GroupID  string
	Keyword  string
	Page     int
	PageSize int
}

// QASummary 定义问答列表项及其索引状态。
type QASummary struct {
	ID               string                      `bun:"id"`
	GroupID          string                      `bun:"group_id"`
	Question         string                      `bun:"question"`
	SimilarQuestions []string                    `bun:"similar_questions,array"`
	Answer           string                      `bun:"answer"`
	Status           domain.KnowledgeIndexStatus `bun:"status"`
	FailureCode      string                      `bun:"failure_code"`
	CreatedAt        time.Time                   `bun:"created_at"`
}

// QARecord 定义完整问答详情。
type QARecord struct {
	ID               string
	GroupID          string
	Question         string
	SimilarQuestions []QASimilarQuestion
	Answer           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// QAListOutput 定义问答分页结果。
type QAListOutput struct {
	Entries  []QASummary
	Page     int
	PageSize int
	Total    int
}
