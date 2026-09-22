//go:build server

// Package installation 实现企业初始化领域的应用操作。
package installation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	commonemail "github.com/runforyou-ai/cervi/internal/common/email"
	commonpassword "github.com/runforyou-ai/cervi/internal/common/password"
	"github.com/runforyou-ai/cervi/internal/common/token"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrAlreadyInstalled  = errors.New("workspace is already installed")
	ErrAccessHostMissing = errors.New("workspace access host is missing")
)

// InstallWorkspaceAction 执行企业初始化操作。
type InstallWorkspaceAction struct {
	db *bun.DB
}

// InstallWorkspaceInput 定义企业初始化输入。
type InstallWorkspaceInput struct {
	AccessHost       string
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
	input.AccessHost = tenant.NormalizeAccessHost(input.AccessHost)
	if input.AccessHost == "" {
		return InstallWorkspaceOutput{}, ErrAccessHostMissing
	}
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

	var identity *servermodels.Identity
	err = a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		created, err := organizationaction.Create(ctx, tx, organizationaction.CreateInput{
			AccessHost:        input.AccessHost,
			Name:              input.OrganizationName,
			AdminDisplayName:  input.DisplayName,
			AdminEmail:        input.Email,
			AdminPasswordHash: passwordHash,
			Locale:            input.Locale,
			TimeZone:          input.TimeZone,
		})
		if errors.Is(err, organizationaction.ErrAccessHostTaken) {
			return ErrAlreadyInstalled
		}
		if err != nil {
			return err
		}
		record := &servermodels.Token{
			UserID:    created.User.ID,
			TokenHash: issued.TokenHash,
			ExpiresAt: issued.ExpiresAt,
		}
		if _, err := tx.NewInsert().
			Model(record).
			Column("user_id", "token_hash", "expires_at").
			Exec(ctx); err != nil {
			return err
		}
		identity = created
		return nil
	})
	if err != nil {
		return InstallWorkspaceOutput{}, fmt.Errorf("install workspace: %w", err)
	}

	return InstallWorkspaceOutput{Identity: identity, Token: issued.Token, ExpiresAt: issued.ExpiresAt}, nil
}
