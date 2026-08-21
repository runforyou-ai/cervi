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
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestServerActionsWithPostgreSQL 验证服务端核心操作。
func TestServerActionsWithPostgreSQL(t *testing.T) {
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
	alreadyInstalled, err := status.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if alreadyInstalled.Installed {
		t.Fatal("fresh database is already installed")
	}

	install := installationaction.NewInstallWorkspaceAction(db)
	installed, err := install.Execute(context.Background(), installationaction.InstallWorkspaceInput{
		OrganizationName: "鹿行测试公司",
		DisplayName:      "所有者",
		Email:            "owner@example.com",
		Password:         "password123",
		Locale:           domain.LocaleEnglishUnitedStates,
		TimeZone:         "America/New_York",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Identity.User.Role != "owner" || installed.Identity.Organization.Name != "鹿行测试公司" || installed.Identity.User.Locale != "en-US" || installed.Identity.User.TimeZone != "America/New_York" || installed.Identity.User.WorkStatus != string(domain.WorkStatusWorking) {
		t.Fatalf("unexpected identity: %#v", installed.Identity)
	}
	currentStatus, err := status.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !currentStatus.Installed || currentStatus.OrganizationName != "鹿行测试公司" {
		t.Fatalf("status = %#v", currentStatus)
	}

	resolveIdentity := authaction.NewResolveIdentityQuery(db)
	identity, err := resolveIdentity.Execute(context.Background(), installed.Token)
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || identity.User.Email != "owner@example.com" {
		t.Fatalf("unexpected identity: %#v", identity)
	}

	logout := authaction.NewLogoutAction(db)
	if err := logout.Execute(context.Background(), installed.Token); err != nil {
		t.Fatal(err)
	}

	login := authaction.NewLoginAction(db)
	loggedIn, err := login.Execute(context.Background(), authaction.LoginInput{
		Email:    "OWNER@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.Identity.User.ID != installed.Identity.User.ID {
		t.Fatalf("login user = %q, want %q", loggedIn.Identity.User.ID, installed.Identity.User.ID)
	}

	createChannel := channelaction.NewCreateWebsiteChannelAction(db)
	staleIdentity := *loggedIn.Identity
	staleIdentity.User = loggedIn.Identity.User
	staleIdentity.User.ID = "00000000-0000-0000-0000-000000000000"
	_, err = createChannel.Execute(context.Background(), &staleIdentity, channelaction.WebsiteChannelInput{
		Name:          "无效渠道",
		DefaultLocale: domain.LocaleChineseSimplified,
	})
	if !errors.Is(err, common.ErrIdentityInvalid) {
		t.Fatalf("stale identity error = %v, want %v", err, common.ErrIdentityInvalid)
	}

	channel, err := createChannel.Execute(context.Background(), loggedIn.Identity, channelaction.WebsiteChannelInput{
		Name:          "产品官网",
		Description:   "接收官网访客咨询",
		DefaultLocale: domain.LocaleChineseSimplified,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.Type != string(domain.ChannelTypeWebsite) || channel.CreatedByUserID != loggedIn.Identity.User.ID {
		t.Fatalf("unexpected created channel: %#v", channel)
	}

	getChannel := channelaction.NewGetWebsiteChannelQuery(db)
	detail, err := getChannel.Execute(context.Background(), loggedIn.Identity, channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ChatInterface.ChatTitle != "产品官网" || detail.ChatInterface.ThemeColor != channelaction.DefaultWebsiteChannelThemeColor {
		t.Fatalf("unexpected default chat interface: %#v", detail.ChatInterface)
	}

	updateChatInterface := channelaction.NewUpdateWebsiteChannelChatInterfaceAction(db)
	chatInterface, err := updateChatInterface.Execute(context.Background(), loggedIn.Identity, channel.ID, channelaction.WebsiteChannelChatInterfaceInput{
		Title:           "在线咨询",
		Subtitle:        "通常会很快回复",
		GreetingMessage: "你好，有什么可以帮你？",
		ThemeColor:      "#16a34a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if chatInterface.ChatTitle != "在线咨询" || chatInterface.ChatSubtitle == nil || *chatInterface.ChatSubtitle != "通常会很快回复" || chatInterface.ThemeColor != "#16A34A" {
		t.Fatalf("unexpected updated chat interface: %#v", chatInterface)
	}

	updateChannel := channelaction.NewUpdateWebsiteChannelAction(db)
	channel, err = updateChannel.Execute(context.Background(), loggedIn.Identity, channel.ID, channelaction.WebsiteChannelInput{
		Name:          "帮助中心",
		DefaultLocale: domain.LocaleEnglishUnitedStates,
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.Name != "帮助中心" || channel.Description != nil || channel.DefaultLocale != string(domain.LocaleEnglishUnitedStates) {
		t.Fatalf("unexpected updated channel: %#v", channel)
	}

	deleteChannel := channelaction.NewDeleteWebsiteChannelAction(db)
	if err := deleteChannel.Execute(context.Background(), loggedIn.Identity, channel.ID); err != nil {
		t.Fatal(err)
	}
	listChannels := channelaction.NewListWebsiteChannelsQuery(db)
	activeChannels, err := listChannels.Execute(context.Background(), loggedIn.Identity, false)
	if err != nil {
		t.Fatal(err)
	}
	deletedChannels, err := listChannels.Execute(context.Background(), loggedIn.Identity, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeChannels) != 0 || len(deletedChannels) != 1 {
		t.Fatalf("active channels = %d, deleted channels = %d", len(activeChannels), len(deletedChannels))
	}

	restoreChannel := channelaction.NewRestoreWebsiteChannelAction(db)
	channel, err = restoreChannel.Execute(context.Background(), loggedIn.Identity, channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.DeletedAt != nil {
		t.Fatalf("restored channel deleted_at = %v, want nil", channel.DeletedAt)
	}

	users, err := useraction.NewListUsersQuery(db).Execute(context.Background(), loggedIn.Identity, useraction.ListInput{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if users.Page.Total != 1 || len(users.Users) != 1 || users.Users[0].ID != loggedIn.Identity.User.ID || users.Users[0].CreatedAt.IsZero() {
		t.Fatalf("unexpected user directory: %#v", users)
	}
	directoryUser, err := useraction.NewGetUserQuery(db).Execute(context.Background(), loggedIn.Identity, loggedIn.Identity.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if directoryUser.ID != loggedIn.Identity.User.ID || directoryUser.CreatedAt.IsZero() {
		t.Fatalf("unexpected directory user: %#v", directoryUser)
	}

	updateProfile := useraction.NewUpdateProfileAction(db)
	updatedUser, err := updateProfile.Execute(context.Background(), loggedIn.Identity, useraction.ProfileInput{
		DisplayName: "  新姓名  ",
		Email:       " NEW@Example.com ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedUser.DisplayName != "新姓名" || updatedUser.Email != "new@example.com" {
		t.Fatalf("updated user = %#v", updatedUser)
	}
	resolvedAfterUpdate, err := resolveIdentity.Execute(context.Background(), loggedIn.Token)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedAfterUpdate == nil || resolvedAfterUpdate.User.Email != "new@example.com" || resolvedAfterUpdate.User.DisplayName != "新姓名" {
		t.Fatalf("identity after profile update = %#v", resolvedAfterUpdate)
	}

	changePassword := useraction.NewChangePasswordAction(db)
	err = changePassword.Execute(context.Background(), resolvedAfterUpdate, useraction.ChangePasswordInput{
		CurrentPassword: "incorrect-password",
		NewPassword:     "new-password123",
	})
	var passwordValidation *useraction.ValidationError
	if !errors.As(err, &passwordValidation) || passwordValidation.Fields["currentPassword"] != useraction.ValidationCurrentPasswordIncorrect {
		t.Fatalf("incorrect current password error = %v, want current password validation", err)
	}
	if err := changePassword.Execute(context.Background(), resolvedAfterUpdate, useraction.ChangePasswordInput{
		CurrentPassword: "password123",
		NewPassword:     "new-password123",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := login.Execute(context.Background(), authaction.LoginInput{Email: "new@example.com", Password: "password123"}); !errors.Is(err, authaction.ErrInvalidCredentials) {
		t.Fatalf("old password login error = %v, want invalid credentials", err)
	}
	if _, err := login.Execute(context.Background(), authaction.LoginInput{Email: "new@example.com", Password: "new-password123"}); err != nil {
		t.Fatalf("new password login error = %v", err)
	}

	otherUser := &servermodels.User{
		OrganizationID: loggedIn.Identity.Organization.ID,
		Email:          "other@example.com",
		DisplayName:    "其他成员",
		PasswordHash:   "unused",
		Role:           string(domain.UserRoleMember),
		Status:         string(domain.UserStatusActive),
	}
	if _, err := db.NewInsert().Model(otherUser).
		Column("organization_id", "email", "display_name", "password_hash", "role", "status").
		Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
		DisplayName: "新姓名",
		Email:       "OTHER@example.com",
	})
	var profileValidation *useraction.ValidationError
	if !errors.As(err, &profileValidation) || profileValidation.Fields["email"] != useraction.ValidationEmailDuplicate {
		t.Fatalf("duplicate profile email error = %v, want email validation", err)
	}

	createContact := contactaction.NewCreateContactAction(db)
	_, err = createContact.Execute(context.Background(), loggedIn.Identity, contactaction.ContactInput{
		DisplayName: "无效渠道联系人",
		ChannelID:   "00000000-0000-0000-0000-000000000099",
		Stage:       domain.ContactStageVisitor,
	})
	var channelValidation *contactaction.ValidationError
	if !errors.As(err, &channelValidation) || channelValidation.Fields["channelId"] != contactaction.ValidationChannelInvalid {
		t.Fatalf("invalid channel error = %v, want channel validation", err)
	}

	contact, err := createContact.Execute(context.Background(), loggedIn.Identity, contactaction.ContactInput{
		DisplayName: "林晓",
		ChannelID:   channel.ID,
		Stage:       domain.ContactStageLead,
		Notes:       "采购负责人",
		Methods: []contactaction.MethodInput{
			{Type: domain.ContactMethodTypeEmail, Value: "LIN@example.com", Label: "工作"},
			{Type: domain.ContactMethodTypeEmail, Value: "lin.private@example.com", Label: "私人"},
			{Type: domain.ContactMethodTypePhone, Value: "+86 138-0000-0000", Label: "手机"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if contact.Contact.DisplayName == nil || *contact.Contact.DisplayName != "林晓" || contact.Contact.SourceChannelID != channel.ID || contact.SourceChannel.ID != channel.ID || len(contact.Methods) != 3 {
		t.Fatalf("unexpected created contact: %#v", contact)
	}
	type methodIdentity struct {
		typeName string
		value    string
	}
	storedMethods := make([]servermodels.ContactMethod, 0)
	if err := db.NewSelect().
		Model(&storedMethods).
		Where("cm.organization_id = ?", loggedIn.Identity.Organization.ID).
		Where("cm.contact_id = ?", contact.Contact.ID).
		Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	preservedMethods := make(map[methodIdentity]servermodels.ContactMethod, len(storedMethods))
	for _, method := range storedMethods {
		preservedMethods[methodIdentity{typeName: method.Type, value: method.Value}] = method
	}
	preservedInputs := make([]contactaction.MethodInput, 0, len(contact.Methods))
	for _, method := range contact.Methods {
		label := ""
		if method.Label != nil {
			label = *method.Label
		}
		preservedInputs = append(preservedInputs, contactaction.MethodInput{
			Type: method.Type, Value: method.Value, Label: label, IsPrimary: method.IsPrimary,
		})
	}
	preservedContact, err := contactaction.NewUpdateContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID, contactaction.ContactInput{
		DisplayName: "林晓（已确认）",
		ChannelID:   channel.ID,
		Stage:       domain.ContactStageLead,
		Notes:       "采购负责人",
		Methods:     preservedInputs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preservedContact.Methods) != len(contact.Methods) {
		t.Fatalf("preserved methods count = %d, want %d", len(preservedContact.Methods), len(contact.Methods))
	}
	afterMethods := make([]servermodels.ContactMethod, 0)
	if err := db.NewSelect().
		Model(&afterMethods).
		Where("cm.organization_id = ?", loggedIn.Identity.Organization.ID).
		Where("cm.contact_id = ?", contact.Contact.ID).
		Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, method := range afterMethods {
		before, ok := preservedMethods[methodIdentity{typeName: method.Type, value: method.Value}]
		labelsEqual := (method.Label == nil && before.Label == nil) ||
			(method.Label != nil && before.Label != nil && *method.Label == *before.Label)
		if !ok || method.ID != before.ID || !labelsEqual || method.IsPrimary != before.IsPrimary || !method.CreatedAt.Equal(before.CreatedAt) || !method.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("contact method was recreated or changed: before=%#v after=%#v", before, method)
		}
	}

	contactList := contactaction.NewListContactsQuery(db)
	activeContacts, err := contactList.Execute(context.Background(), loggedIn.Identity, contactaction.ListInput{
		Query: "lin@example", Stage: domain.ContactStageLead, ChannelID: channel.ID, Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if activeContacts.Page.Total != 1 || len(activeContacts.Contacts) != 1 || activeContacts.Contacts[0].PrimaryEmail == nil || activeContacts.Contacts[0].SourceChannelName != channel.Name {
		t.Fatalf("unexpected contact list: %#v", activeContacts)
	}

	_, err = contactaction.NewUpdateContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID, contactaction.ContactInput{
		DisplayName: "林晓",
		ChannelID:   "00000000-0000-0000-0000-000000000099",
		Stage:       domain.ContactStageLead,
		Methods:     []contactaction.MethodInput{{Type: domain.ContactMethodTypeEmail, Value: "lin@example.com"}},
	})
	var immutableChannelValidation *contactaction.ValidationError
	if !errors.As(err, &immutableChannelValidation) || immutableChannelValidation.Fields["channelId"] != contactaction.ValidationChannelImmutable {
		t.Fatalf("immutable channel error = %v, want channel validation", err)
	}

	updatedContact, err := contactaction.NewUpdateContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID, contactaction.ContactInput{
		DisplayName: "林晓（采购）",
		ChannelID:   channel.ID,
		Stage:       domain.ContactStageCustomer,
		Methods:     []contactaction.MethodInput{{Type: domain.ContactMethodTypeEmail, Value: "lin@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedContact.Contact.Stage != domain.ContactStageCustomer || len(updatedContact.Methods) != 1 {
		t.Fatalf("unexpected updated contact: %#v", updatedContact)
	}

	if _, err := db.NewUpdate().Table("users").Set("status = 'inactive'").Where("id = ?", loggedIn.Identity.User.ID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := contactaction.NewDeleteContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID); !errors.Is(err, common.ErrIdentityInvalid) {
		t.Fatalf("inactive user delete error = %v, want %v", err, common.ErrIdentityInvalid)
	}
	if _, err := db.NewUpdate().Table("users").Set("status = 'active'").Where("id = ?", loggedIn.Identity.User.ID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := contactaction.NewDeleteContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID); err != nil {
		t.Fatal(err)
	}
	deletedContacts, err := contactList.Execute(context.Background(), loggedIn.Identity, contactaction.ListInput{Deleted: true, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if deletedContacts.Page.Total != 1 || len(deletedContacts.Contacts) != 1 {
		t.Fatalf("unexpected deleted contact list: %#v", deletedContacts)
	}
	if _, err := db.NewUpdate().Table("users").Set("status = 'inactive'").Where("id = ?", loggedIn.Identity.User.ID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := contactaction.NewRestoreContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID); !errors.Is(err, common.ErrIdentityInvalid) {
		t.Fatalf("inactive user restore error = %v, want %v", err, common.ErrIdentityInvalid)
	}
	if _, err := db.NewUpdate().Table("users").Set("status = 'active'").Where("id = ?", loggedIn.Identity.User.ID).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := contactaction.NewRestoreContactAction(db).Execute(context.Background(), loggedIn.Identity, contact.Contact.ID); err != nil {
		t.Fatal(err)
	}

	s3Setting := settingaction.S3Setting{
		Enabled:         true,
		Provider:        domain.StorageProviderAWS,
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		Bucket:          "cervi",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	}
	saveS3Setting := settingaction.NewSaveS3SettingAction(db)
	savedS3Setting, err := saveS3Setting.Execute(context.Background(), loggedIn.Identity, s3Setting)
	if err != nil {
		t.Fatal(err)
	}
	if savedS3Setting != s3Setting {
		t.Fatalf("saved S3 setting = %#v, want %#v", savedS3Setting, s3Setting)
	}
	getS3Setting := settingaction.NewGetS3SettingQuery(db)
	loadedS3Setting, err := getS3Setting.Execute(context.Background(), loggedIn.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if loadedS3Setting != s3Setting {
		t.Fatalf("loaded S3 setting = %#v, want %#v", loadedS3Setting, s3Setting)
	}

	disabledS3Setting := s3Setting
	disabledS3Setting.Enabled = false
	if _, err := saveS3Setting.Execute(context.Background(), loggedIn.Identity, disabledS3Setting); err != nil {
		t.Fatal(err)
	}
	loadedS3Setting, err = getS3Setting.Execute(context.Background(), loggedIn.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if loadedS3Setting != disabledS3Setting {
		t.Fatalf("disabled S3 setting = %#v, want preserved setting %#v", loadedS3Setting, disabledS3Setting)
	}
}
