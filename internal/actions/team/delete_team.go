//go:build server

package team

import (
	"context"
	"fmt"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// DeleteTeamAction 删除企业团队。
type DeleteTeamAction struct {
	db       *bun.DB
	enqueuer servertask.TxEnqueuer
}

// NewDeleteTeamAction 创建团队删除操作。
func NewDeleteTeamAction(db *bun.DB, enqueuer servertask.TxEnqueuer) *DeleteTeamAction {
	return &DeleteTeamAction{db: db, enqueuer: enqueuer}
}

// Execute 删除团队及其成员关系，并把渠道关联和团队队列中的客服处理周期重置到公共队列，为并入公共队列的等待周期投递分配任务。
func (a *DeleteTeamAction) Execute(ctx context.Context, identity *servermodels.Identity, teamID string) error {
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if _, err := loadTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		// 先锁定团队行，与转交给团队的共享锁互斥，队列清理后不会再有新的周期写入该团队。
		if err := lockTeam(ctx, tx, identity.Organization.ID, teamID); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*servermodels.TeamMember)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("team_id = ?", teamID).
			Exec(ctx); err != nil {
			return err
		}
		if err := channelaction.ResetRoutingTarget(ctx, tx, identity.Organization.ID, domain.ChannelRoutingTargetTypeTeam, teamID); err != nil {
			return err
		}
		// 已关闭周期重开后仍读取队列，团队的全部客服处理周期并入公共队列；队列中的周期重新计算队列等待提醒。
		moved := make([]servermodels.ServiceSession, 0)
		if _, err := tx.NewUpdate().Model(&moved).
			Set("team_id = NULL").
			Set("reminded_at = CASE WHEN assignee_identity_id IS NULL THEN NULL ELSE reminded_at END").
			Set("updated_at = now()").
			Where("organization_id = ?", identity.Organization.ID).
			Where("team_id = ?", teamID).
			Returning("id, status, assignee_identity_id").
			Exec(ctx); err != nil {
			return err
		}
		for _, session := range moved {
			if domain.ServiceSessionStatus(session.Status) != domain.ServiceSessionStatusOpen || session.AssigneeIdentityID != nil {
				continue
			}
			if err := serviceassignment.EnqueueAssign(ctx, tx, a.enqueuer, serviceassignment.AssignInput{
				OrganizationID: identity.Organization.ID, ServiceSessionID: session.ID,
			}); err != nil {
				return err
			}
		}
		_, err := tx.NewDelete().Model((*servermodels.Team)(nil)).
			Where("organization_id = ?", identity.Organization.ID).
			Where("id = ?", teamID).
			Exec(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}
