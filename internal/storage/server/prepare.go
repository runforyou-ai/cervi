//go:build server

package server

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun/driver/pgdriver"
)

//go:embed database/prepare.sql
var databasePreparationSQL string

// prepareDatabase 串行准备当前数据库并校验扩展能力与应用账号权限。
func prepareDatabase(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('cervi:database:prepare', 0))"); err != nil {
		return fmt.Errorf("获取数据库准备锁: %w", err)
	}
	if _, err := tx.ExecContext(ctx, databasePreparationSQL); err != nil {
		// 仅对权限错误提示管理员安装与授权，保留缺扩展文件等错误的原始原因。
		var postgresError pgdriver.Error
		if errors.As(err, &postgresError) && postgresError.Field('C') == "42501" {
			return fmt.Errorf("准备扩展与 schema 权限不足，请管理员对当前目标数据库执行 database/prepare-admin.sql 的安装与授权: %w", err)
		}
		return fmt.Errorf("准备扩展与 schema 失败: %w", err)
	}

	versions := make(map[string]string, 2)
	for _, name := range []string{"vector", "pg_trgm"} {
		var schema, version string
		if err := tx.QueryRowContext(ctx, `SELECT n.nspname, e.extversion FROM pg_extension e
            JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = $1`, name).Scan(&schema, &version); err != nil {
			return fmt.Errorf("读取扩展 %s: %w", name, err)
		}
		if schema != "public" {
			return fmt.Errorf("扩展 %s 位于 %s，必须由管理员在 public schema 准备", name, schema)
		}
		versions[name] = version
	}
	for _, permission := range []struct{ schema, privilege string }{
		{"public", "USAGE"}, {"haystack", "USAGE"}, {"haystack", "CREATE"},
	} {
		var allowed bool
		if err := tx.QueryRowContext(ctx, "SELECT has_schema_privilege(current_user, $1, $2)", permission.schema, permission.privilege).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("当前账号缺少 schema %s 的 %s 权限，请管理员对当前目标数据库执行 database/prepare-admin.sql 授权", permission.schema, permission.privilege)
		}
	}

	// 使用实际表达式验证类型、距离算符和 trigram 函数可见且可执行。
	for _, capability := range []struct{ name, query string }{
		{"vector 类型可见性", "SELECT to_regtype('vector') = 'public.vector'::regtype"},
		{"vector 维度函数", "SELECT vector_dims('[1,2,3]'::vector) = 3"},
		{"vector L2 距离", "SELECT ('[1,0]'::vector <-> '[0,1]'::vector) > 1"},
		{"vector 内积距离", "SELECT ('[1,0]'::vector <#> '[1,0]'::vector) = -1"},
		{"vector 余弦距离", "SELECT ('[1,0]'::vector <=> '[1,0]'::vector) = 0"},
		{"pg_trgm similarity", "SELECT similarity('cervi', 'cervi') = 1"},
		{"pg_trgm show_trgm", "SELECT cardinality(show_trgm('cervi')) > 0"},
	} {
		var available bool
		if err := tx.QueryRowContext(ctx, capability.query).Scan(&available); err != nil {
			return fmt.Errorf("数据库能力 %s 不可用: %w", capability.name, err)
		}
		if !available {
			return fmt.Errorf("数据库能力 %s 校验失败", capability.name)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	slog.Info("PostgreSQL 数据库准备完成", "vector_version", versions["vector"], "pg_trgm_version", versions["pg_trgm"])
	return nil
}
