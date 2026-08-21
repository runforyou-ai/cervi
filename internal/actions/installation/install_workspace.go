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

// InstallWorkspaceOutput 返回企业管理员和初始令牌。
type InstallWorkspaceOutput struct {
	Identity  *servermodels.Identity
	Token     string
	ExpiresAt time.Time
}

// NewInstallWorkspaceAction 创建企业初始化操作。
func NewInstallWorkspaceAction(db *bun.DB) *InstallWorkspaceAction {
	return &InstallWorkspaceAction{db: db}
}

// Execute 校验初始化信息并创建企业管理员和登录令牌。
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

		user := &servermodels.User{
			OrganizationID: organization.ID,
			Email:          input.Email,
			DisplayName:    input.DisplayName,
			PasswordHash:   passwordHash,
			RoleID:         adminRoleID,
			Status:         "active",
			Locale:         string(input.Locale),
			TimeZone:       input.TimeZone,
		}
		if _, err := tx.NewInsert().
			Model(user).
			Column("organization_id", "email", "display_name", "password_hash", "role_id", "status", "locale", "time_zone").
			Returning("id, work_status").
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
		identity.User = *user
		return nil
	})
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("install workspace: %w", err)
	}

	return InstallWorkspaceOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
