//go:build server

package knowledgebase

import "errors"

var (
	// ErrQANotFound 表示指定知识库中不存在该问答。
	ErrQANotFound = errors.New("knowledge QA entry not found")
	// ErrQAUnsupported 表示知识库不支持本地问答维护。
	ErrQAUnsupported = errors.New("knowledge QA unsupported")
	// ErrBaseHasContent 表示知识库类型受已有内容限制。
	ErrBaseHasContent = errors.New("knowledge base has content")
	// ErrNotFound 表示当前企业中不存在指定知识库。
	ErrNotFound = errors.New("knowledge base not found")
	// ErrGroupNotFound 表示知识库中不存在指定分组。
	ErrGroupNotFound = errors.New("knowledge group not found")
	// ErrGroupInvalid 表示分组层级或默认分组操作无效。
	ErrGroupInvalid = errors.New("knowledge group invalid")
	// ErrGroupNotEmpty 表示分组仍包含子分组或问答。
	ErrGroupNotEmpty = errors.New("knowledge group not empty")
)
