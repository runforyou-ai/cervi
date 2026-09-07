//go:build server

package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/uptrace/bun/driver/pgdriver"
)

// newPreparationDatabase 为初始化测试创建独立空库并注册清理。
func newPreparationDatabase(t *testing.T) (serverconfig.DatabaseConfig, *sql.DB) {
	t.Helper()
	config := testDatabaseConfig(t)
	admin := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(postgresDSN(config))))
	config.Name = fmt.Sprintf("prepare_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(context.Background(), "CREATE DATABASE "+config.Name+" TEMPLATE template0"); err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(postgresDSN(config))))
	t.Cleanup(func() {
		_ = db.Close()
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE "+config.Name+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	return config, db
}

// TestDatabasePreparationConcurrent 验证新库并发启动、重复启动和已有业务数据保留。
func TestDatabasePreparationConcurrent(t *testing.T) {
	config, db := newPreparationDatabase(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE TABLE existing_data (value text); INSERT INTO existing_data VALUES ('保留数据')"); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan error, 4)
	for range 4 {
		group.Go(func() {
			store, err := Open(ctx, config)
			if err == nil {
				err = store.Close()
			}
			results <- err
		})
	}
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var value string
	if err := db.QueryRowContext(ctx, "SELECT value FROM existing_data").Scan(&value); err != nil || value != "保留数据" {
		t.Fatalf("existing data = %q, error = %v", value, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pg_extension WHERE extname IN ('vector', 'pg_trgm')").Scan(&count); err != nil || count != 2 {
		t.Fatalf("extensions = %d, error = %v", count, err)
	}
}

// TestDatabasePreparationApplicationRole 验证普通账号、管理员预装和缺失授权的启动行为。
func TestDatabasePreparationApplicationRole(t *testing.T) {
	config, admin := newPreparationDatabase(t)
	ctx := context.Background()
	role := config.Name + "_app"
	if _, err := admin.ExecContext(ctx, "CREATE ROLE "+role+" LOGIN PASSWORD 'prepare-test-password' NOSUPERUSER NOCREATEDB NOCREATEROLE"); err != nil {
		t.Fatal(err)
	}
	// 清理应用账号拥有的测试对象，再由空库夹具删除数据库。
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP OWNED BY "+role); err != nil {
			t.Error(err)
		}
		if _, err := admin.ExecContext(ctx, "DROP ROLE "+role); err != nil {
			t.Error(err)
		}
	})
	if _, err := admin.ExecContext(ctx, "GRANT CREATE ON DATABASE "+config.Name+" TO "+role+"; GRANT USAGE, CREATE ON SCHEMA public TO "+role); err != nil {
		t.Fatal(err)
	}
	config.User, config.Password = role, "prepare-test-password"
	application := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(postgresDSN(config))))
	defer application.Close()
	if err := prepareDatabase(ctx, application); err == nil || !strings.Contains(err.Error(), "prepare-admin.sql") {
		t.Fatalf("missing installation permission = %v", err)
	}
	if err := prepareDatabase(ctx, admin); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(ctx, application); err == nil || !strings.Contains(err.Error(), "haystack") {
		t.Fatalf("missing schema grant = %v", err)
	}
	if _, err := admin.ExecContext(ctx, "GRANT USAGE, CREATE ON SCHEMA haystack TO "+role); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := application.ExecContext(ctx, "CREATE TABLE haystack.documents (embedding vector(3)); INSERT INTO haystack.documents VALUES ('[1,2,3]'); DROP TABLE haystack.documents"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, "REVOKE CREATE ON SCHEMA haystack FROM "+role); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(ctx, application); err == nil || !strings.Contains(err.Error(), "CREATE") {
		t.Fatalf("revoked schema create = %v", err)
	}
}

// TestDatabasePreparationCapabilities 验证扩展位置和实际执行能力的错误信息。
func TestDatabasePreparationCapabilities(t *testing.T) {
	_, db := newPreparationDatabase(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA misplaced; CREATE EXTENSION vector WITH SCHEMA misplaced"); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(ctx, db); err == nil || !strings.Contains(err.Error(), "位于 misplaced") {
		t.Fatalf("misplaced extension = %v", err)
	}
	if _, err := db.ExecContext(ctx, "ALTER EXTENSION vector SET SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "ALTER FUNCTION public.similarity(text,text) RENAME TO unavailable_similarity"); err != nil {
		t.Fatal(err)
	}
	if err := prepareDatabase(ctx, db); err == nil || !strings.Contains(err.Error(), "pg_trgm similarity") {
		t.Fatalf("missing similarity capability = %v", err)
	}
}
