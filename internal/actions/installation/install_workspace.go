//go:build server

// Package installation 实现企业初始化领域的应用操作。
package installation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/common/token"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

var ErrAlreadyInstalled = errors.New("workspace is already installed")

// InstallWorkspaceAction 执行企业初始化操作。
type InstallWorkspaceAction struct {
	db *bun.DB
}

// InstallWorkspaceInput 定义企业初始化输入。
type InstallWorkspaceInput struct {
	OrganizationName string
	DisplayName      string
	Email            string
	Password         string
	Locale           domain.Locale
	TimeZone         string
}

// InstallWorkspaceOutput 返回企业管理员、默认团队和初始令牌。
type InstallWorkspaceOutput struct {
	Identity      *servermodels.Identity
	DefaultTeamID string
	Token         string
	ExpiresAt     time.Time
}

// NewInstallWorkspaceAction 创建企业初始化操作。
func NewInstallWorkspaceAction(db *bun.DB) *InstallWorkspaceAction {
	return &InstallWorkspaceAction{db: db}
}

// Execute 校验初始化信息并创建企业管理员、默认团队和登录令牌。
func (a *InstallWorkspaceAction) Execute(ctx context.Context, input InstallWorkspaceInput) (InstallWorkspaceOutput, error) {
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = commonemail.Normalize(input.Email)
	if fields := validateInput(input); len(fields) > 0 {
		return InstallWorkspaceOutput{}, &ValidationError{Fields: fields}
	}

	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("hash administrator password: %w", err)
	}
	issued, err := token.Issue()
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("issue installation token: %w", err)
	}

	identity := &servermodels.Identity{}
	var defaultTeamID string
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "LOCK TABLE organizations IN EXCLUSIVE MODE"); err != nil {
			return err
		}

		installed, err := tx.NewSelect().Model((*servermodels.Organization)(nil)).Exists(ctx)
		if err != nil {
			return err
		}
		if installed {
			return ErrAlreadyInstalled
		}

		organization := &servermodels.Organization{Name: input.OrganizationName}
		if _, err := tx.NewInsert().
			Model(organization).
			Column("name").
			Returning("id").
			Exec(ctx); err != nil {
			return err
		}

		var adminRoleID string
		for _, kind := range domain.BuiltInRoleKinds() {
			role := &servermodels.Role{OrganizationID: organization.ID, Kind: string(kind)}
			if _, err := tx.NewInsert().
				Model(role).
				Column("organization_id", "kind").
				Returning("id").
				Exec(ctx); err != nil {
				return err
			}
			if kind == domain.RoleKindAdmin {
				adminRoleID = role.ID
			}
			permissions := domain.DefaultRolePermissions(kind)
			if len(permissions) == 0 {
				continue
			}
			records := make([]servermodels.RolePermission, 0, len(permissions))
			for _, permission := range permissions {
				records = append(records, servermodels.RolePermission{
					OrganizationID: organization.ID,
					RoleID:         role.ID,
					Permission:     string(permission),
				})
			}
			if _, err := tx.NewInsert().
				Model(&records).
				Column("organization_id", "role_id", "permission").
				Exec(ctx); err != nil {
				return err
			}
		}

		organizationIdentity := &servermodels.OrganizationIdentity{
			OrganizationID: organization.ID,
			Type:           string(domain.OrganizationIdentityTypeUser),
			DisplayName:    input.DisplayName,
			WorkStatus:     string(domain.WorkStatusWorking),
		}
		if _, err := tx.NewInsert().Model(organizationIdentity).
			Column("organization_id", "type", "display_name", "work_status").
			Returning("id, work_status, work_status_updated_at").Exec(ctx); err != nil {
			return err
		}
		user := &servermodels.User{
			IdentityID:                  organizationIdentity.ID,
			OrganizationID:              organization.ID,
			Email:                       input.Email,
			PasswordHash:                passwordHash,
			RoleID:                      adminRoleID,
			Status:                      string(domain.UserStatusActive),
			Locale:                      string(input.Locale),
			TimeZone:                    input.TimeZone,
			MessageNotificationsEnabled: true,
		}
		if _, err := tx.NewInsert().
			Model(user).
			Column("identity_id", "organization_id", "email", "password_hash", "role_id", "status", "locale", "time_zone", "message_notifications_enabled").
			Returning("id").
			Exec(ctx); err != nil {
			return err
		}
		teamName := "Customer Service Team"
		if input.Locale == domain.LocaleChineseSimplified {
			teamName = "客户服务团队"
		}
		team := &servermodels.Team{
			OrganizationID:  organization.ID,
			Name:            teamName,
			CreatedByUserID: user.ID,
		}
		if _, err := tx.NewInsert().
			Model(team).
			Column("organization_id", "name", "created_by_user_id").
			Returning("id").
			Exec(ctx); err != nil {
			return err
		}
		teamMember := &servermodels.TeamMember{
			OrganizationID:  organization.ID,
			TeamID:          team.ID,
			IdentityID:      user.IdentityID,
			CreatedByUserID: user.ID,
		}
		if _, err := tx.NewInsert().Model(teamMember).
			Column("organization_id", "team_id", "identity_id", "created_by_user_id").
			Exec(ctx); err != nil {
			return err
		}

		record := &servermodels.Token{
			UserID:    user.ID,
			TokenHash: issued.TokenHash,
			ExpiresAt: issued.ExpiresAt,
		}
		if _, err := tx.NewInsert().
			Model(record).
			Column("user_id", "token_hash", "expires_at").
			Exec(ctx); err != nil {
			return err
		}

		identity.Organization = *organization
		identity.OrganizationIdentity = *organizationIdentity
		identity.User = *user
		defaultTeamID = team.ID
		return nil
	})
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("install workspace: %w", err)
	}

	return InstallWorkspaceOutput{Identity: identity, DefaultTeamID: defaultTeamID, Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
