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
)

// KnowledgeBaseCategory 表示知识库内容类型。
type KnowledgeBaseCategory string

const (
	KnowledgeBaseCategoryStandard KnowledgeBaseCategory = "standard"
	KnowledgeBaseCategoryQA       KnowledgeBaseCategory = "qa"
)

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
