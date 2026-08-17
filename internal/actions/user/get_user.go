//go:build server

package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// GetUserQuery 读取当前企业的内部用户详情。
type GetUserQuery struct {
	db *bun.DB
}

// NewGetUserQuery 创建内部用户详情查询。
func NewGetUserQuery(db *bun.DB) *GetUserQuery {
	return &GetUserQuery{db: db}
}

// Execute 返回当前企业的指定用户。
func (q *GetUserQuery) Execute(ctx context.Context, principal *servermodels.Principal, userID string) (*DirectoryUser, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, ErrNotFound
	}
	user := &DirectoryUser{}
	err := q.db.NewSelect().
		TableExpr("users AS u").
		ColumnExpr("u.id::text AS id").
		Column("email", "display_name", "role", "status", "created_at").
		Where("u.id = ?", userID).
		Where("u.organization_id = ?", principal.Organization.ID).
		Scan(ctx, user)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}
