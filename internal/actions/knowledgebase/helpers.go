//go:build server

package knowledgebase

import (
	"context"
	"database/sql"
	"errors"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/storage/server/pgerr"
	"github.com/uptrace/bun"
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

// validateDifyConnection 校验外部知识库引用的是当前企业的 Dify 连接。
func validateDifyConnection(ctx context.Context, db bun.IDB, organizationID, connectionID string) error {
	if connectionID == "" {
		return nil
	}
	var id string
	err := db.NewSelect().
		TableExpr("integration_connections AS ic").
		ColumnExpr("ic.id::text").
		Where("ic.id = ?", connectionID).
		Where("ic.organization_id = ?", organizationID).
		Where("ic.connector_type = ?", domain.IntegrationConnectionTypeDify).
		For("SHARE").
		Scan(ctx, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return invalidDifyConnectionError()
	}
	return err
}

// loadDifyConfiguration 读取当前企业的 Dify 连接配置。
func loadDifyConfiguration(
	ctx context.Context,
	db bun.IDB,
	organizationID, connectionID string,
) (servermodels.IntegrationConnectionConfiguration, error) {
	if !common.ValidUUID(connectionID) {
		return servermodels.IntegrationConnectionConfiguration{}, invalidDifyConnectionError()
	}
	connection := &servermodels.IntegrationConnection{}
	err := db.NewSelect().
		Model(connection).
		Column("configuration").
		Where("ic.id = ?", connectionID).
		Where("ic.organization_id = ?", organizationID).
		Where("ic.connector_type = ?", domain.IntegrationConnectionTypeDify).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return servermodels.IntegrationConnectionConfiguration{}, invalidDifyConnectionError()
	}
	if err != nil {
		return servermodels.IntegrationConnectionConfiguration{}, err
	}
	return connection.Configuration, nil
}

// invalidDifyConnectionError 返回 Dify 连接字段错误。
func invalidDifyConnectionError() *common.FieldError {
	return &common.FieldError{Fields: map[string]common.FieldCode{
		"integrationConnectionId": ValidationIntegrationConnectionInvalid,
	}}
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
	byBase, err := loadGroupRecordsByBase(ctx, db, []string{knowledgeBaseID})
	if err != nil {
		return nil, err
	}
	groups := byBase[knowledgeBaseID]
	if groups == nil {
		groups = make([]GroupRecord, 0)
	}
	return groups, nil
}

// loadGroupRecordsByBase 一次查询多个知识库的两级分组树，按知识库编号分组返回。
func loadGroupRecordsByBase(ctx context.Context, db bun.IDB, knowledgeBaseIDs []string) (map[string][]GroupRecord, error) {
	byBase := make(map[string][]GroupRecord, len(knowledgeBaseIDs))
	if len(knowledgeBaseIDs) == 0 {
		return byBase, nil
	}
	type groupRow struct {
		GroupRecord
		KnowledgeBaseID string `bun:"knowledge_base_id"`
	}
	flat := make([]groupRow, 0)
	if err := db.NewSelect().
		TableExpr("knowledge_groups AS kg").
		ColumnExpr("kg.id::text AS id").
		ColumnExpr("kg.parent_id::text AS parent_id").
		ColumnExpr("kg.knowledge_base_id::text AS knowledge_base_id").
		Column("name", "is_default", "sort_order").
		Where("kg.knowledge_base_id IN (?)", bun.In(knowledgeBaseIDs)).
		OrderExpr("kg.parent_id NULLS FIRST, kg.sort_order ASC, lower(kg.name) ASC, kg.id ASC").
		Scan(ctx, &flat); err != nil {
		return nil, err
	}
	children := make(map[string][]GroupRecord)
	topLevelByBase := make(map[string][]GroupRecord)
	for _, row := range flat {
		group := row.GroupRecord
		group.Children = make([]GroupRecord, 0)
		if group.ParentID == nil {
			topLevelByBase[row.KnowledgeBaseID] = append(topLevelByBase[row.KnowledgeBaseID], group)
			continue
		}
		children[*group.ParentID] = append(children[*group.ParentID], group)
	}
	for baseID, topLevel := range topLevelByBase {
		for index := range topLevel {
			if nested := children[topLevel[index].ID]; nested != nil {
				topLevel[index].Children = nested
			}
		}
		byBase[baseID] = topLevel
	}
	return byBase, nil
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
		IntegrationConnectionID: common.StringValue(knowledgeBase.IntegrationConnectionID),
		ExternalResourceID:      common.StringValue(knowledgeBase.ExternalResourceID),
		CreatedAt:               knowledgeBase.CreatedAt, UpdatedAt: knowledgeBase.UpdatedAt,
	}
}

// isConstraintConflict 判断 PostgreSQL 唯一约束冲突名称。
func isConstraintConflict(err error, constraint string) bool {
	return pgerr.UniqueViolationOn(err, constraint)
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
