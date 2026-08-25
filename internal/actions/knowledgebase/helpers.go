//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// loadKnowledgeBase 读取当前企业中的知识库。
func loadKnowledgeBase(ctx context.Context, db bun.IDB, organizationID, knowledgeBaseID string) (*servermodels.KnowledgeBase, error) {
	if !common.ValidUUID(knowledgeBaseID) {
		return nil, ErrNotFound
	}
	knowledgeBase := &servermodels.KnowledgeBase{}
	if err := db.NewSelect().
		Model(knowledgeBase).
		Where("kb.id = ?", knowledgeBaseID).
		Where("kb.organization_id = ?", organizationID).
		Scan(ctx); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return knowledgeBase, nil
}

// loadKnowledgeBaseRecord 读取带分组树的知识库详情。
func loadKnowledgeBaseRecord(ctx context.Context, db bun.IDB, organizationID, knowledgeBaseID string) (*Record, error) {
	knowledgeBase, err := loadKnowledgeBase(ctx, db, organizationID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	record := recordFromModel(*knowledgeBase)
	record.Groups, err = loadGroupRecords(ctx, db, knowledgeBase.ID)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// loadGroupRecords 返回知识库的两级分组树。
func loadGroupRecords(ctx context.Context, db bun.IDB, knowledgeBaseID string) ([]GroupRecord, error) {
	flat := make([]GroupRecord, 0)
	if err := db.NewSelect().
		TableExpr("knowledge_groups AS kg").
		ColumnExpr("kg.id::text AS id").
		ColumnExpr("kg.parent_id::text AS parent_id").
		Column("name", "is_default", "sort_order").
		Where("kg.knowledge_base_id = ?", knowledgeBaseID).
		OrderExpr("kg.parent_id NULLS FIRST, kg.sort_order ASC, lower(kg.name) ASC, kg.id ASC").
		Scan(ctx, &flat); err != nil {
		return nil, err
	}
	children := make(map[string][]GroupRecord)
	topLevel := make([]GroupRecord, 0)
	for _, group := range flat {
		group.Children = make([]GroupRecord, 0)
		if group.ParentID == nil {
			topLevel = append(topLevel, group)
			continue
		}
		children[*group.ParentID] = append(children[*group.ParentID], group)
	}
	for index := range topLevel {
		topLevel[index].Children = children[topLevel[index].ID]
		if topLevel[index].Children == nil {
			topLevel[index].Children = make([]GroupRecord, 0)
		}
	}
	return topLevel, nil
}

// loadKnowledgeGroup 读取当前企业知识库中的分组。
func loadKnowledgeGroup(ctx context.Context, db bun.IDB, organizationID, knowledgeBaseID, groupID string) (*servermodels.KnowledgeGroup, error) {
	if !common.ValidUUID(knowledgeBaseID) {
		return nil, ErrNotFound
	}
	if !common.ValidUUID(groupID) {
		return nil, ErrGroupNotFound
	}
	group := &servermodels.KnowledgeGroup{}
	err := db.NewSelect().Model(group).
		Join("JOIN knowledge_bases AS kb ON kb.id = kg.knowledge_base_id").
		Where("kg.id = ?", groupID).
		Where("kg.knowledge_base_id = ?", knowledgeBaseID).
		Where("kb.organization_id = ?", organizationID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupNotFound
	}
	return group, err
}

// recordFromModel 转换知识库存储模型。
func recordFromModel(knowledgeBase servermodels.KnowledgeBase) Record {
	return Record{
		ID: knowledgeBase.ID, Name: knowledgeBase.Name, Category: domain.KnowledgeBaseCategory(knowledgeBase.Category),
		Description: knowledgeBase.Description, Groups: make([]GroupRecord, 0),
		IntegrationConnectionID: stringValue(knowledgeBase.IntegrationConnectionID),
		ExternalResourceID:      stringValue(knowledgeBase.ExternalResourceID),
		CreatedAt:               knowledgeBase.CreatedAt, UpdatedAt: knowledgeBase.UpdatedAt,
	}
}

// optionalString 把空字符串转换为数据库空值。
func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// stringValue 把数据库可空字符串转换为传输字符串。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// isConstraintConflict 判断 PostgreSQL 唯一约束冲突名称。
func isConstraintConflict(err error, constraint string) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "23505" && postgresError.Field('n') == constraint
}

// validateIdentity 校验当前企业用户账号仍可用。
func validateIdentity(ctx context.Context, db bun.IDB, identity *servermodels.Identity) error {
	return identityaction.Validate(ctx, db, identity)
}

// rowsAffectedOne 校验写操作确实命中一行。
func rowsAffectedOne(result sql.Result, notFound error) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return notFound
	}
	return nil
}
