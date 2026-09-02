//go:build server

package auth

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/common/token"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// issueToken 签发登录令牌并返回对应身份。
func issueToken(ctx context.Context, db bun.IDB, organizationID string, userID string) (token.Issued, *servermodels.Identity, error) {
	issued, err := token.Issue()
	if err != nil {
		return token.Issued{}, nil, fmt.Errorf("issue token: %w", err)
	}
	record := &servermodels.Token{
		UserID:    userID,
		TokenHash: issued.TokenHash,
		ExpiresAt: issued.ExpiresAt,
	}
	if _, err := db.NewInsert().
		Model(record).
		Column("user_id", "token_hash", "expires_at").
		Exec(ctx); err != nil {
		return token.Issued{}, nil, fmt.Errorf("save token: %w", err)
	}

	identity, err := resolveIdentity(ctx, db, organizationID, issued.Token)
	if err != nil {
		return token.Issued{}, nil, fmt.Errorf("find token identity: %w", err)
	}
	return issued, identity, nil
}
