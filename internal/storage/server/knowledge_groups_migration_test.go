//go:build server

package server

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"uuid"

	"github.com/pressly/goose/v3"
	"github.com/runforyou-ai/cervi/internal/servertest"
	"github.com/uptrace/bun/driver/pgdriver"
)

// TestRemoveKnowledgeGroupsMigration 验证分组迁移保留知识内容并支持空内容表的结构回滚。
func TestRemoveKnowledgeGroupsMigration(t *testing.T) {
	ctx := context.Background()
	db := newKnowledgeMigrationDatabase(t)
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations)
	if err != nil {
		t.Fatal(err)
	}
	const version = int64(20260922072641)
	if _, err := provider.UpTo(ctx, version-1); err != nil {
		t.Fatal(err)
	}

	// 历史结构中同时保存有分组的在线文档、问答和对应正文。
	for _, query := range []string{
		`INSERT INTO knowledge_groups (id, knowledge_base_id, name) VALUES
		 ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', '产品资料')`,
		`INSERT INTO knowledge_documents (id, knowledge_base_id, group_id, source_kind, title, created_by_user_id) VALUES
		 ('00000000-0000-0000-0000-000000000003', '00000000-0000-0000-0000-000000000002',
		  '00000000-0000-0000-0000-000000000001', 'text', '产品说明', '00000000-0000-0000-0000-000000000004')`,
		`INSERT INTO knowledge_document_contents (document_id, content) VALUES
		 ('00000000-0000-0000-0000-000000000003', '# 产品说明')`,
		`INSERT INTO knowledge_qa_entries (id, knowledge_base_id, group_id, created_by_user_id) VALUES
		 ('00000000-0000-0000-0000-000000000005', '00000000-0000-0000-0000-000000000002',
		  '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000004')`,
		`INSERT INTO knowledge_qa_contents (entry_id, kind, content) VALUES
		 ('00000000-0000-0000-0000-000000000005', 'primary_question', '如何使用产品？'),
		 ('00000000-0000-0000-0000-000000000005', 'answer', '请阅读产品说明。')`,
	} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	before := knowledgeMigrationContentSnapshot(t, db)
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatal(err)
	}
	if after := knowledgeMigrationContentSnapshot(t, db); after != before {
		t.Fatalf("knowledge content changed: before %s, after %s", before, after)
	}
	assertKnowledgeGroupSchema(t, db, false)

	// 空内容表允许恢复原有非空分组字段并再次应用删除迁移。
	if _, err := db.ExecContext(ctx, `TRUNCATE knowledge_document_contents, knowledge_documents, knowledge_qa_contents, knowledge_qa_entries`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	assertKnowledgeGroupSchema(t, db, true)
	if _, err := provider.UpTo(ctx, version); err != nil {
		t.Fatal(err)
	}
	assertKnowledgeGroupSchema(t, db, false)
}

// newKnowledgeMigrationDatabase 创建独立临时数据库并注册关闭连接和删除数据库的清理步骤。
func newKnowledgeMigrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	config := servertest.DatabaseConfig(t)
	config.Name = "postgres"
	admin := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(postgresDSN(config))))
	t.Cleanup(func() { _ = admin.Close() })
	config.Name = "cervi_knowledge_" + strings.ReplaceAll(uuid.NewV7().String(), "-", "") + "_test"
	if _, err := admin.ExecContext(context.Background(), `CREATE DATABASE "`+config.Name+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), `DROP DATABASE "`+config.Name+`"`); err != nil {
			t.Errorf("drop temporary database: %v", err)
		}
	})
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(postgresDSN(config))))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// knowledgeMigrationContentSnapshot 读取文档、问答及正文中分组字段之外的完整持久化内容。
func knowledgeMigrationContentSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	var snapshot string
	err := db.QueryRowContext(context.Background(), `SELECT jsonb_build_object(
		'documents', (SELECT jsonb_agg(to_jsonb(d) - 'group_id' ORDER BY id) FROM knowledge_documents d),
		'document_contents', (SELECT jsonb_agg(to_jsonb(c) ORDER BY document_id) FROM knowledge_document_contents c),
		'qa_entries', (SELECT jsonb_agg(to_jsonb(q) - 'group_id' ORDER BY id) FROM knowledge_qa_entries q),
		'qa_contents', (SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM knowledge_qa_contents c)
	)::text`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// assertKnowledgeGroupSchema 校验分组表、唯一索引以及文档和问答的非空分组字段。
func assertKnowledgeGroupSchema(t *testing.T, db *sql.DB, exists bool) {
	t.Helper()
	var tableExists, indexesExist bool
	var columns, requiredColumns int
	err := db.QueryRowContext(context.Background(), `SELECT
		to_regclass('knowledge_groups') IS NOT NULL,
		to_regclass('knowledge_groups_sibling_name_unique') IS NOT NULL
			AND to_regclass('knowledge_groups_default_unique') IS NOT NULL,
		(SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public'
			AND table_name IN ('knowledge_documents', 'knowledge_qa_entries') AND column_name = 'group_id'),
		(SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public'
			AND table_name IN ('knowledge_documents', 'knowledge_qa_entries') AND column_name = 'group_id'
			AND data_type = 'uuid' AND is_nullable = 'NO' AND column_default IS NULL)
	`).Scan(&tableExists, &indexesExist, &columns, &requiredColumns)
	if err != nil {
		t.Fatal(err)
	}
	wantColumns := 0
	if exists {
		wantColumns = 2
	}
	if tableExists != exists || indexesExist != exists || columns != wantColumns || requiredColumns != wantColumns {
		t.Fatalf("group schema = table:%t indexes:%t columns:%d required:%d, want exists:%t columns:%d", tableExists, indexesExist, columns, requiredColumns, exists, wantColumns)
	}
}
