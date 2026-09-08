package domain

const (
	// KnowledgeBaseNameMaxLength 是知识库名称允许的最大字符数。
	KnowledgeBaseNameMaxLength = 120
	// KnowledgeBaseDescriptionMaxLength 是知识库描述允许的最大字符数。
	KnowledgeBaseDescriptionMaxLength = 1000
	// KnowledgeGroupNameMaxLength 是知识库分组名称允许的最大字符数。
	KnowledgeGroupNameMaxLength = 120
	// KnowledgeRetrievalQueryMaxLength 是知识库检索内容允许的最大字符数。
	KnowledgeRetrievalQueryMaxLength = 250
)

// KnowledgeBaseCategory 表示知识库内容类型。
type KnowledgeBaseCategory string

const (
	KnowledgeBaseCategoryStandard KnowledgeBaseCategory = "standard"
	KnowledgeBaseCategoryQA       KnowledgeBaseCategory = "qa"
)
