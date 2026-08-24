//go:build server

package server

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestServerActionsWithPostgreSQL 验证服务端核心操作。
func TestServerActionsWithPostgreSQL(t *testing.T) {
	databaseConfig := testDatabaseConfig(t)
	store, err := Open(context.Background(), databaseConfig)
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
	if identity == nil || identity.User.Email != "admin@example.com" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	if _, err := useraction.NewUpdateWorkStatusAction(db).Execute(context.Background(), identity, useraction.WorkStatusInput{WorkStatus: domain.WorkStatusAway}); err != nil {
		t.Fatal(err)
	}

	logout := authaction.NewLogoutAction(db)
	if err := logout.Execute(context.Background(), installed.Token); err != nil {
		t.Fatal(err)
	}

	login := authaction.NewLoginAction(db)
	loggedIn, err := login.Execute(context.Background(), authaction.LoginInput{
		Email:    "ADMIN@example.com",
		Password: "password123",
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
	updatedPreferences, err := useraction.NewUpdatePreferencesAction(db).Execute(context.Background(), loggedIn.Identity, useraction.PreferencesInput{
		Locale:                      domain.LocaleEnglishUnitedStates,
		TimeZone:                    "America/New_York",
		MessageNotificationsEnabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedPreferences.User.MessageNotificationsEnabled {
		t.Fatal("message notifications enabled = true, want false")
	}
	loggedIn.Identity = updatedPreferences
	resolvedPreferences, err := resolveIdentity.Execute(context.Background(), loggedIn.Token)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedPreferences == nil || resolvedPreferences.User.MessageNotificationsEnabled {
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

	updateOrganization := organizationaction.NewUpdateOrganizationAction(db)
	organization, err := updateOrganization.Execute(context.Background(), loggedIn.Identity, organizationaction.Input{Name: "  鹿行协作  "})
	if err != nil {
		t.Fatal(err)
	}
	if organization.Name != "鹿行协作" {
		t.Fatalf("updated organization name = %q, want 鹿行协作", organization.Name)
	}
	currentStatus, err = status.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if currentStatus.OrganizationName != "鹿行协作" {
		t.Fatalf("status organization name = %q, want 鹿行协作", currentStatus.OrganizationName)
	}

	createChannel := channelaction.NewCreateWebsiteChannelAction(db)
	staleIdentity := *loggedIn.Identity
	staleIdentity.User = loggedIn.Identity.User
	staleIdentity.User.ID = "00000000-0000-0000-0000-000000000000"
	_, err = createChannel.Execute(context.Background(), &staleIdentity, channelaction.WebsiteChannelInput{
		Type:                  domain.ChannelTypeWebsite,
		Name:                  "无效渠道",
		DefaultLocale:         domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
	})
	if !errors.Is(err, common.ErrIdentityInvalid) {
		t.Fatalf("stale identity error = %v, want %v", err, common.ErrIdentityInvalid)
	}

	channel, err := createChannel.Execute(context.Background(), loggedIn.Identity, channelaction.WebsiteChannelInput{
		Type:                  domain.ChannelTypeWebsite,
		Name:                  "产品官网",
		Description:           "接收官网访客咨询",
		DefaultLocale:         domain.LocaleChineseSimplified,
		NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
		FallbackTarget:        channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
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
		Type:                  domain.ChannelTypeWebsite,
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

	updateChannelStatus := channelaction.NewUpdateWebsiteChannelStatusAction(db)
	channel, err = updateChannelStatus.Execute(context.Background(), loggedIn.Identity, channel.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if channel.Enabled {
		t.Fatal("channel enabled = true, want false")
	}
	listChannels := channelaction.NewListWebsiteChannelsQuery(db)
	channels, err := listChannels.Execute(context.Background(), loggedIn.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Enabled {
		t.Fatalf("unexpected disabled channels: %#v", channels)
	}

	channel, err = updateChannelStatus.Execute(context.Background(), loggedIn.Identity, channel.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !channel.Enabled {
		t.Fatal("channel enabled = false, want true")
	}

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

	team, err := teamaction.NewCreateTeamAction(db).Execute(context.Background(), loggedIn.Identity, teamaction.Input{Name: "客户成功", Description: "服务客户"})
	if err != nil {
		t.Fatal(err)
	}
	memberRole := &servermodels.Role{}
	if err := db.NewSelect().Model(memberRole).
		Where("organization_id = ?", loggedIn.Identity.Organization.ID).
		Where("kind = ?", domain.RoleKindMember).
		Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	createdMember, err := useraction.NewCreateUserAction(db).Execute(context.Background(), loggedIn.Identity, useraction.CreateInput{
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
	createdAgent, err := agentaction.NewCreateAgentAction(db).Execute(context.Background(), loggedIn.Identity, agentaction.CreateInput{
		DisplayName: "接待智能体",
		TeamIDs:     []string{team.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(createdAgent.Teams) != 1 || createdAgent.Teams[0].ID != team.ID || createdAgent.CreatedAt.IsZero() {
		t.Fatalf("created agent = %#v", createdAgent)
	}
	if createdAgent.IdentityID == "" || createdAgent.IdentityID == createdAgent.ID {
		t.Fatalf("agent identity id = %q, agent id = %q", createdAgent.IdentityID, createdAgent.ID)
	}
	agents, err := agentaction.NewListAgentsQuery(db).Execute(context.Background(), loggedIn.Identity, agentaction.ListInput{Page: 1, PageSize: 50})
	if err != nil || agents.Total != 1 || len(agents.Agents) != 1 || agents.Agents[0].ID != createdAgent.ID {
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
	channel, err = updateChannel.Execute(context.Background(), loggedIn.Identity, channel.ID, channelaction.WebsiteChannelInput{
		Type:                  domain.ChannelTypeWebsite,
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
	detail, err = getChannel.Execute(context.Background(), loggedIn.Identity, channel.ID)
	if err != nil || detail.InitialRoutingTargetType != string(domain.ChannelRoutingTargetTypePublicQueue) || detail.InitialRoutingTargetID != nil {
		t.Fatalf("channel routing after agent deactivation = %#v, error = %v", detail.Channel, err)
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

	updateProfile := useraction.NewUpdateProfileAction(db)
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
	resolvedAfterUpdate, err := resolveIdentity.Execute(context.Background(), loggedIn.Token)
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
	resolvedAfterUpdate, err = resolveIdentity.Execute(context.Background(), loggedIn.Token)
	if err != nil {
		t.Fatal(err)
	}
	_, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
		DisplayName: "不应保存的姓名", Email: "discarded@example.com", AvatarFileID: "00000000-0000-0000-0000-000000000099",
	})
	if !errors.Is(err, useraction.ErrAvatarFileNotFound) {
		t.Fatalf("invalid avatar error = %v, want file not found", err)
	}
	resolvedAfterUpdate, err = resolveIdentity.Execute(context.Background(), loggedIn.Token)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedAfterUpdate.User.Email != "new@example.com" || resolvedAfterUpdate.OrganizationIdentity.DisplayName != "新姓名" {
		t.Fatalf("profile changed after invalid avatar: %#v", resolvedAfterUpdate)
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
	updatedIdentity, err = updateProfile.Execute(context.Background(), resolvedAfterUpdate, useraction.ProfileInput{
		DisplayName: "新姓名", Email: "new@example.com", AvatarFileID: retryAvatar.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedIdentity.OrganizationIdentity.AvatarFileID == nil || *updatedIdentity.OrganizationIdentity.AvatarFileID != retryAvatar.ID {
		t.Fatalf("retried profile avatar = %#v", updatedIdentity)
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
	if _, err := login.Execute(context.Background(), authaction.LoginInput{Email: "new@example.com", Password: "new-password123"}); !errors.Is(err, authaction.ErrInvalidCredentials) {
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
