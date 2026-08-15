//go:build server

package server

import (
	"context"
	"os"
	"testing"
	"time"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	commonsession "github.com/runforyou-ai/cervi/internal/common/session"
)

// TestAuthenticationActionsWithPostgreSQL 验证 PostgreSQL 上的初始化和认证流程。
func TestAuthenticationActionsWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	store, err := Open(context.Background(), Config{
		DSN:             dsn,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
		StartupTimeout:  30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	db := store.DB()
	status := installationaction.NewStatusQuery(db)
	installed, err := status.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("fresh database is already installed")
	}

	install := installationaction.NewInstallWorkspaceAction(db)
	installedSession, err := install.Execute(context.Background(), installationaction.InstallWorkspaceInput{
		OrganizationName: "鹿行测试公司",
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installedSession.Principal.User.Role != "owner" || installedSession.Principal.Organization.Name != "鹿行测试公司" {
		t.Fatalf("unexpected principal: %#v", installedSession.Principal)
	}

	sessions := commonsession.NewManager(db)
	principal, err := sessions.Resolve(context.Background(), installedSession.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal == nil || principal.User.Email != "owner@example.com" {
		t.Fatalf("unexpected session principal: %#v", principal)
	}

	logout := authaction.NewLogoutAction(sessions)
	if err := logout.Execute(context.Background(), installedSession.Token); err != nil {
		t.Fatal(err)
	}

	login := authaction.NewLoginAction(db, sessions)
	loginSession, err := login.Execute(context.Background(), authaction.LoginInput{
		Email:    "OWNER@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if loginSession.Principal.User.ID != installedSession.Principal.User.ID {
		t.Fatalf("login user = %q, want %q", loginSession.Principal.User.ID, installedSession.Principal.User.ID)
	}
}
