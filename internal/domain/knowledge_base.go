package domain

const (
	// KnowledgeBaseNameMaxLength 是知识库名称允许的最大字符数。
	KnowledgeBaseNameMaxLength = 120
	// KnowledgeBaseDescriptionMaxLength 是知识库描述允许的最大字符数。
	KnowledgeBaseDescriptionMaxLength = 1000
	// KnowledgeBaseExternalResourceIDMaxLength 是外部知识库编号允许的最大字符数。
	KnowledgeBaseExternalResourceIDMaxLength = 512
	// KnowledgeGroupNameMaxLength 是知识库分组名称允许的最大字符数。
	KnowledgeGroupNameMaxLength = 120
	// KnowledgeRetrievalQueryMaxLength 是知识库检索内容允许的最大字符数。
	KnowledgeRetrievalQueryMaxLength = 250
	// KnowledgeRetrievalTopKMax 是知识库单次检索允许返回的最大分段数。
	KnowledgeRetrievalTopKMax = 100
)

// KnowledgeBaseCategory 表示知识库内容类型。
type KnowledgeBaseCategory string

const (
	KnowledgeBaseCategoryStandard KnowledgeBaseCategory = "standard"
	KnowledgeBaseCategoryQA       KnowledgeBaseCategory = "qa"
)

// KnowledgeRetrievalMethod 表示知识库检索方式。
type KnowledgeRetrievalMethod string

const (
	KnowledgeRetrievalMethodKeyword  KnowledgeRetrievalMethod = "keyword"
	KnowledgeRetrievalMethodSemantic KnowledgeRetrievalMethod = "semantic"
	KnowledgeRetrievalMethodFullText KnowledgeRetrievalMethod = "full_text"
	KnowledgeRetrievalMethodHybrid   KnowledgeRetrievalMethod = "hybrid"
)

// KnowledgeRetrievalOptions 定义各类知识库共用的检索参数。
type KnowledgeRetrievalOptions struct {
	Method                KnowledgeRetrievalMethod
	RerankingEnabled      bool
	TopK                  int
	ScoreThresholdEnabled bool
	ScoreThreshold        float64
}

// KnowledgeDocumentStatus 表示知识文档的统一状态。
type KnowledgeDocumentStatus string

const (
	KnowledgeDocumentStatusQueued     KnowledgeDocumentStatus = "queued"
	KnowledgeDocumentStatusProcessing KnowledgeDocumentStatus = "processing"
	KnowledgeDocumentStatusReady      KnowledgeDocumentStatus = "ready"
	KnowledgeDocumentStatusPaused     KnowledgeDocumentStatus = "paused"
	KnowledgeDocumentStatusError      KnowledgeDocumentStatus = "error"
	KnowledgeDocumentStatusDisabled   KnowledgeDocumentStatus = "disabled"
	KnowledgeDocumentStatusArchived   KnowledgeDocumentStatus = "archived"
)

// KnowledgeDocumentSegmentIndexStatus 表示知识文档分段的索引状态。
type KnowledgeDocumentSegmentIndexStatus string

const (
	KnowledgeDocumentSegmentIndexStatusWaiting   KnowledgeDocumentSegmentIndexStatus = "waiting"
	KnowledgeDocumentSegmentIndexStatusIndexing  KnowledgeDocumentSegmentIndexStatus = "indexing"
	KnowledgeDocumentSegmentIndexStatusCompleted KnowledgeDocumentSegmentIndexStatus = "completed"
	KnowledgeDocumentSegmentIndexStatusError     KnowledgeDocumentSegmentIndexStatus = "error"
	KnowledgeDocumentSegmentIndexStatusPaused    KnowledgeDocumentSegmentIndexStatus = "paused"
	KnowledgeDocumentSegmentIndexStatusResegment KnowledgeDocumentSegmentIndexStatus = "re_segment"
)
