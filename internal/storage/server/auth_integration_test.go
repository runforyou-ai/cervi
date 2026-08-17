//go:build server

package server

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
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

	resolveSession := authaction.NewResolveSessionQuery(db)
	principal, err := resolveSession.Execute(context.Background(), installedSession.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal == nil || principal.User.Email != "owner@example.com" {
		t.Fatalf("unexpected session principal: %#v", principal)
	}

	logout := authaction.NewLogoutAction(db)
	if err := logout.Execute(context.Background(), installedSession.Token); err != nil {
		t.Fatal(err)
	}

	login := authaction.NewLoginAction(db)
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

	createChannel := channelaction.NewCreateWebsiteChannelAction(db)
	stalePrincipal := *loginSession.Principal
	stalePrincipal.User = loginSession.Principal.User
	stalePrincipal.User.ID = "00000000-0000-0000-0000-000000000000"
	_, err = createChannel.Execute(context.Background(), &stalePrincipal, channelaction.WebsiteChannelInput{
		Name:          "无效渠道",
		DefaultLocale: channelaction.LocaleChineseSimplified,
	})
	if !errors.Is(err, channelaction.ErrPrincipalInvalid) {
		t.Fatalf("stale principal error = %v, want ErrPrincipalInvalid", err)
	}

	channel, err := createChannel.Execute(context.Background(), loginSession.Principal, channelaction.WebsiteChannelInput{
		Name:          "产品官网",
		Description:   "接收官网访客咨询",
		DefaultLocale: channelaction.LocaleChineseSimplified,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.Type != channelaction.TypeWebsite || channel.CreatedByUserID != loginSession.Principal.User.ID {
		t.Fatalf("unexpected created channel: %#v", channel)
	}

	updateChannel := channelaction.NewUpdateWebsiteChannelAction(db)
	channel, err = updateChannel.Execute(context.Background(), loginSession.Principal, channel.ID, channelaction.WebsiteChannelInput{
		Name:          "帮助中心",
		DefaultLocale: channelaction.LocaleEnglishUnitedStates,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.Name != "帮助中心" || channel.Description != nil || channel.DefaultLocale != channelaction.LocaleEnglishUnitedStates {
		t.Fatalf("unexpected updated channel: %#v", channel)
	}

	deleteChannel := channelaction.NewDeleteWebsiteChannelAction(db)
	if err := deleteChannel.Execute(context.Background(), loginSession.Principal, channel.ID); err != nil {
		t.Fatal(err)
	}
	listChannels := channelaction.NewListWebsiteChannelsQuery(db)
	activeChannels, err := listChannels.Execute(context.Background(), loginSession.Principal, false)
	if err != nil {
		t.Fatal(err)
	}
	deletedChannels, err := listChannels.Execute(context.Background(), loginSession.Principal, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeChannels) != 0 || len(deletedChannels) != 1 {
		t.Fatalf("active channels = %d, deleted channels = %d", len(activeChannels), len(deletedChannels))
	}

	restoreChannel := channelaction.NewRestoreWebsiteChannelAction(db)
	channel, err = restoreChannel.Execute(context.Background(), loginSession.Principal, channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.DeletedAt != nil {
		t.Fatalf("restored channel deleted_at = %v, want nil", channel.DeletedAt)
	}

	s3Setting := settingaction.S3Setting{
		Enabled:         true,
		Provider:        settingaction.ProviderAWS,
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		Bucket:          "cervi",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	}
	saveS3Setting := settingaction.NewSaveS3SettingAction(db)
	savedS3Setting, err := saveS3Setting.Execute(context.Background(), loginSession.Principal, s3Setting)
	if err != nil {
		t.Fatal(err)
	}
	if savedS3Setting != s3Setting {
		t.Fatalf("saved S3 setting = %#v, want %#v", savedS3Setting, s3Setting)
	}
	getS3Setting := settingaction.NewGetS3SettingQuery(db)
	loadedS3Setting, err := getS3Setting.Execute(context.Background(), loginSession.Principal)
	if err != nil {
		t.Fatal(err)
	}
	if loadedS3Setting != s3Setting {
		t.Fatalf("loaded S3 setting = %#v, want %#v", loadedS3Setting, s3Setting)
	}

	disabledS3Setting := s3Setting
	disabledS3Setting.Enabled = false
	if _, err := saveS3Setting.Execute(context.Background(), loginSession.Principal, disabledS3Setting); err != nil {
		t.Fatal(err)
	}
	loadedS3Setting, err = getS3Setting.Execute(context.Background(), loginSession.Principal)
	if err != nil {
		t.Fatal(err)
	}
	if loadedS3Setting != disabledS3Setting {
		t.Fatalf("disabled S3 setting = %#v, want preserved setting %#v", loadedS3Setting, disabledS3Setting)
	}
}
