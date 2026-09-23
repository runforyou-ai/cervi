//go:build server

package user

import (
	"context"
	"fmt"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/realtime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// CreateUserAction 创建企业成员账号。
type CreateUserAction struct {
	db       *bun.DB
	enqueuer servertask.TxEnqueuer
}

// NewCreateUserAction 创建企业成员新增操作。
func NewCreateUserAction(db *bun.DB, enqueuer servertask.TxEnqueuer) *CreateUserAction {
	return &CreateUserAction{db: db, enqueuer: enqueuer}
}

// Execute 校验并创建企业成员及其团队关系，开启接待的成员随即从所在队列补分配。
func (a *CreateUserAction) Execute(ctx context.Context, identity *servermodels.Identity, input CreateInput) (*User, error) {
	input, fields := normalizeCreateInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash member password: %w", err)
	}
	var output *User
	err = realtime.RunInTx(ctx, a.db, func(ctx context.Context, tx bun.Tx) error {
		if err := identityaction.LockActiveUser(ctx, tx, identity); err != nil {
			return err
		}
		if err := validateRoleID(ctx, tx, identity.Organization.ID, input.RoleID); err != nil {
			return err
		}
		teamIDs, err := validateTeamIDs(ctx, tx, identity.Organization.ID, input.TeamIDs)
		if err != nil {
			return err
		}
		organizationIdentity := &servermodels.OrganizationIdentity{
			OrganizationID:   identity.Organization.ID,
			Type:             string(domain.OrganizationIdentityTypeUser),
			DisplayName:      input.DisplayName,
			HandlesCustomers: input.HandlesCustomers,
			WorkStatus:       string(domain.WorkStatusWorking),
		}
		// 传入头像时激活已上传的图片并随身份一起写入。
		if input.AvatarFileID != "" {
			avatarFileID, err := fileaction.ActivateLinkedImage(ctx, tx, identity.Organization.ID, domain.FilePurposeUserAvatar, input.AvatarFileID, nil)
			if err != nil {
				return err
			}
			organizationIdentity.AvatarFileID = avatarFileID
		}
		_, err = tx.NewInsert().Model(organizationIdentity).
			Column("organization_id", "type", "display_name", "avatar_file_id", "handles_customers", "work_status").Returning("id").Exec(ctx)
		if err != nil {
			return err
		}
		user := &servermodels.User{
			IdentityID:         organizationIdentity.ID,
			OrganizationID:     identity.Organization.ID,
			RoleID:             input.RoleID,
			Email:              input.Email,
			PasswordHash:       passwordHash,
			Status:             string(domain.UserStatusActive),
			Locale:             identity.User.Locale,
			TimeZone:           identity.User.TimeZone,
			MaxServiceSessions: input.MaxServiceSessions,
		}
		_, err = tx.NewInsert().Model(user).
			Column("identity_id", "organization_id", "role_id", "email", "password_hash", "status", "locale", "time_zone", "max_service_sessions").Returning("id").Exec(ctx)
		if isUniqueViolation(err) {
			return &ValidationError{Fields: map[string]ValidationCode{"email": ValidationEmailDuplicate}}
		}
		if err != nil {
			return err
		}
		if err := teamaction.ReplaceIdentityTeams(ctx, tx, identity, user.IdentityID, teamIDs); err != nil {
			return err
		}
		if input.HandlesCustomers {
			if err := serviceassignment.EnqueueBackfill(ctx, tx, a.enqueuer, serviceassignment.BackfillInput{OrganizationID: identity.Organization.ID, IdentityID: user.IdentityID}); err != nil {
				return err
			}
		}
		output, err = loadUser(ctx, tx, identity.Organization.ID, user.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return output, nil
}
