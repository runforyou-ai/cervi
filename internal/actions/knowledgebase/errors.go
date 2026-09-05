//go:build server

package knowledgebase

import "errors"

var (
	// ErrQANotFound 表示指定知识库中不存在该问答。
	ErrQANotFound = errors.New("knowledge QA entry not found")
	// ErrQAUnsupported 表示知识库不支持本地问答维护。
	ErrQAUnsupported = errors.New("knowledge QA unsupported")
	// ErrBaseHasContent 表示已有内容的知识库不能切换类型或来源。
	ErrBaseHasContent = errors.New("knowledge base has content")
	// ErrNotFound 表示当前企业中不存在指定知识库。
	ErrNotFound = errors.New("knowledge base not found")
	// ErrGroupNotFound 表示知识库中不存在指定分组。
	ErrGroupNotFound = errors.New("knowledge group not found")
	// ErrGroupInvalid 表示分组层级或默认分组操作无效。
	ErrGroupInvalid = errors.New("knowledge group invalid")
	// ErrGroupNotEmpty 表示分组仍包含子分组或问答。
	ErrGroupNotEmpty = errors.New("knowledge group not empty")
	// ErrExternalGroupUnsupported 表示外部知识库不允许创建分组。
	ErrExternalGroupUnsupported = errors.New("external knowledge base group unsupported")
	// ErrDocumentReadUnsupported 表示当前知识库来源不支持读取文档。
	ErrDocumentReadUnsupported = errors.New("knowledge document read unsupported")
	// ErrDocumentNotFound 表示外部知识库中不存在指定文档。
	ErrDocumentNotFound = errors.New("knowledge document not found")
)
