//go:build server

package server

import (
	"context"
	"strings"
	"testing"
	"uuid"
)

// TestKnowledgeContractionMigration 验证收缩迁移保留本地内容、清理外部映射并支持结构回滚。
func TestKnowledgeContractionMigration(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// 在事务内的独立 schema 重建迁移前结构，避免影响其他集成测试。
	schema := "knowledge_contraction_" + strings.ReplaceAll(uuid.NewV7().String(), "-", "")
	if _, err := tx.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema+", public"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"20260901002048_create_knowledge_bases_table.sql",
		"20260901002049_create_knowledge_groups_table.sql",
		"20260901002051_create_integration_connections_table.sql",
		"20260905164158_create_knowledge_qa_entries_table.sql",
		"20260905164204_create_knowledge_qa_contents_table.sql",
	} {
		data, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, strings.SplitN(string(data), "-- +goose Down", 2)[0]); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO integration_connections (organization_id, connector_type, name, configuration)
		VALUES (uuidv7(), 'dify', '待移除连接', '{}');
		INSERT INTO knowledge_bases (organization_id, created_by_user_id, name, category)
		VALUES (uuidv7(), uuidv7(), '本地知识库', 'qa');
		INSERT INTO knowledge_bases (organization_id, created_by_user_id, name, category, integration_connection_id, external_resource_id)
		SELECT organization_id, uuidv7(), '外部知识库', 'standard', id, 'dataset' FROM integration_connections;
		INSERT INTO knowledge_groups (knowledge_base_id, name) SELECT id, '分组' FROM knowledge_bases;
		INSERT INTO knowledge_qa_entries (knowledge_base_id, group_id, created_by_user_id)
		SELECT kb.id, kg.id, kb.created_by_user_id FROM knowledge_bases kb
		JOIN knowledge_groups kg ON kg.knowledge_base_id = kb.id WHERE kb.name = '本地知识库';
		INSERT INTO knowledge_qa_contents (entry_id, kind, content)
		SELECT id, 'answer', '保留的答案' FROM knowledge_qa_entries;
	`); err != nil {
		t.Fatal(err)
	}
	data, err := migrationFiles.ReadFile("migrations/20260908185523_remove_external_knowledge_connections.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(data), "-- +goose Down", 2)
	if _, err := tx.ExecContext(ctx, parts[0]); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"knowledge_bases", "knowledge_groups", "knowledge_qa_entries", "knowledge_qa_contents"} {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s: count=%d err=%v", table, count, err)
		}
	}
	var answer string
	if err := tx.QueryRowContext(ctx, "SELECT content FROM knowledge_qa_contents").Scan(&answer); err != nil || answer != "保留的答案" {
		t.Fatalf("answer=%q err=%v", answer, err)
	}
	// 回滚恢复表和列后再次升级，确认迁移可往返执行。
	if _, err := tx.ExecContext(ctx, parts[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "SELECT integration_connection_id, external_resource_id FROM knowledge_bases; SELECT * FROM integration_connections"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, parts[0]); err != nil {
		t.Fatal(err)
	}
}
