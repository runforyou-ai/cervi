//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
)

// TestServerActionsWithPostgreSQL 验证服务端核心操作。
// 企业安装与管理员登录是全局前置，留在顶层；其余按领域拆成有序子测试，
// 子测试之间存在数据依赖，必须按声明顺序执行，不可并行。
func TestServerActionsWithPostgreSQL(t *testing.T) {
	databaseConfig := testDatabaseConfig(t)
	store, err := Open(context.Background(), databaseConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// 全局前置：安装企业并校验初始状态，失败直接终止整个测试。
	db := store.DB()
	const accessHost = "cervi.test"
	tenantContext := tenant.WithAccessHost(context.Background(), accessHost)
	tenantResolver := organizationaction.NewTenantResolver(db)
	status := installationaction.NewStatusQuery(tenantResolver)
	alreadyInstalled, err := status.Execute(tenantContext)
	if err != nil {
		t.Fatal(err)
	}
	if alreadyInstalled.Installed {
		t.Fatal("fresh database is already installed")
	}

	install := installationaction.NewInstallWorkspaceAction(db)
	installed, err := install.Execute(context.Background(), installationaction.InstallWorkspaceInput{
		AccessHost:       accessHost,
		OrganizationName: "鹿行测试公司",
		DisplayName:      "管理员",
		Email:            "admin@example.com",
		Password:         "password123",
		Locale:           domain.LocaleEnglishUnitedStates,
		TimeZone:         "America/New_York",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Identity.User.RoleID == "" || installed.Identity.Organization.Name != "鹿行测试公司" || installed.Identity.User.Locale != "en-US" || installed.Identity.User.TimeZone != "America/New_York" || !installed.Identity.User.MessageNotificationsEnabled || installed.Identity.OrganizationIdentity.WorkStatus != string(domain.WorkStatusWorking) {
		t.Fatalf("unexpected identity: %#v", installed.Identity)
	}
	if installed.Identity.Organization.AccessHost != accessHost {
		t.Fatalf("organization access host = %q, want %q", installed.Identity.Organization.AccessHost, accessHost)
	}
	if _, err := install.Execute(context.Background(), installationaction.InstallWorkspaceInput{
		AccessHost:       accessHost,
		OrganizationName: "重复企业",
		DisplayName:      "管理员",
		Email:            "duplicate@example.com",
		Password:         "password123",
		Locale:           domain.LocaleChineseSimplified,
		TimeZone:         "Asia/Shanghai",
	}); !errors.Is(err, installationaction.ErrAlreadyInstalled) {
		t.Fatalf("duplicate access host error = %v, want ErrAlreadyInstalled", err)
	}
	if installed.Identity.User.WorkspaceTabsEnabled {
		t.Fatal("workspace tabs enabled = true, want false")
	}
	if installed.Identity.User.IdentityID == "" || installed.Identity.User.IdentityID == installed.Identity.User.ID {
		t.Fatalf("user identity id = %q, user id = %q", installed.Identity.User.IdentityID, installed.Identity.User.ID)
	}
	teamCount, err := db.NewSelect().Model((*servermodels.Team)(nil)).
		Where("organization_id = ?", installed.Identity.Organization.ID).
		Count(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if teamCount != 0 {
		t.Fatalf("team count after installation = %d, want 0", teamCount)
	}
	teamMemberCount, err := db.NewSelect().Model((*servermodels.TeamMember)(nil)).
		Where("organization_id = ?", installed.Identity.Organization.ID).
		Count(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if teamMemberCount != 0 {
		t.Fatalf("team member count after installation = %d, want 0", teamMemberCount)
	}
	currentStatus, err := status.Execute(tenantContext)
	if err != nil {
		t.Fatal(err)
	}
	if !currentStatus.Installed || currentStatus.OrganizationName != "鹿行测试公司" {
		t.Fatalf("status = %#v", currentStatus)
	}
	const otherAccessHost = "other.cervi.test:8443"
	otherTenantContext := tenant.WithAccessHost(context.Background(), otherAccessHost)
	otherStatus, err := status.Execute(otherTenantContext)
	if err != nil {
		t.Fatal(err)
	}
	if otherStatus.Installed {
		t.Fatalf("unbound access host status = %#v, want setup", otherStatus)
	}
	otherInstalled, err := install.Execute(context.Background(), installationaction.InstallWorkspaceInput{
		AccessHost:       otherAccessHost,
		OrganizationName: "另一家测试公司",
		DisplayName:      "管理员",
		Email:            "admin@example.com",
		Password:         "password123",
		Locale:           domain.LocaleChineseSimplified,
		TimeZone:         "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if otherInstalled.Identity.Organization.ID == installed.Identity.Organization.ID {
		t.Fatal("different access hosts resolved to the same organization")
	}
	otherStatus, err = status.Execute(otherTenantContext)
	if err != nil {
		t.Fatal(err)
	}
	if !otherStatus.Installed || otherStatus.OrganizationName != "另一家测试公司" {
		t.Fatalf("other tenant status = %#v", otherStatus)
	}

	// 全局前置：解析安装令牌、登出并重新登录管理员，失败直接终止整个测试。
	resolveIdentity := authaction.NewResolveIdentityQuery(db)
	identity, err := resolveIdentity.Execute(context.Background(), installed.Identity.Organization.ID, installed.Token)
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || identity.User.Email != "admin@example.com" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	if _, err := useraction.NewUpdateWorkStatusAction(db).Execute(context.Background(), identity, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusAway}); err != nil {
		t.Fatal(err)
	}

	logout := authaction.NewLogoutAction(db)
	if err := logout.Execute(context.Background(), installed.Identity.Organization.ID, installed.Token); err != nil {
		t.Fatal(err)
	}

	login := authaction.NewLoginAction(db)
	loggedIn, err := login.Execute(context.Background(), authaction.LoginInput{
		OrganizationID: installed.Identity.Organization.ID,
		Email:          "ADMIN@example.com",
		Password:       "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if loggedIn.Identity.User.ID != installed.Identity.User.ID {
		t.Fatalf("login user = %q, want %q", loggedIn.Identity.User.ID, installed.Identity.User.ID)
	}
	if loggedIn.Identity.OrganizationIdentity.WorkStatus != string(domain.WorkStatusWorking) {
		t.Fatalf("login work status = %q, want %q", loggedIn.Identity.OrganizationIdentity.WorkStatus, domain.WorkStatusWorking)
	}

	// 跨子测试共享：前面子测试创建的实体和操作在后续子测试中继续使用。
	getChannel := channelaction.NewGetWebsiteChannelQuery(db)
	updateChannel := channelaction.NewUpdateMessageChannelAction(db)
	updateProfile := useraction.NewUpdateProfileAction(db)
	var (
		channel             *channelaction.MessageChannelRecord
		team                *teamaction.TeamRecord
		memberRole          *servermodels.Role
		createdMember       *useraction.User
		resolvedAfterUpdate *servermodels.Identity
	)
	// runStep 在子测试失败时终止整个测试，避免后续子测试解引用未赋值的共享变量。
	runStep := func(name string, step func(t *testing.T)) {
		t.Helper()
		if !t.Run(name, step) {
			t.FailNow()
		}
	}

	// 覆盖用户偏好设置更新与工作状态切换流程。
	runStep("用户偏好与工作状态", func(t *testing.T) {
		updatedPreferences, err := useraction.NewUpdatePreferencesAction(db).Execute(context.Background(), loggedIn.Identity, useraction.PreferencesInput{
			Locale:                      domain.LocaleEnglishUnitedStates,
			TimeZone:                    "America/New_York",
			MessageNotificationsEnabled: false,
			WorkspaceTabsEnabled:        true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if updatedPreferences.User.MessageNotificationsEnabled {
			t.Fatal("message notifications enabled = true, want false")
		}
		if !updatedPreferences.User.WorkspaceTabsEnabled {
			t.Fatal("workspace tabs enabled = false, want true")
		}
		loggedIn.Identity = updatedPreferences
		resolvedPreferences, err := resolveIdentity.Execute(context.Background(), loggedIn.Identity.Organization.ID, loggedIn.Token)
		if err != nil {
			t.Fatal(err)
		}
		if resolvedPreferences == nil || resolvedPreferences.User.MessageNotificationsEnabled || !resolvedPreferences.User.WorkspaceTabsEnabled {
			t.Fatalf("identity after preferences update = %#v", resolvedPreferences)
		}
		updatedWorkStatus, err := useraction.NewUpdateWorkStatusAction(db).Execute(context.Background(), loggedIn.Identity, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusAway})
		if err != nil {
			t.Fatal(err)
		}
		if updatedWorkStatus.OrganizationIdentity.WorkStatus != string(domain.WorkStatusAway) {
			t.Fatalf("updated work status = %q, want %q", updatedWorkStatus.OrganizationIdentity.WorkStatus, domain.WorkStatusAway)
		}
		if _, err := useraction.NewUpdateWorkStatusAction(db).Execute(context.Background(), loggedIn.Identity, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusWorking}); err != nil {
			t.Fatal(err)
		}
	})

	// 覆盖组织名称更新及安装状态同步。
	runStep("组织信息更新", func(t *testing.T) {
		updateOrganization := organizationaction.NewUpdateOrganizationAction(db)
		organization, err := updateOrganization.Execute(context.Background(), loggedIn.Identity, organizationaction.Input{Name: "  鹿行协作  "})
		if err != nil {
			t.Fatal(err)
		}
		if organization.Name != "鹿行协作" || organization.AccessHost != accessHost {
			t.Fatalf("updated organization = %#v, want name 鹿行协作 and access host %q", organization, accessHost)
		}
		currentStatus, err := status.Execute(tenantContext)
		if err != nil {
			t.Fatal(err)
		}
		if currentStatus.OrganizationName != "鹿行协作" {
			t.Fatalf("status organization name = %q, want 鹿行协作", currentStatus.OrganizationName)
		}
		otherStatus, err := status.Execute(otherTenantContext)
		if err != nil {
			t.Fatal(err)
		}
		if otherStatus.OrganizationName != "另一家测试公司" {
			t.Fatalf("other status organization name = %q, want 另一家测试公司", otherStatus.OrganizationName)
		}
	})

	// 覆盖消息渠道的创建、详情、聊天界面配置、更新与启停列表流程。
	runStep("消息渠道管理", func(t *testing.T) {
		createChannel := channelaction.NewCreateMessageChannelAction(db)
		staleIdentity := *loggedIn.Identity
		staleIdentity.User = loggedIn.Identity.User
		staleIdentity.User.ID = "00000000-0000-0000-0000-000000000000"
		_, err := createChannel.Execute(context.Background(), &staleIdentity, channelaction.CreateMessageChannelInput{
			Type: domain.ChannelTypeWebsite,
			MessageChannelInput: channelaction.MessageChannelInput{
				Name:                  "无效渠道",
				DefaultLocale:         domain.LocaleChineseSimplified,
				NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
				FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			},
		})
		if !errors.Is(err, common.ErrIdentityInvalid) {
			t.Fatalf("stale identity error = %v, want %v", err, common.ErrIdentityInvalid)
		}

		channel, err = createChannel.Execute(context.Background(), loggedIn.Identity, channelaction.CreateMessageChannelInput{
			Type: domain.ChannelTypeWebsite,
			MessageChannelInput: channelaction.MessageChannelInput{
				Name:                  "产品官网",
				Description:           "接收官网访客咨询",
				DefaultLocale:         domain.LocaleChineseSimplified,
				NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
				FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if channel.Type != string(domain.ChannelTypeWebsite) || channel.CreatedByUserID != loggedIn.Identity.User.ID {
			t.Fatalf("unexpected created channel: %#v", channel)
		}

		detail, err := getChannel.Execute(context.Background(), loggedIn.Identity, channel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if detail.ChatInterface.ChatTitle != "产品官网" || detail.ChatInterface.ThemeColor != channelaction.DefaultWebsiteChannelThemeColor {
			t.Fatalf("unexpected default chat interface: %#v", detail.ChatInterface)
		}

		telegramChannel, err := createChannel.Execute(context.Background(), loggedIn.Identity, channelaction.CreateMessageChannelInput{
			Type: domain.ChannelTypeTelegram,
			MessageChannelInput: channelaction.MessageChannelInput{
				Name:                  "Telegram 客服",
				DefaultLocale:         domain.LocaleChineseSimplified,
				NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
				FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		telegramDetail, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if telegramDetail.Type != string(domain.ChannelTypeTelegram) || telegramDetail.Connection.BotToken != "" || telegramDetail.Connection.WebhookStatus != nil {
			t.Fatalf("unexpected telegram channel: %#v", telegramDetail)
		}
		telegramSettingCount, err := db.NewSelect().
			Model((*servermodels.TelegramChannelSetting)(nil)).
			Where("tcs.channel_id = ?", telegramChannel.ID).
			Count(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if telegramSettingCount != 1 {
			t.Fatalf("telegram setting count = %d, want 1", telegramSettingCount)
		}

		telegramAPI := &telegramBotAPIFake{bot: telegramintegration.Bot{
			ID: 987654321, IsBot: true, FirstName: "Cervi", LastName: "Support", Username: "cervi_support_bot",
		}}
		telegramRunner := connectiontest.NewRunner(time.Second)
		testTelegram := channelaction.NewTestTelegramConnectionAction(db, telegramRunner, telegramAPI)
		if err := testTelegram.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, channelaction.TelegramChannelConnectionTestInput{BotToken: "123456:draft_token"}); err != nil {
			t.Fatal(err)
		}
		detailAfterTest, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if detailAfterTest.Connection.BotToken != "" || detailAfterTest.Connection.WebhookStatus != nil || len(telegramAPI.webhooks()) != 0 {
			t.Fatalf("test changed Telegram setting: %#v", detailAfterTest.Connection)
		}

		saveTelegram := channelaction.NewSaveTelegramConnectionAction(db, telegramRunner, telegramAPI)
		savedTelegram, err := saveTelegram.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, channelaction.TelegramChannelConnectionInput{
			BotToken:       "123456:saved_token",
			WebhookBaseURL: "http://127.0.0.1:34115/cervi",
		})
		if err != nil {
			t.Fatal(err)
		}
		if savedTelegram.Connection.BotID == nil || *savedTelegram.Connection.BotID != telegramAPI.bot.ID ||
			savedTelegram.Connection.BotUsername == nil || *savedTelegram.Connection.BotUsername != telegramAPI.bot.Username ||
			savedTelegram.Connection.BotDisplayName == nil || *savedTelegram.Connection.BotDisplayName != "Cervi Support" ||
			savedTelegram.Connection.WebhookStatus == nil || *savedTelegram.Connection.WebhookStatus != string(domain.TelegramWebhookStatusWaiting) {
			t.Fatalf("saved Telegram connection = %#v", savedTelegram.Connection)
		}
		const expectedTelegramWebhookURL = "http://127.0.0.1:34115/cervi/api/public/telegram-channels/"
		if savedTelegram.Connection.WebhookURL != expectedTelegramWebhookURL+telegramChannel.ID+"/webhook" {
			t.Fatalf("webhook URL = %q", savedTelegram.Connection.WebhookURL)
		}
		webhooks := telegramAPI.webhooks()
		if len(webhooks) != 1 || webhooks[0].URL != savedTelegram.Connection.WebhookURL || webhooks[0].Secret != savedTelegram.Connection.WebhookSecret {
			t.Fatalf("registered webhooks = %#v, saved secret = %q", webhooks, savedTelegram.Connection.WebhookSecret)
		}

		telegramAvatarAPI := &telegramProfilePhotoAPIStub{
			photo:      &telegramintegration.ProfilePhoto{FileID: "avatar-file-1", UniqueID: "avatar-version-1"},
			downloaded: telegramintegration.DownloadedPhoto{ContentType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff}},
		}
		importedAvatarWriter := &importedFileWriterStub{}
		telegramAvatarFiles := fileaction.NewImportAction(db, func(context.Context, string) (domain.FileStorageBackend, error) {
			return domain.FileStorageBackendLocal, nil
		}, importedAvatarWriter)
		receiveTelegram := channelaction.NewReceiveTelegramWebhookAction(db, telegramAvatarAPI, telegramAvatarFiles)
		if err := receiveTelegram.Preflight(context.Background(), telegramChannel.ID, "wrong-secret"); !errors.Is(err, channelaction.ErrTelegramWebhookUnauthorized) {
			t.Fatalf("wrong secret error = %v", err)
		}
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, channelaction.TelegramWebhookInput{
			Secret: savedTelegram.Connection.WebhookSecret, UpdateID: 1,
		}); err != nil {
			t.Fatalf("ignored update error = %v", err)
		}
		connectedTelegram, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if connectedTelegram.Connection.WebhookStatus == nil || *connectedTelegram.Connection.WebhookStatus != string(domain.TelegramWebhookStatusNormal) {
			t.Fatalf("status after ignored update = %#v", connectedTelegram.Connection.WebhookStatus)
		}
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, channelaction.TelegramWebhookInput{
			Secret: savedTelegram.Connection.WebhookSecret, UpdateID: 2, MyChatMember: true,
		}); err != nil {
			t.Fatal(err)
		}
		connectedTelegram, err = channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if connectedTelegram.Connection.WebhookStatus == nil || *connectedTelegram.Connection.WebhookStatus != string(domain.TelegramWebhookStatusNormal) {
			t.Fatalf("connected Telegram status = %#v", connectedTelegram.Connection.WebhookStatus)
		}
		telegramOriginatedAt := time.Date(2026, time.August, 30, 5, 6, 7, 0, time.UTC)
		telegramMessage := channelaction.TelegramWebhookInput{
			Secret: savedTelegram.Connection.WebhookSecret, UpdateID: 3,
			Message: &channelaction.TelegramWebhookMessage{
				ChatID: 998877, MessageID: 41, SenderID: 998877,
				DisplayName: "Telegram 访客", Body: "Telegram 私聊消息",
				OriginatedAt: telegramOriginatedAt,
			},
		}
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, telegramMessage); err != nil {
			t.Fatal(err)
		}
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, telegramMessage); err != nil {
			t.Fatalf("duplicate Telegram message with unchanged avatar error = %v", err)
		}
		telegramAvatarAPI.err = errors.New("avatar unavailable")
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, telegramMessage); err != nil {
			t.Fatalf("duplicate Telegram message with avatar failure error = %v", err)
		}
		telegramAvatarAPI.err = nil
		telegramMessages := make([]servermodels.Message, 0)
		if err := db.NewSelect().Model(&telegramMessages).
			Join("JOIN customer_conversations AS cc ON cc.conversation_id = msg.conversation_id AND cc.organization_id = msg.organization_id").
			Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			OrderExpr("msg.originated_at ASC, msg.source_order ASC, msg.id ASC").
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(telegramMessages) != 1 || telegramMessages[0].Body != telegramMessage.Message.Body || telegramMessages[0].SourceOrder != telegramMessage.Message.MessageID || !telegramMessages[0].OriginatedAt.Equal(telegramOriginatedAt) {
			t.Fatalf("Telegram messages = %#v", telegramMessages)
		}
		telegramMessage.UpdateID = 4
		telegramMessage.Message.MessageID = 42
		telegramMessage.Message.DisplayName = "Telegram 新名称"
		telegramMessage.Message.Body = "同秒第二条消息"
		telegramAvatarAPI.photo = &telegramintegration.ProfilePhoto{FileID: "avatar-file-2", UniqueID: "avatar-version-2"}
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, telegramMessage); err != nil {
			t.Fatal(err)
		}
		var telegramIdentity servermodels.ContactChannelIdentity
		if err := db.NewSelect().Model(&telegramIdentity).
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if telegramIdentity.DisplayName == nil || *telegramIdentity.DisplayName != telegramMessage.Message.DisplayName {
			t.Fatalf("Telegram identity display name = %#v", telegramIdentity.DisplayName)
		}
		if telegramIdentity.AvatarFileID == nil {
			t.Fatalf("Telegram identity avatar = %#v", telegramIdentity)
		}
		telegramAvatarFilesInDatabase := make([]servermodels.File, 0)
		if err := db.NewSelect().Model(&telegramAvatarFilesInDatabase).
			Where("f.organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("f.purpose = ?", domain.FilePurposeContactAvatar).
			OrderExpr("f.created_at ASC, f.id ASC").
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(telegramAvatarFilesInDatabase) != 2 || telegramAvatarFilesInDatabase[0].Status != string(domain.FileStatusDeleting) ||
			telegramAvatarFilesInDatabase[1].ID != *telegramIdentity.AvatarFileID || telegramAvatarFilesInDatabase[1].Status != string(domain.FileStatusActive) ||
			telegramAvatarFilesInDatabase[1].StorageBackend != string(domain.FileStorageBackendLocal) || telegramAvatarFilesInDatabase[1].ContentType != "image/jpeg" ||
			telegramAvatarFilesInDatabase[0].ExternalID == nil || *telegramAvatarFilesInDatabase[0].ExternalID != "avatar-version-1" ||
			telegramAvatarFilesInDatabase[1].ExternalID == nil || *telegramAvatarFilesInDatabase[1].ExternalID != "avatar-version-2" {
			t.Fatalf("Telegram avatar files = %#v", telegramAvatarFilesInDatabase)
		}
		if importedAvatarWriter.saved != 2 {
			t.Fatalf("imported Telegram avatar writes = %d, want 2", importedAvatarWriter.saved)
		}
		latestTelegramMessage := servermodels.Message{}
		if err := db.NewSelect().Model(&latestTelegramMessage).
			Join("JOIN customer_conversations AS cc ON cc.conversation_id = msg.conversation_id AND cc.organization_id = msg.organization_id").
			Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			OrderExpr("msg.originated_at DESC, msg.source_order DESC, msg.id DESC").
			Limit(1).
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if latestTelegramMessage.Body != telegramMessage.Message.Body || latestTelegramMessage.SourceOrder != telegramMessage.Message.MessageID || !latestTelegramMessage.OriginatedAt.Equal(telegramOriginatedAt) {
			t.Fatalf("latest Telegram message = %#v", latestTelegramMessage)
		}
		telegramConversation := struct {
			ID string `bun:"id"`
		}{}
		err = db.NewSelect().
			TableExpr("customer_conversations AS cc").
			ColumnExpr("cc.conversation_id AS id").
			Join("JOIN contact_channel_identities AS cci ON cci.id = cc.contact_channel_identity_id AND cci.organization_id = cc.organization_id").
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			Scan(context.Background(), &telegramConversation)
		if err != nil {
			t.Fatal(err)
		}
		messageCountBeforeReply, err := db.NewSelect().Model((*servermodels.Message)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("conversation_id = ?", telegramConversation.ID).
			Count(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if messageCountBeforeReply != 2 {
			t.Fatalf("Telegram message count = %d, want 2", messageCountBeforeReply)
		}
		sendCustomerMessage := conversationaction.NewSendCustomerTextMessageAction(db)
		_, err = sendCustomerMessage.Execute(context.Background(), loggedIn.Identity, conversationaction.CustomerTextMessageInput{
			ConversationID:  telegramConversation.ID,
			ClientMessageID: "019d4e1c-40a5-77dd-82e6-6951f9957ba5",
			Body:            "不应写入的 Telegram 回复",
		})
		var replyConflict *conversationaction.ConflictError
		if !errors.As(err, &replyConflict) || replyConflict.Reason != conversationaction.ConflictReasonChannelOutboundUnsupported {
			t.Fatalf("Telegram reply error = %#v", err)
		}
		messageCountAfterReply, err := db.NewSelect().Model((*servermodels.Message)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("conversation_id = ?", telegramConversation.ID).
			Count(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if messageCountAfterReply != messageCountBeforeReply {
			t.Fatalf("Telegram message count after rejected reply = %d, want %d", messageCountAfterReply, messageCountBeforeReply)
		}
		sameAvatarMessage := *telegramMessage.Message
		sameAvatarMessage.MessageID = 43
		sameAvatarMessage.Body = "头像未变化的 Telegram 消息"
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, channelaction.TelegramWebhookInput{
			Secret: savedTelegram.Connection.WebhookSecret, UpdateID: 5, Message: &sameAvatarMessage,
		}); err != nil {
			t.Fatal(err)
		}
		telegramIdentity = servermodels.ContactChannelIdentity{}
		if err := db.NewSelect().Model(&telegramIdentity).
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if telegramIdentity.AvatarFileID == nil || *telegramIdentity.AvatarFileID != telegramAvatarFilesInDatabase[1].ID || importedAvatarWriter.saved != 2 {
			t.Fatalf("unchanged Telegram avatar = %#v, writes = %d", telegramIdentity, importedAvatarWriter.saved)
		}
		noAvatarMessage := *telegramMessage.Message
		noAvatarMessage.MessageID = 44
		noAvatarMessage.Body = "删除头像后的 Telegram 消息"
		telegramAvatarAPI.photo = nil
		if err := receiveTelegram.Execute(context.Background(), telegramChannel.ID, channelaction.TelegramWebhookInput{
			Secret: savedTelegram.Connection.WebhookSecret, UpdateID: 6, Message: &noAvatarMessage,
		}); err != nil {
			t.Fatal(err)
		}
		telegramIdentity = servermodels.ContactChannelIdentity{}
		if err := db.NewSelect().Model(&telegramIdentity).
			Where("cci.channel_id = ?", telegramChannel.ID).
			Where("cci.external_id = ?", "998877").
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if telegramIdentity.AvatarFileID != nil {
			t.Fatalf("deleted Telegram avatar = %#v", telegramIdentity)
		}
		activeTelegramAvatarCount, err := db.NewSelect().Model((*servermodels.File)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("purpose = ?", domain.FilePurposeContactAvatar).
			Where("status = ?", domain.FileStatusActive).
			Count(context.Background())
		if err != nil || activeTelegramAvatarCount != 0 {
			t.Fatalf("active Telegram avatar count = %d, error = %v", activeTelegramAvatarCount, err)
		}

		reusedBotChannel, err := createChannel.Execute(context.Background(), loggedIn.Identity, channelaction.CreateMessageChannelInput{
			Type: domain.ChannelTypeTelegram,
			MessageChannelInput: channelaction.MessageChannelInput{
				Name:                  "Telegram 复用确认",
				DefaultLocale:         domain.LocaleChineseSimplified,
				NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
				FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		webhookCountBeforeReuse := len(telegramAPI.webhooks())
		_, err = saveTelegram.Execute(context.Background(), loggedIn.Identity, reusedBotChannel.ID, channelaction.TelegramChannelConnectionInput{
			BotToken: "123456:reused_token", WebhookBaseURL: "http://127.0.0.1:34115/cervi",
		})
		if !errors.Is(err, channelaction.ErrTelegramBotReuseConfirmationRequired) {
			t.Fatalf("unconfirmed Telegram bot reuse error = %v", err)
		}
		unconfirmedReuse, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, reusedBotChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if unconfirmedReuse.Connection.BotToken != "" || len(telegramAPI.webhooks()) != webhookCountBeforeReuse {
			t.Fatalf("unconfirmed reuse changed state: detail=%#v webhooks=%#v", unconfirmedReuse.Connection, telegramAPI.webhooks())
		}
		confirmedReuse, err := saveTelegram.Execute(context.Background(), loggedIn.Identity, reusedBotChannel.ID, channelaction.TelegramChannelConnectionInput{
			BotToken: "123456:reused_token", WebhookBaseURL: "http://127.0.0.1:34115/cervi", ConfirmBotReuse: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if confirmedReuse.Connection.BotID == nil || *confirmedReuse.Connection.BotID != telegramAPI.bot.ID ||
			len(telegramAPI.webhooks()) != webhookCountBeforeReuse+1 ||
			telegramAPI.webhooks()[webhookCountBeforeReuse].URL != confirmedReuse.Connection.WebhookURL {
			t.Fatalf("confirmed Telegram bot reuse = %#v, webhooks = %#v", confirmedReuse.Connection, telegramAPI.webhooks())
		}
		originalAfterReuse, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if originalAfterReuse.Connection.WebhookSecret != connectedTelegram.Connection.WebhookSecret ||
			originalAfterReuse.Connection.WebhookStatus == nil || *originalAfterReuse.Connection.WebhookStatus != string(domain.TelegramWebhookStatusNormal) {
			t.Fatalf("reusing bot changed old channel = %#v", originalAfterReuse.Connection)
		}

		updateTelegramStatus := channelaction.NewUpdateTelegramChannelStatusAction(db, telegramRunner, telegramAPI)
		disabledTelegram, err := updateTelegramStatus.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		if disabledTelegram.Enabled {
			t.Fatal("disabled Telegram channel remains enabled")
		}
		if err := receiveTelegram.Preflight(context.Background(), telegramChannel.ID, savedTelegram.Connection.WebhookSecret); !errors.Is(err, channelaction.ErrNotFound) {
			t.Fatalf("disabled webhook preflight error = %v", err)
		}
		if len(telegramAPI.deletedTokens()) != 0 {
			t.Fatalf("deleted Telegram tokens = %#v", telegramAPI.deletedTokens())
		}

		reenabledTelegram, err := updateTelegramStatus.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if !reenabledTelegram.Enabled {
			t.Fatal("re-enabled Telegram channel remains disabled")
		}
		reenabledDetail, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if reenabledDetail.Connection.WebhookStatus == nil || *reenabledDetail.Connection.WebhookStatus != string(domain.TelegramWebhookStatusWaiting) || reenabledDetail.Connection.WebhookSecret == savedTelegram.Connection.WebhookSecret {
			t.Fatalf("re-enabled Telegram connection = %#v", reenabledDetail.Connection)
		}
		if _, err := db.NewDelete().Model((*servermodels.TelegramChannelSetting)(nil)).Where("channel_id = ?", reusedBotChannel.ID).Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.NewDelete().Model((*servermodels.Channel)(nil)).Where("id = ?", reusedBotChannel.ID).Exec(context.Background()); err != nil {
			t.Fatal(err)
		}

		startSaves := make(chan struct{})
		saveErrors := make(chan error, 2)
		for _, token := range []string{"123456:concurrent_one", "123456:concurrent_two"} {
			token := token
			go func() {
				<-startSaves
				_, err := saveTelegram.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, channelaction.TelegramChannelConnectionInput{
					BotToken: token, WebhookBaseURL: "http://127.0.0.1:34115/cervi",
				})
				saveErrors <- err
			}()
		}
		close(startSaves)
		for range 2 {
			if err := <-saveErrors; err != nil {
				t.Fatal(err)
			}
		}
		concurrentDetail, err := channelaction.NewGetTelegramChannelQuery(db).Execute(context.Background(), loggedIn.Identity, telegramChannel.ID)
		if err != nil {
			t.Fatal(err)
		}
		webhooks = telegramAPI.webhooks()
		if len(webhooks) < 4 || webhooks[len(webhooks)-1].Secret != concurrentDetail.Connection.WebhookSecret {
			t.Fatalf("final registered webhook = %#v, database secret = %q", webhooks, concurrentDetail.Connection.WebhookSecret)
		}
		settingCount, err := db.NewSelect().
			Model((*servermodels.WebsiteChannelSetting)(nil)).
			Where("wcs.channel_id = ?", telegramChannel.ID).
			Count(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if settingCount != 0 {
			t.Fatalf("telegram website setting count = %d, want 0", settingCount)
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

		channel, err = updateChannel.Execute(context.Background(), loggedIn.Identity, channel.ID, channelaction.MessageChannelInput{
			Name:                  "帮助中心",
			DefaultLocale:         domain.LocaleEnglishUnitedStates,
			NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		})
		if err != nil {
			t.Fatal(err)
		}
		if channel.Name != "帮助中心" || channel.Description != nil || channel.DefaultLocale != string(domain.LocaleEnglishUnitedStates) {
			t.Fatalf("unexpected updated channel: %#v", channel)
		}
		telegramChannel, err = updateChannel.Execute(context.Background(), loggedIn.Identity, telegramChannel.ID, channelaction.MessageChannelInput{
			Name:                  "Telegram 支持",
			DefaultLocale:         domain.LocaleEnglishUnitedStates,
			NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		})
		if err != nil {
			t.Fatal(err)
		}
		if telegramChannel.Type != string(domain.ChannelTypeTelegram) || telegramChannel.Name != "Telegram 支持" || telegramChannel.DefaultLocale != string(domain.LocaleEnglishUnitedStates) {
			t.Fatalf("unexpected updated telegram channel: %#v", telegramChannel)
		}

		updateChannelStatus := channelaction.NewUpdateMessageChannelStatusAction(db)
		channel, err = updateChannelStatus.Execute(context.Background(), loggedIn.Identity, channel.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		if channel.Enabled {
			t.Fatal("channel enabled = true, want false")
		}
		listChannels := channelaction.NewListMessageChannelsQuery(db)
		channels, err := listChannels.Execute(context.Background(), loggedIn.Identity)
		if err != nil {
			t.Fatal(err)
		}
		if len(channels) != 2 || channels[0].ID != channel.ID || channels[0].Enabled {
			t.Fatalf("unexpected disabled channels: %#v", channels)
		}
		telegramListed := false
		for _, listedChannel := range channels {
			if listedChannel.ID == telegramChannel.ID && listedChannel.Type == string(domain.ChannelTypeTelegram) {
				telegramListed = true
				break
			}
		}
		if !telegramListed {
			t.Fatalf("telegram channel missing from list: %#v", channels)
		}

		channel, err = updateChannelStatus.Execute(context.Background(), loggedIn.Identity, channel.ID, true)
		if err != nil {
			t.Fatal(err)
		}
		if !channel.Enabled {
			t.Fatal("channel enabled = false, want true")
		}
	})

	// 覆盖用户目录列表与单个用户查询。
	runStep("用户目录", func(t *testing.T) {
		users, err := useraction.NewListUsersQuery(db).Execute(context.Background(), loggedIn.Identity, useraction.ListInput{Page: 1, PageSize: 50})
		if err != nil {
			t.Fatal(err)
		}
		if users.Page.Total != 1 || len(users.Users) != 1 || users.Users[0].ID != loggedIn.Identity.User.ID || users.Users[0].CreatedAt.IsZero() {
			t.Fatalf("unexpected user directory: %#v", users)
		}
		user, err := useraction.NewGetUserQuery(db).Execute(context.Background(), loggedIn.Identity, loggedIn.Identity.User.ID)
		if err != nil {
			t.Fatal(err)
		}
		if user.ID != loggedIn.Identity.User.ID || user.CreatedAt.IsZero() {
			t.Fatalf("unexpected user: %#v", user)
		}
	})

	// 覆盖团队创建、成员账号管理、角色变更保护与团队成员增删流程。
	runStep("团队与成员管理", func(t *testing.T) {
		var err error
		team, err = teamaction.NewCreateTeamAction(db).Execute(context.Background(), loggedIn.Identity, teamaction.Input{Name: "客户成功", Description: "服务客户"})
		if err != nil {
			t.Fatal(err)
		}
		memberRole = &servermodels.Role{}
		if err := db.NewSelect().Model(memberRole).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("kind = ?", domain.RoleKindMember).
			Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		createdMember, err = useraction.NewCreateUserAction(db).Execute(context.Background(), loggedIn.Identity, useraction.CreateInput{
			DisplayName: "团队成员", Email: "member@example.com", Password: "password123", RoleID: memberRole.ID, TeamIDs: []string{team.ID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(createdMember.Teams) != 1 || createdMember.Teams[0].ID != team.ID {
			t.Fatalf("created member teams = %#v", createdMember.Teams)
		}
		if createdMember.IdentityID == "" || createdMember.IdentityID == createdMember.ID {
			t.Fatalf("member identity id = %q, user id = %q", createdMember.IdentityID, createdMember.ID)
		}
		updateRoles := useraction.NewUpdateRolesAction(db)
		if err := updateRoles.Execute(context.Background(), loggedIn.Identity, []useraction.RoleChangeInput{{UserID: loggedIn.Identity.User.ID, RoleID: memberRole.ID}}); !errors.Is(err, useraction.ErrLastActiveAdministrator) {
			t.Fatalf("remove last active administrator error = %v", err)
		}
		administratorAfterRollback, err := useraction.NewGetUserQuery(db).Execute(context.Background(), loggedIn.Identity, loggedIn.Identity.User.ID)
		if err != nil || administratorAfterRollback.RoleID != loggedIn.Identity.User.RoleID {
			t.Fatalf("administrator after rollback = %#v, error = %v", administratorAfterRollback, err)
		}
		if err := updateRoles.Execute(context.Background(), loggedIn.Identity, []useraction.RoleChangeInput{
			{UserID: loggedIn.Identity.User.ID, RoleID: memberRole.ID},
			{UserID: createdMember.ID, RoleID: loggedIn.Identity.User.RoleID},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := useraction.NewUpdateStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdMember.ID, domain.UserStatusInactive); !errors.Is(err, useraction.ErrLastActiveAdministrator) {
			t.Fatalf("deactivate last active administrator error = %v", err)
		}
		if err := updateRoles.Execute(context.Background(), loggedIn.Identity, []useraction.RoleChangeInput{
			{UserID: loggedIn.Identity.User.ID, RoleID: loggedIn.Identity.User.RoleID},
			{UserID: createdMember.ID, RoleID: memberRole.ID},
		}); err != nil {
			t.Fatal(err)
		}
		teamUsers, err := useraction.NewListUsersQuery(db).Execute(context.Background(), loggedIn.Identity, useraction.ListInput{TeamID: team.ID, Page: 1, PageSize: 50})
		if err != nil || teamUsers.Page.Total != 1 || len(teamUsers.Users) != 1 {
			t.Fatalf("team users = %#v, error = %v", teamUsers, err)
		}
		inactiveMember, err := useraction.NewUpdateStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdMember.ID, domain.UserStatusInactive)
		if err != nil {
			t.Fatal(err)
		}
		if inactiveMember.WorkStatus != domain.WorkStatusOffDuty {
			t.Fatalf("inactive member work status = %q, want %q", inactiveMember.WorkStatus, domain.WorkStatusOffDuty)
		}
		inactiveTeamMembers, err := teamaction.NewListMembersQuery(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.MemberListInput{Page: 1, PageSize: 50})
		if err != nil || inactiveTeamMembers.Page.Total != 0 || len(inactiveTeamMembers.Members) != 0 {
			t.Fatalf("team members after user deactivation = %#v, error = %v", inactiveTeamMembers, err)
		}
		teamAfterUserDeactivation, err := teamaction.NewListTeamsQuery(db).Execute(context.Background(), loggedIn.Identity, teamaction.ListInput{Page: 1, PageSize: 50})
		if err != nil || len(teamAfterUserDeactivation.Teams) != 1 || teamAfterUserDeactivation.Teams[0].MemberCount != 0 {
			t.Fatalf("team after user deactivation = %#v, error = %v", teamAfterUserDeactivation, err)
		}
		reactivatedMember, err := useraction.NewUpdateStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdMember.ID, domain.UserStatusActive)
		if err != nil {
			t.Fatal(err)
		}
		if reactivatedMember.WorkStatus != domain.WorkStatusOffDuty {
			t.Fatalf("reactivated member work status = %q, want %q", reactivatedMember.WorkStatus, domain.WorkStatusOffDuty)
		}
		reactivatedTeamMembers, err := teamaction.NewListMembersQuery(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.MemberListInput{Page: 1, PageSize: 50})
		if err != nil || reactivatedTeamMembers.Page.Total != 1 || len(reactivatedTeamMembers.Members) != 1 || reactivatedTeamMembers.Members[0].IdentityID != createdMember.IdentityID {
			t.Fatalf("team members after user reactivation = %#v, error = %v", reactivatedTeamMembers, err)
		}
		teamAfterUserReactivation, err := teamaction.NewListTeamsQuery(db).Execute(context.Background(), loggedIn.Identity, teamaction.ListInput{Page: 1, PageSize: 50})
		if err != nil || len(teamAfterUserReactivation.Teams) != 1 || teamAfterUserReactivation.Teams[0].MemberCount != 1 {
			t.Fatalf("team after user reactivation = %#v, error = %v", teamAfterUserReactivation, err)
		}
		if _, err := teamaction.NewRemoveMembersAction(db).Execute(context.Background(), loggedIn.Identity, team.ID, []teamaction.MemberIdentity{{IdentityType: domain.OrganizationIdentityTypeUser, IdentityID: createdMember.IdentityID}}); err != nil {
			t.Fatal(err)
		}
		candidates, err := teamaction.NewListMemberCandidatesQuery(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.MemberCandidateInput{Query: createdMember.DisplayName, Page: 1, PageSize: 50})
		if err != nil || candidates.Page.Total != 1 || len(candidates.Members) != 1 || candidates.Members[0].IdentityID != createdMember.IdentityID {
			t.Fatalf("team member candidates = %#v, error = %v", candidates, err)
		}
		team, err = teamaction.NewAddMembersAction(db).Execute(context.Background(), loggedIn.Identity, team.ID, []teamaction.MemberIdentity{{IdentityType: domain.OrganizationIdentityTypeUser, IdentityID: createdMember.IdentityID}})
		if err != nil || team.MemberCount != 1 {
			t.Fatalf("team after adding member = %#v, error = %v", team, err)
		}
	})

	// 覆盖双向并发发起、双方收件箱、成员授权和内部文本消息。
	runStep("企业成员内部单聊", func(t *testing.T) {
		memberLogin, err := login.Execute(context.Background(), authaction.LoginInput{
			OrganizationID: loggedIn.Identity.Organization.ID,
			Email:          createdMember.Email,
			Password:       "password123",
		})
		if err != nil {
			t.Fatal(err)
		}

		start := conversationaction.NewStartDirectConversationAction(db)
		startGate := make(chan struct{})
		startResults := make(chan conversationaction.DirectConversationSummary, 2)
		startErrors := make(chan error, 2)
		requests := []struct {
			identity *servermodels.Identity
			targetID string
		}{
			{identity: loggedIn.Identity, targetID: memberLogin.Identity.OrganizationIdentity.ID},
			{identity: memberLogin.Identity, targetID: loggedIn.Identity.OrganizationIdentity.ID},
		}
		for _, request := range requests {
			request := request
			go func() {
				<-startGate
				summary, executeErr := start.Execute(context.Background(), request.identity, conversationaction.DirectConversationInput{TargetIdentityID: request.targetID})
				startResults <- summary
				startErrors <- executeErr
			}()
		}
		close(startGate)

		conversationID := ""
		for range requests {
			if executeErr := <-startErrors; executeErr != nil {
				t.Fatal(executeErr)
			}
			summary := <-startResults
			if conversationID == "" {
				conversationID = summary.ID
			} else if summary.ID != conversationID {
				t.Fatalf("concurrent direct conversation ids = %q and %q", conversationID, summary.ID)
			}
		}

		conversationCount, err := db.NewSelect().
			Model((*servermodels.Conversation)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("type = ?", domain.ConversationTypeDirect).
			Count(context.Background())
		if err != nil || conversationCount != 1 {
			t.Fatalf("direct conversation count = %d, error = %v", conversationCount, err)
		}
		participantCount, err := db.NewSelect().
			Model((*servermodels.ConversationParticipant)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("conversation_id = ?", conversationID).
			Count(context.Background())
		if err != nil || participantCount != 2 {
			t.Fatalf("direct participant count = %d, error = %v", participantCount, err)
		}

		inbox := inboxaction.NewLoadInboxQuery(db)
		for _, request := range requests {
			items, loadErr := inbox.Execute(context.Background(), request.identity)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			found := false
			for _, item := range items {
				if item.ID == conversationID && item.Type == domain.ConversationTypeDirect && item.Direct != nil && item.Direct.PeerIdentityID == request.targetID && item.Direct.LastMessageAt == nil {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("direct conversation missing from inbox: %#v", items)
			}
		}

		send := conversationaction.NewSendDirectTextMessageAction(db)
		message, err := send.Execute(context.Background(), memberLogin.Identity, conversationaction.DirectTextMessageInput{
			ConversationID:  conversationID,
			ClientMessageID: "0198ddf0-a234-7f01-8d99-e3e0af0f5f65",
			Body:            "你好，管理员",
		})
		if err != nil {
			t.Fatal(err)
		}
		if message.Sender == nil || message.Sender.SourceID != memberLogin.Identity.OrganizationIdentity.ID {
			t.Fatalf("direct message sender = %#v", message.Sender)
		}
		history, err := conversationaction.NewListConversationMessagesQuery(db).Execute(context.Background(), loggedIn.Identity, conversationaction.ConversationMessageHistoryInput{ConversationID: conversationID})
		if err != nil || len(history.Messages) != 1 || history.Messages[0].ID != message.ID || history.Messages[0].Sender == nil || history.Messages[0].Sender.SourceID != memberLogin.Identity.OrganizationIdentity.ID {
			t.Fatalf("direct message history = %#v, error = %v", history, err)
		}

		if _, err := db.NewUpdate().Model((*servermodels.Conversation)(nil)).
			Set("status = ?", domain.ConversationStatusArchived).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("id = ?", conversationID).
			Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		_, err = send.Execute(context.Background(), loggedIn.Identity, conversationaction.DirectTextMessageInput{
			ConversationID:  conversationID,
			ClientMessageID: "0198ddf0-a234-7f01-8d99-e3e0af0f5f66",
			Body:            "归档后发送",
		})
		if !errors.Is(err, conversationaction.ErrConversationNotFound) {
			t.Fatalf("send archived direct error = %v, want conversation not found", err)
		}
		reopened, err := start.Execute(context.Background(), loggedIn.Identity, conversationaction.DirectConversationInput{TargetIdentityID: memberLogin.Identity.OrganizationIdentity.ID})
		if err != nil || reopened.ID != conversationID {
			t.Fatalf("reopened direct conversation = %#v, error = %v", reopened, err)
		}
	})

	// 覆盖 AI 员工的创建、执行配置修订、状态切换、团队与渠道联动及团队删除。
	runStep("AI员工", func(t *testing.T) {
		provider := &servermodels.AIProvider{
			OrganizationID: loggedIn.Identity.Organization.ID,
			Brand:          string(domain.AIProviderBrandOpenAI),
			Name:           "测试模型服务",
			APIKey:         "test-key",
			APIURL:         "https://example.com/v1",
		}
		if _, err := db.NewInsert().Model(provider).
			Column("organization_id", "brand", "name", "api_key", "api_url").
			Returning("id").
			Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		model := &servermodels.AIProviderModel{
			ProviderID: provider.ID, OrganizationID: loggedIn.Identity.Organization.ID,
			Identifier: "chat-model", Name: "测试对话模型", Type: string(domain.AIModelTypeChat),
			InputModalities: json.RawMessage(`["text"]`), ContextWindow: 128000, MaxOutputTokens: 4096,
		}
		if _, err := db.NewInsert().Model(model).
			Column("provider_id", "organization_id", "identifier", "name", "model_type", "input_modalities", "context_window", "max_output_tokens").
			Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		createdAgent, err := agentaction.NewCreateAgentAction(db).Execute(context.Background(), loggedIn.Identity, agentaction.CreateInput{
			DisplayName: "接待智能体",
			TeamIDs:     []string{team.ID},
			Execution: agentaction.ExecutionInput{
				Mode: domain.AgentExecutionModeManaged,
				Managed: &agentaction.ManagedExecutionInput{
					ProviderID: provider.ID, ModelIdentifier: model.Identifier, SystemInstruction: "负责接待客户。",
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(createdAgent.Teams) != 1 || createdAgent.Teams[0].ID != team.ID || createdAgent.CreatedAt.IsZero() || createdAgent.Execution.Managed == nil || createdAgent.Execution.Managed.ModelIdentifier != model.Identifier {
			t.Fatalf("created agent = %#v", createdAgent)
		}
		originalRevisionID := createdAgent.Execution.RevisionID
		agentWithUpdatedExecution, err := agentaction.NewUpdateExecutionAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, agentaction.ExecutionInput{
			Mode: domain.AgentExecutionModeManaged,
			Managed: &agentaction.ManagedExecutionInput{
				ProviderID: provider.ID, ModelIdentifier: model.Identifier, SystemInstruction: "负责接待并回答客户问题。",
			},
		})
		if err != nil || agentWithUpdatedExecution.Execution.RevisionID == originalRevisionID || agentWithUpdatedExecution.Execution.Managed.SystemInstruction != "负责接待并回答客户问题。" {
			t.Fatalf("updated agent execution = %#v, error = %v", agentWithUpdatedExecution, err)
		}
		revisionCount, err := db.NewSelect().Model((*servermodels.AgentRevision)(nil)).
			Where("organization_id = ?", loggedIn.Identity.Organization.ID).
			Where("agent_id = ?", createdAgent.ID).
			Count(context.Background())
		if err != nil || revisionCount != 2 {
			t.Fatalf("agent revision count = %d, error = %v", revisionCount, err)
		}
		if createdAgent.IdentityID == "" || createdAgent.IdentityID == createdAgent.ID {
			t.Fatalf("agent identity id = %q, agent id = %q", createdAgent.IdentityID, createdAgent.ID)
		}
		agents, err := agentaction.NewListAgentsQuery(db).Execute(context.Background(), loggedIn.Identity, agentaction.ListInput{Page: 1, PageSize: 50})
		if err != nil || agents.Page.Total != 1 || len(agents.Agents) != 1 || agents.Agents[0].ID != createdAgent.ID {
			t.Fatalf("agent directory = %#v, error = %v", agents, err)
		}
		updatedAgent, err := agentaction.NewUpdateAgentAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, agentaction.UpdateInput{
			DisplayName: "售前智能体",
			TeamIDs:     []string{team.ID},
		})
		if err != nil || updatedAgent.DisplayName != "售前智能体" {
			t.Fatalf("updated agent = %#v, error = %v", updatedAgent, err)
		}
		agent, err := agentaction.NewGetAgentQuery(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID)
		if err != nil || agent.DisplayName != "售前智能体" {
			t.Fatalf("agent detail = %#v, error = %v", agent, err)
		}
		updatedAgent, err = agentaction.NewUpdateWorkStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, agentaction.WorkStatusInput{WorkStatus: domain.WorkStatusAway})
		if err != nil || updatedAgent.WorkStatus != domain.WorkStatusAway {
			t.Fatalf("away agent = %#v, error = %v", updatedAgent, err)
		}
		teamMembers, err := teamaction.NewListMembersQuery(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.MemberListInput{WorkStatus: domain.WorkStatusAway, Page: 1, PageSize: 50})
		if err != nil || teamMembers.Page.Total != 1 || len(teamMembers.Members) != 1 || teamMembers.Members[0].IdentityID != createdAgent.IdentityID || teamMembers.Members[0].WorkStatus != domain.WorkStatusAway {
			t.Fatalf("team directory = %#v, error = %v", teamMembers, err)
		}
		channel, err = updateChannel.Execute(context.Background(), loggedIn.Identity, channel.ID, channelaction.MessageChannelInput{
			Name:                  channel.Name,
			DefaultLocale:         domain.LocaleEnglishUnitedStates,
			NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypeMember, ID: createdAgent.IdentityID},
			FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		})
		if err != nil || channel.InitialRoutingTargetID == nil || *channel.InitialRoutingTargetID != createdAgent.IdentityID {
			t.Fatalf("agent channel routing = %#v, error = %v", channel, err)
		}
		updatedAgent, err = agentaction.NewUpdateStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, domain.UserStatusInactive)
		if err != nil || updatedAgent.Status != domain.UserStatusInactive || updatedAgent.WorkStatus != domain.WorkStatusOffDuty {
			t.Fatalf("inactive agent = %#v, error = %v", updatedAgent, err)
		}
		teamMembersAfterAgentDeactivation, err := teamaction.NewListMembersQuery(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.MemberListInput{Page: 1, PageSize: 50})
		if err != nil || teamMembersAfterAgentDeactivation.Page.Total != 1 || len(teamMembersAfterAgentDeactivation.Members) != 1 || teamMembersAfterAgentDeactivation.Members[0].IdentityID != createdMember.IdentityID {
			t.Fatalf("team members after agent deactivation = %#v, error = %v", teamMembersAfterAgentDeactivation, err)
		}
		teamAfterAgentDeactivation, err := teamaction.NewUpdateTeamAction(db).Execute(context.Background(), loggedIn.Identity, team.ID, teamaction.Input{Name: team.Name, Description: team.Description})
		if err != nil || teamAfterAgentDeactivation.MemberCount != 1 {
			t.Fatalf("team after agent deactivation = %#v, error = %v", teamAfterAgentDeactivation, err)
		}
		if _, err := agentaction.NewUpdateWorkStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, agentaction.WorkStatusInput{WorkStatus: domain.WorkStatusWorking}); err == nil {
			t.Fatal("inactive agent work status update succeeded")
		} else {
			var fieldError *common.FieldError
			if !errors.As(err, &fieldError) || fieldError.Fields["workStatus"] != agentaction.ValidationWorkStatusUnavailable {
				t.Fatalf("inactive agent work status error = %#v", err)
			}
		}
		detail, err := getChannel.Execute(context.Background(), loggedIn.Identity, channel.ID)
		if err != nil || detail.InitialRoutingTargetType != string(domain.ChannelRoutingTargetTypePublicQueue) || detail.InitialRoutingTargetID != nil {
			t.Fatalf("channel routing after agent deactivation = %#v, error = %v", detail.MessageChannelRecord, err)
		}
		updatedAgent, err = agentaction.NewUpdateStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, domain.UserStatusActive)
		if err != nil {
			t.Fatal(err)
		}
		if updatedAgent.WorkStatus != domain.WorkStatusOffDuty {
			t.Fatalf("reactivated agent work status = %q, want %q", updatedAgent.WorkStatus, domain.WorkStatusOffDuty)
		}
		teamAfterAgentReactivation, err := teamaction.NewListTeamsQuery(db).Execute(context.Background(), loggedIn.Identity, teamaction.ListInput{Page: 1, PageSize: 50})
		if err != nil || len(teamAfterAgentReactivation.Teams) != 1 || teamAfterAgentReactivation.Teams[0].MemberCount != 2 {
			t.Fatalf("team after agent reactivation = %#v, error = %v", teamAfterAgentReactivation, err)
		}
		updatedAgent, err = agentaction.NewUpdateWorkStatusAction(db).Execute(context.Background(), loggedIn.Identity, createdAgent.ID, agentaction.WorkStatusInput{WorkStatus: domain.WorkStatusWorking})
		if err != nil || updatedAgent.WorkStatus != domain.WorkStatusWorking {
			t.Fatalf("working agent = %#v, error = %v", updatedAgent, err)
		}
		if _, err := useraction.NewUpdateUserAction(db).Execute(context.Background(), loggedIn.Identity, createdMember.ID, useraction.UpdateInput{
			DisplayName: createdMember.DisplayName, Email: createdMember.Email, RoleID: createdMember.RoleID, TeamIDs: []string{team.ID},
		}); err != nil {
			t.Fatal(err)
		}
		if err := teamaction.NewDeleteTeamAction(db).Execute(context.Background(), loggedIn.Identity, team.ID); err != nil {
			t.Fatal(err)
		}
		memberAfterTeamDelete, err := useraction.NewGetUserQuery(db).Execute(context.Background(), loggedIn.Identity, createdMember.ID)
		if err != nil || len(memberAfterTeamDelete.Teams) != 0 {
			t.Fatalf("member after team delete = %#v, error = %v", memberAfterTeamDelete, err)
		}
	})

	// 覆盖头像文件上传激活、个人资料更新、头像替换与过期清理流程。
	runStep("文件与个人资料", func(t *testing.T) {
		avatar, err := fileaction.NewCreateUploadAction(db).Execute(context.Background(), loggedIn.Identity, domain.FileStorageBackendLocal, fileaction.UploadInput{
			Purpose: domain.FilePurposeUserAvatar, FileName: "avatar.png", ContentType: "image/png", ByteSize: 1024,
		})
		if err != nil {
			t.Fatal(err)
		}
		if avatar.Status != string(domain.FileStatusPending) || avatar.ExpiresAt == nil {
			t.Fatalf("pending avatar = %#v", avatar)
		}
		avatar, err = fileaction.NewMarkUploadedAction(db).Execute(context.Background(), loggedIn.Identity, avatar.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		if avatar.Status != string(domain.FileStatusUploaded) || avatar.ExpiresAt == nil {
			t.Fatalf("uploaded avatar = %#v", avatar)
		}

		updatedIdentity, err := updateProfile.Execute(context.Background(), loggedIn.Identity, useraction.ProfileInput{
			DisplayName:  "  新姓名  ",
			Email:        " NEW@Example.com ",
			AvatarFileID: avatar.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if updatedIdentity.OrganizationIdentity.DisplayName != "新姓名" || updatedIdentity.User.Email != "new@example.com" || updatedIdentity.OrganizationIdentity.AvatarFileID == nil || *updatedIdentity.OrganizationIdentity.AvatarFileID != avatar.ID {
			t.Fatalf("updated identity = %#v", updatedIdentity)
		}
		activeAvatar := &servermodels.File{}
		if err := db.NewSelect().Model(activeAvatar).Where("f.id = ?", avatar.ID).Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if activeAvatar.Status != string(domain.FileStatusActive) || activeAvatar.ExpiresAt != nil {
			t.Fatalf("active avatar = %#v", activeAvatar)
		}
		resolvedAfterUpdate, err = resolveIdentity.Execute(context.Background(), loggedIn.Identity.Organization.ID, loggedIn.Token)
		if err != nil {
			t.Fatal(err)
		}
		if resolvedAfterUpdate == nil || resolvedAfterUpdate.User.Email != "new@example.com" || resolvedAfterUpdate.OrganizationIdentity.DisplayName != "新姓名" {
			t.Fatalf("identity after profile update = %#v", resolvedAfterUpdate)
		}
		replacement, err := fileaction.NewCreateUploadAction(db).Execute(context.Background(), resolvedAfterUpdate, domain.FileStorageBackendLocal, fileaction.UploadInput{
			Purpose: domain.FilePurposeUserAvatar, FileName: "replacement.webp", ContentType: "image/webp", ByteSize: 2048,
		})
		if err != nil {
			t.Fatal(err)
		}
		replacement, err = fileaction.NewMarkUploadedAction(db).Execute(context.Background(), resolvedAfterUpdate, replacement.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		updatedIdentity, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
			DisplayName: "新姓名", Email: "new@example.com", AvatarFileID: replacement.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if updatedIdentity.OrganizationIdentity.AvatarFileID == nil || *updatedIdentity.OrganizationIdentity.AvatarFileID != replacement.ID {
			t.Fatalf("replacement avatar identity = %#v", updatedIdentity)
		}
		if err := db.NewSelect().Model(activeAvatar).Where("f.id = ?", avatar.ID).Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if activeAvatar.Status != string(domain.FileStatusDeleting) || activeAvatar.ExpiresAt == nil {
			t.Fatalf("replaced avatar = %#v", activeAvatar)
		}
		if _, err := db.NewUpdate().Model((*servermodels.File)(nil)).
			Set("expires_at = ?", time.Now().UTC().Add(-time.Second)).
			Where("id = ?", avatar.ID).
			Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		localFiles, err := serverfilecontent.NewLocalStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		cleanup := fileaction.NewDeleteExpiredAction(db, serverfilecontent.NewDeleter(localFiles, nil))
		if err := cleanup.Execute(context.Background(), fileaction.DeleteExpiredInput{FileID: avatar.ID}); err != nil {
			t.Fatal(err)
		}
		resolvedAfterUpdate, err = resolveIdentity.Execute(context.Background(), loggedIn.Identity.Organization.ID, loggedIn.Token)
		if err != nil {
			t.Fatal(err)
		}
		_, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
			DisplayName: "不应保存的姓名", Email: "discarded@example.com", AvatarFileID: "00000000-0000-0000-0000-000000000099",
		})
		if !errors.Is(err, useraction.ErrAvatarFileNotFound) {
			t.Fatalf("invalid avatar error = %v, want file not found", err)
		}
		resolvedAfterUpdate, err = resolveIdentity.Execute(context.Background(), loggedIn.Identity.Organization.ID, loggedIn.Token)
		if err != nil {
			t.Fatal(err)
		}
		if resolvedAfterUpdate.User.Email != "new@example.com" || resolvedAfterUpdate.OrganizationIdentity.DisplayName != "新姓名" {
			t.Fatalf("profile changed after invalid avatar: %#v", resolvedAfterUpdate)
		}
	})

	// 覆盖修改密码校验与新旧密码登录验证。
	runStep("修改密码", func(t *testing.T) {
		changePassword := useraction.NewChangePasswordAction(db)
		err := changePassword.Execute(context.Background(), resolvedAfterUpdate, useraction.ChangePasswordInput{
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
		if _, err := login.Execute(context.Background(), authaction.LoginInput{OrganizationID: loggedIn.Identity.Organization.ID, Email: "new@example.com", Password: "password123"}); !errors.Is(err, authaction.ErrInvalidCredentials) {
			t.Fatalf("old password login error = %v, want invalid credentials", err)
		}
		if _, err := login.Execute(context.Background(), authaction.LoginInput{OrganizationID: loggedIn.Identity.Organization.ID, Email: "new@example.com", Password: "new-password123"}); err != nil {
			t.Fatalf("new password login error = %v", err)
		}
	})

	// 覆盖个人资料邮箱重复校验失败后头像文件保留并可重试的流程。
	runStep("邮箱冲突与头像重试", func(t *testing.T) {
		otherIdentity := &servermodels.OrganizationIdentity{
			OrganizationID: loggedIn.Identity.Organization.ID,
			Type:           string(domain.OrganizationIdentityTypeUser), DisplayName: "其他成员", WorkStatus: string(domain.WorkStatusWorking),
		}
		if _, err := db.NewInsert().Model(otherIdentity).
			Column("organization_id", "type", "display_name", "work_status").Returning("id").Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		otherUser := &servermodels.User{
			IdentityID:     otherIdentity.ID,
			OrganizationID: loggedIn.Identity.Organization.ID,
			Email:          "other@example.com",
			PasswordHash:   "unused",
			RoleID:         memberRole.ID,
			Status:         string(domain.UserStatusActive),
		}
		if _, err := db.NewInsert().Model(otherUser).
			Column("identity_id", "organization_id", "email", "password_hash", "role_id", "status").
			Exec(context.Background()); err != nil {
			t.Fatal(err)
		}
		retryAvatar, err := fileaction.NewCreateUploadAction(db).Execute(context.Background(), resolvedAfterUpdate, domain.FileStorageBackendLocal, fileaction.UploadInput{
			Purpose: domain.FilePurposeUserAvatar, FileName: "retry.png", ContentType: "image/png", ByteSize: 4096,
		})
		if err != nil {
			t.Fatal(err)
		}
		retryAvatar, err = fileaction.NewMarkUploadedAction(db).Execute(context.Background(), resolvedAfterUpdate, retryAvatar.ID, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
			DisplayName:  "新姓名",
			Email:        "OTHER@example.com",
			AvatarFileID: retryAvatar.ID,
		})
		var profileValidation *useraction.ValidationError
		if !errors.As(err, &profileValidation) || profileValidation.Fields["email"] != useraction.ValidationEmailDuplicate {
			t.Fatalf("duplicate profile email error = %v, want email validation", err)
		}
		if err := db.NewSelect().Model(retryAvatar).Where("f.id = ?", retryAvatar.ID).Scan(context.Background()); err != nil {
			t.Fatal(err)
		}
		if retryAvatar.Status != string(domain.FileStatusUploaded) || retryAvatar.ExpiresAt == nil {
			t.Fatalf("retry avatar after validation failure = %#v", retryAvatar)
		}
		updatedIdentity, err := updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
			DisplayName: "新姓名", Email: "new@example.com", AvatarFileID: retryAvatar.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if updatedIdentity.OrganizationIdentity.AvatarFileID == nil || *updatedIdentity.OrganizationIdentity.AvatarFileID != retryAvatar.ID {
			t.Fatalf("retried profile avatar = %#v", updatedIdentity)
		}
	})

	// 覆盖联系人的创建、联系方式保留、渠道不可变校验、软删除恢复与失效身份拦截。
	runStep("联系人管理", func(t *testing.T) {
		createContact := contactaction.NewCreateContactAction(db)
		_, err := createContact.Execute(context.Background(), loggedIn.Identity, contactaction.ContactInput{
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
		if _, err := login.Execute(context.Background(), authaction.LoginInput{OrganizationID: loggedIn.Identity.Organization.ID, Email: "new@example.com", Password: "new-password123"}); !errors.Is(err, authaction.ErrInvalidCredentials) {
			t.Fatalf("inactive user login error = %v, want invalid credentials", err)
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
	})

	// 覆盖 S3 对象存储设置的保存、读取与停用后配置保留。
	runStep("S3设置", func(t *testing.T) {
		s3Setting := settingaction.S3Setting{
			Enabled:         true,
			Provider:        domain.StorageProviderAWS,
			Endpoint:        "https://s3.example.com",
			PublicBaseURL:   "https://cdn.example.com",
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
	})
}

type telegramBotAPIFake struct {
	mu             sync.Mutex
	bot            telegramintegration.Bot
	getMeTokens    []string
	setWebhooks    []telegramintegration.Webhook
	deleteWebhooks []string
}

// GetMe 记录 Token 并返回测试机器人。
func (f *telegramBotAPIFake) GetMe(_ context.Context, token string) (telegramintegration.Bot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getMeTokens = append(f.getMeTokens, token)
	return f.bot, nil
}

// SetWebhook 记录最后一次注册参数。
func (f *telegramBotAPIFake) SetWebhook(_ context.Context, _ string, webhook telegramintegration.Webhook) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setWebhooks = append(f.setWebhooks, webhook)
	return nil
}

// DeleteWebhook 记录被清理的 Token。
func (f *telegramBotAPIFake) DeleteWebhook(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteWebhooks = append(f.deleteWebhooks, token)
	return nil
}

// webhooks 返回注册调用快照。
func (f *telegramBotAPIFake) webhooks() []telegramintegration.Webhook {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]telegramintegration.Webhook(nil), f.setWebhooks...)
}

// deletedTokens 返回删除调用快照。
func (f *telegramBotAPIFake) deletedTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleteWebhooks...)
}

var _ telegramintegration.BotAPI = (*telegramBotAPIFake)(nil)

type telegramProfilePhotoAPIStub struct {
	photo      *telegramintegration.ProfilePhoto
	downloaded telegramintegration.DownloadedPhoto
	err        error
}

// GetUserProfilePhoto 返回测试预设的当前头像。
func (s *telegramProfilePhotoAPIStub) GetUserProfilePhoto(context.Context, string, int64) (*telegramintegration.ProfilePhoto, error) {
	return s.photo, s.err
}

// DownloadPhoto 返回测试预设的头像内容。
func (s *telegramProfilePhotoAPIStub) DownloadPhoto(context.Context, string, string) (telegramintegration.DownloadedPhoto, error) {
	return s.downloaded, s.err
}

var _ telegramintegration.ProfilePhotoAPI = (*telegramProfilePhotoAPIStub)(nil)

type importedFileWriterStub struct {
	saved int
}

// Save 接受测试导入内容并返回固定 ETag。
func (s *importedFileWriterStub) Save(context.Context, *servermodels.File, []byte) (string, error) {
	s.saved++
	return "telegram-avatar-etag", nil
}

var _ fileaction.ContentWriter = (*importedFileWriterStub)(nil)

// testDatabaseConfig 从测试专用的 PostgreSQL 分项环境变量读取连接配置。
func testDatabaseConfig(t *testing.T) serverconfig.DatabaseConfig {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	port, err := strconv.Atoi(os.Getenv("TEST_POSTGRES_PORT"))
	if err != nil {
		t.Fatalf("TEST_POSTGRES_PORT is invalid: %v", err)
	}
	return serverconfig.DatabaseConfig{
		Host:     host,
		Port:     port,
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
		Name:     os.Getenv("TEST_POSTGRES_DB"),
		SSLMode:  os.Getenv("TEST_POSTGRES_SSLMODE"),
	}
}
