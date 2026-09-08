//go:build server

package conversation

import (
	"context"
	"slices"
	"uuid"

	"github.com/runforyou-ai/cervi/internal/actions/chatstate"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// ensureOrganizationIdentityChatSubjects 按身份编号升序取得共享主体，结果按身份编号索引。
func ensureOrganizationIdentityChatSubjects(ctx context.Context, db bun.IDB, organizationID string, identityIDs []string) (map[string]*servermodels.ChatSubject, error) {
	ordered := slices.Clone(identityIDs)
	slices.Sort(ordered)
	subjects := make(map[string]*servermodels.ChatSubject, len(ordered))
	for _, identityID := range slices.Compact(ordered) {
		subject, err := chatstate.EnsureOrganizationIdentityChatSubject(ctx, db, organizationID, identityID, uuid.NewV7().String())
		if err != nil {
			return nil, err
		}
		subjects[identityID] = subject
	}
	return subjects, nil
}
