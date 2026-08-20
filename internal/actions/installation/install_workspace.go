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
	"github.com/runforyou-ai/cervi/internal/common/sessiontoken"
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
}

// InstallWorkspaceOutput 返回企业所有者和初始会话。
type InstallWorkspaceOutput struct {
	Identity  *servermodels.Identity
	Token     string
	ExpiresAt time.Time
}

// NewInstallWorkspaceAction 创建企业初始化操作。
func NewInstallWorkspaceAction(db *bun.DB) *InstallWorkspaceAction {
	return &InstallWorkspaceAction{db: db}
}

// Execute 校验初始化信息并创建企业所有者和会话。
func (a *InstallWorkspaceAction) Execute(ctx context.Context, input InstallWorkspaceInput) (InstallWorkspaceOutput, error) {
	input.OrganizationName = strings.TrimSpace(input.OrganizationName)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = commonemail.Normalize(input.Email)
	if fields := validateInput(input); len(fields) > 0 {
		return InstallWorkspaceOutput{}, &ValidationError{Fields: fields}
	}

	passwordHash, err := commonpassword.Hash(input.Password)
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("hash owner password: %w", err)
	}
	issued, err := sessiontoken.Issue()
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("create installation session: %w", err)
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

		user := &servermodels.User{
			OrganizationID: organization.ID,
			Email:          input.Email,
			DisplayName:    input.DisplayName,
			PasswordHash:   passwordHash,
			Role:           "owner",
			Status:         "active",
		}
		if _, err := tx.NewInsert().
			Model(user).
			Column("organization_id", "email", "display_name", "password_hash", "role", "status").
			Returning("id").
			Exec(ctx); err != nil {
			return err
		}

		record := &servermodels.Session{
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
