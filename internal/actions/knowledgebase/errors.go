//go:build server

package knowledgebase

import "errors"

var (
	// ErrNotFound 表示当前企业中不存在指定知识库。
	ErrNotFound = errors.New("knowledge base not found")
	// ErrGroupNotFound 表示知识库中不存在指定分组。
	ErrGroupNotFound = errors.New("knowledge group not found")
	// ErrGroupInvalid 表示分组层级或默认分组操作无效。
	ErrGroupInvalid = errors.New("knowledge group invalid")
	// ErrGroupNotEmpty 表示分组仍包含子分组。
	ErrGroupNotEmpty = errors.New("knowledge group not empty")
	// ErrExternalGroupUnsupported 表示外部知识库不允许创建分组。
	ErrExternalGroupUnsupported = errors.New("external knowledge base group unsupported")
	// ErrDocumentReadUnsupported 表示当前知识库来源不支持读取文档。
	ErrDocumentReadUnsupported = errors.New("knowledge document read unsupported")
	// ErrDocumentNotFound 表示外部知识库中不存在指定文档。
	ErrDocumentNotFound = errors.New("knowledge document not found")
)
