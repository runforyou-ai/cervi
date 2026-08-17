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
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
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

	users, err := useraction.NewListUsersQuery(db).Execute(context.Background(), loginSession.Principal, useraction.ListInput{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if users.Page.Total != 1 || len(users.Users) != 1 || users.Users[0].ID != loginSession.Principal.User.ID || users.Users[0].CreatedAt.IsZero() {
		t.Fatalf("unexpected user directory: %#v", users)
	}
	directoryUser, err := useraction.NewGetUserQuery(db).Execute(context.Background(), loginSession.Principal, loginSession.Principal.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if directoryUser.ID != loginSession.Principal.User.ID || directoryUser.CreatedAt.IsZero() {
		t.Fatalf("unexpected directory user: %#v", directoryUser)
	}

	createContact := contactaction.NewCreateContactAction(db)
	_, err = createContact.Execute(context.Background(), loginSession.Principal, contactaction.ContactInput{
		DisplayName: "无效渠道联系人",
		ChannelID:   "00000000-0000-0000-0000-000000000099",
		Stage:       contactaction.StageVisitor,
	})
	var channelValidation *contactaction.ValidationError
	if !errors.As(err, &channelValidation) || channelValidation.Fields["channelId"] != contactaction.ValidationChannelInvalid {
		t.Fatalf("invalid channel error = %v, want channel validation", err)
	}

	contact, err := createContact.Execute(context.Background(), loginSession.Principal, contactaction.ContactInput{
		DisplayName: "林晓",
		ChannelID:   channel.ID,
		Stage:       contactaction.StageLead,
		Notes:       "采购负责人",
		Methods: []contactaction.MethodInput{
			{Type: contactaction.MethodEmail, Value: "LIN@example.com"},
			{Type: contactaction.MethodPhone, Value: "+86 138-0000-0000"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if contact.Contact.DisplayName == nil || *contact.Contact.DisplayName != "林晓" || contact.Contact.SourceChannelID == nil || *contact.Contact.SourceChannelID != channel.ID || contact.SourceChannel == nil || contact.SourceChannel.ID != channel.ID || len(contact.Methods) != 2 {
		t.Fatalf("unexpected created contact: %#v", contact)
	}

	contactList := contactaction.NewListContactsQuery(db)
	activeContacts, err := contactList.Execute(context.Background(), loginSession.Principal, contactaction.ListInput{
		Query: "lin@example", Stage: contactaction.StageLead, ChannelID: channel.ID, Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if activeContacts.Page.Total != 1 || len(activeContacts.Contacts) != 1 || activeContacts.Contacts[0].PrimaryEmail == nil || activeContacts.Contacts[0].SourceChannelName == nil || *activeContacts.Contacts[0].SourceChannelName != channel.Name || activeContacts.Contacts[0].ChannelCount != 1 {
		t.Fatalf("unexpected contact list: %#v", activeContacts)
	}

	var legacyContactID string
	if err := db.NewRaw(`
		INSERT INTO contacts (organization_id, created_by_user_id, display_name, stage)
		VALUES (?, ?, ?, ?)
		RETURNING id::text
	`, loginSession.Principal.Organization.ID, loginSession.Principal.User.ID, "旧联系人", contactaction.StageVisitor).Scan(context.Background(), &legacyContactID); err != nil {
		t.Fatal(err)
	}
	legacyContact, err := contactaction.NewGetContactQuery(db).Execute(context.Background(), loginSession.Principal, legacyContactID)
	if err != nil {
		t.Fatal(err)
	}
	if legacyContact.Contact.SourceChannelID != nil || legacyContact.SourceChannel != nil {
		t.Fatalf("legacy contact source channel = %#v, want nil", legacyContact.SourceChannel)
	}
	if _, err := contactaction.NewUpdateContactAction(db).Execute(context.Background(), loginSession.Principal, legacyContactID, contactaction.ContactInput{
		DisplayName: "旧联系人（已更新）",
		Stage:       contactaction.StageLead,
	}); err != nil {
		t.Fatalf("update legacy contact: %v", err)
	}

	_, err = contactaction.NewUpdateContactAction(db).Execute(context.Background(), loginSession.Principal, contact.Contact.ID, contactaction.ContactInput{
		DisplayName: "林晓",
		ChannelID:   "00000000-0000-0000-0000-000000000099",
		Stage:       contactaction.StageLead,
		Methods:     []contactaction.MethodInput{{Type: contactaction.MethodEmail, Value: "lin@example.com"}},
	})
	var immutableChannelValidation *contactaction.ValidationError
	if !errors.As(err, &immutableChannelValidation) || immutableChannelValidation.Fields["channelId"] != contactaction.ValidationChannelImmutable {
		t.Fatalf("immutable channel error = %v, want channel validation", err)
	}

	updatedContact, err := contactaction.NewUpdateContactAction(db).Execute(context.Background(), loginSession.Principal, contact.Contact.ID, contactaction.ContactInput{
		DisplayName: "林晓（采购）",
		ChannelID:   channel.ID,
		Stage:       contactaction.StageCustomer,
		Methods:     []contactaction.MethodInput{{Type: contactaction.MethodEmail, Value: "lin@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedContact.Contact.Stage != contactaction.StageCustomer || len(updatedContact.Methods) != 1 {
		t.Fatalf("unexpected updated contact: %#v", updatedContact)
	}

	if err := contactaction.NewDeleteContactAction(db).Execute(context.Background(), loginSession.Principal, contact.Contact.ID); err != nil {
		t.Fatal(err)
	}
	deletedContacts, err := contactList.Execute(context.Background(), loginSession.Principal, contactaction.ListInput{Deleted: true, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if deletedContacts.Page.Total != 1 || len(deletedContacts.Contacts) != 1 {
		t.Fatalf("unexpected deleted contact list: %#v", deletedContacts)
	}
	if _, err := contactaction.NewRestoreContactAction(db).Execute(context.Background(), loginSession.Principal, contact.Contact.ID); err != nil {
		t.Fatal(err)
	}
}
