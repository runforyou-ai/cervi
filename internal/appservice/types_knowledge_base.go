package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// KnowledgeBaseCategory 表示知识库内容类型。
type KnowledgeBaseCategory string

const (
	KnowledgeBaseCategoryStandard KnowledgeBaseCategory = KnowledgeBaseCategory(domain.KnowledgeBaseCategoryStandard)
	KnowledgeBaseCategoryQA       KnowledgeBaseCategory = KnowledgeBaseCategory(domain.KnowledgeBaseCategoryQA)
)

// KnowledgeDocumentStatus 表示知识文档的统一状态。
type KnowledgeDocumentStatus string

const (
	KnowledgeDocumentStatusQueued     KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusQueued)
	KnowledgeDocumentStatusProcessing KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusProcessing)
	KnowledgeDocumentStatusReady      KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusReady)
	KnowledgeDocumentStatusPaused     KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusPaused)
	KnowledgeDocumentStatusError      KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusError)
	KnowledgeDocumentStatusDisabled   KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusDisabled)
	KnowledgeDocumentStatusArchived   KnowledgeDocumentStatus = KnowledgeDocumentStatus(domain.KnowledgeDocumentStatusArchived)
)

// KnowledgeDocumentSegmentIndexStatus 表示知识文档分段的索引状态。
type KnowledgeDocumentSegmentIndexStatus string

const (
	KnowledgeDocumentSegmentIndexStatusWaiting   KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusWaiting)
	KnowledgeDocumentSegmentIndexStatusIndexing  KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusIndexing)
	KnowledgeDocumentSegmentIndexStatusCompleted KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusCompleted)
	KnowledgeDocumentSegmentIndexStatusError     KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusError)
	KnowledgeDocumentSegmentIndexStatusPaused    KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusPaused)
	KnowledgeDocumentSegmentIndexStatusResegment KnowledgeDocumentSegmentIndexStatus = KnowledgeDocumentSegmentIndexStatus(domain.KnowledgeDocumentSegmentIndexStatusResegment)
)

// KnowledgeBaseInput 定义知识库可编辑字段。
type KnowledgeBaseInput struct {
	Name                    string                `json:"name"`
	Category                KnowledgeBaseCategory `json:"category"`
	Description             string                `json:"description"`
	IntegrationConnectionID string                `json:"integrationConnectionId"`
	ExternalResourceID      string                `json:"externalResourceId"`
}

// KnowledgeGroupInput 定义知识库分组可编辑字段。
type KnowledgeGroupInput struct {
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
}

// KnowledgeGroup 定义知识库分组树节点。
type KnowledgeGroup struct {
	ID        string           `json:"id"`
	ParentID  string           `json:"parentId"`
	Name      string           `json:"name"`
	IsDefault bool             `json:"isDefault"`
	Children  []KnowledgeGroup `json:"children"`
}

// KnowledgeBase 定义知识库详情。
type KnowledgeBase struct {
	ID                      string                `json:"id"`
	Name                    string                `json:"name"`
	Category                KnowledgeBaseCategory `json:"category"`
	Description             string                `json:"description"`
	IntegrationConnectionID string                `json:"integrationConnectionId"`
	ExternalResourceID      string                `json:"externalResourceId"`
	Groups                  []KnowledgeGroup      `json:"groups"`
	CreatedAt               time.Time             `json:"createdAt"`
	UpdatedAt               time.Time             `json:"updatedAt"`
}

// KnowledgeBaseList 定义知识库列表。
type KnowledgeBaseList struct {
	KnowledgeBases []KnowledgeBase `json:"knowledgeBases"`
}

// ExternalKnowledgeBaseOption 定义外部知识库选择项。
type ExternalKnowledgeBaseOption struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Category KnowledgeBaseCategory `json:"category"`
}

// ExternalKnowledgeBaseOptionList 定义外部知识库选择项列表。
type ExternalKnowledgeBaseOptionList struct {
	KnowledgeBases []ExternalKnowledgeBaseOption `json:"knowledgeBases"`
}

// KnowledgeDocumentListInput 定义知识文档列表查询条件。
type KnowledgeDocumentListInput struct {
	Keyword  string                   `json:"keyword" query:"keyword"`
	Status   *KnowledgeDocumentStatus `json:"status,omitempty" query:"status"`
	Page     int                      `json:"page" query:"page,default=1"`
	PageSize int                      `json:"pageSize" query:"pageSize,default=20"`
}

// KnowledgeDocumentSummary 定义知识文档列表项。
type KnowledgeDocumentSummary struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Status    KnowledgeDocumentStatus `json:"status"`
	CreatedAt *time.Time              `json:"createdAt"`
}

// KnowledgeDocumentList 定义知识文档分页结果。
type KnowledgeDocumentList struct {
	Documents []KnowledgeDocumentSummary `json:"documents"`
	Page      PageInfo                   `json:"page"`
}

// KnowledgeDocument 定义知识文档详情。
type KnowledgeDocument struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Status    KnowledgeDocumentStatus `json:"status"`
	WordCount *int                    `json:"wordCount"`
	HitCount  int                     `json:"hitCount"`
	CreatedAt *time.Time              `json:"createdAt"`
}

// KnowledgeDocumentSegmentListInput 定义知识文档分段列表查询条件。
type KnowledgeDocumentSegmentListInput struct {
	Keyword  string `json:"keyword" query:"keyword"`
	Page     int    `json:"page" query:"page,default=1"`
	PageSize int    `json:"pageSize" query:"pageSize,default=20"`
}

// KnowledgeDocumentSegment 定义知识文档分段列表项。
type KnowledgeDocumentSegment struct {
	ID          string                              `json:"id"`
	Position    int                                 `json:"position"`
	Content     string                              `json:"content"`
	Answer      *string                             `json:"answer"`
	WordCount   int                                 `json:"wordCount"`
	HitCount    int                                 `json:"hitCount"`
	IndexStatus KnowledgeDocumentSegmentIndexStatus `json:"indexStatus"`
	CreatedAt   *time.Time                          `json:"createdAt"`
}

// KnowledgeDocumentSegmentList 定义知识文档分段分页结果。
type KnowledgeDocumentSegmentList struct {
	Segments []KnowledgeDocumentSegment `json:"segments"`
	Page     PageInfo                   `json:"page"`
}
