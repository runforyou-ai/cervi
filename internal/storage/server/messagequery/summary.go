//go:build server

// Package messagequery 提供消息查询共用的展示表达式。
package messagequery

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// Summary 返回正文优先、无正文附件使用文件名、已删除消息为空的摘要。
func Summary(alias string) schema.QueryWithArgs {
	name := bun.Ident(alias)
	return bun.SafeQuery(`CASE WHEN ?.deleted_at IS NOT NULL THEN ''
 WHEN ?.type = ? AND ?.body = '' THEN (
  SELECT ma.name FROM message_attachments AS ma WHERE ma.message_id = ?.id AND ma.organization_id = ?.organization_id
 ) ELSE ?.body END`, name, name, domain.MessageTypeAttachment, name, name, name, name)
}
