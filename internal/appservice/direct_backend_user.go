//go:build server

package appservice

import (
	"context"
	"errors"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	"log/slog"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	memberaction "github.com/runforyou-ai/cervi/internal/actions/member"
	organizationaction "github.com/runforyou-ai/cervi/internal/actions/organization"
	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// directoryOps 持有企业成员、团队、角色与组织的 Action 和 Query。
type directoryOps struct {
	listMemberOptions        *memberaction.ListOptionsQuery
	listUsers                *useraction.ListUsersQuery
	getUser                  *useraction.GetUserQuery
	createUser               *useraction.CreateUserAction
	updateUser               *useraction.UpdateUserAction
	updateRoleAssignments    *roleaction.UpdateAssignmentsAction
	updateUserStatus         *useraction.UpdateStatusAction
	listTeams                *teamaction.ListTeamsQuery
	getTeam                  *teamaction.GetTeamQuery
	createTeam               *teamaction.CreateTeamAction
	updateTeam               *teamaction.UpdateTeamAction
	deleteTeam               *teamaction.DeleteTeamAction
	listTeamMembers          *teamaction.ListMembersQuery
	listTeamMemberCandidates *teamaction.ListMemberCandidatesQuery
	addTeamMembers           *teamaction.AddMembersAction
	removeTeamMembers        *teamaction.RemoveMembersAction
	updateProfile            *useraction.UpdateProfileAction
	changePassword           *useraction.ChangePasswordAction
	updateUserPreferences    *useraction.UpdatePreferencesAction
	updateUserWorkStatus     *useraction.UpdateWorkStatusAction
	listRoles                *roleaction.ListRolesQuery
	getRole                  *roleaction.GetRoleQuery
	createRole               *roleaction.CreateRoleAction
	updateRole               *roleaction.UpdateRoleAction
	deleteRole               *roleaction.DeleteRoleAction
	updateOrganization       *organizationaction.UpdateOrganizationAction
}

// newDirectoryOps 创建企业成员、团队、角色与组织的业务实现依赖。
func newDirectoryOps(db *bun.DB, agentCoordinator *agentrunaction.ExecuteAction, taskEnqueuer servertask.TxEnqueuer) directoryOps {
	return directoryOps{
		listMemberOptions:        memberaction.NewListOptionsQuery(db),
		listUsers:                useraction.NewListUsersQuery(db),
		getUser:                  useraction.NewGetUserQuery(db),
		createUser:               useraction.NewCreateUserAction(db, taskEnqueuer),
		updateUser:               useraction.NewUpdateUserAction(db, agentCoordinator, taskEnqueuer),
		updateRoleAssignments:    roleaction.NewUpdateAssignmentsAction(db),
		updateUserStatus:         useraction.NewUpdateStatusAction(db, agentCoordinator),
		listTeams:                teamaction.NewListTeamsQuery(db),
		getTeam:                  teamaction.NewGetTeamQuery(db),
		createTeam:               teamaction.NewCreateTeamAction(db),
		updateTeam:               teamaction.NewUpdateTeamAction(db),
		deleteTeam:               teamaction.NewDeleteTeamAction(db, taskEnqueuer),
		listTeamMembers:          teamaction.NewListMembersQuery(db),
		listTeamMemberCandidates: teamaction.NewListMemberCandidatesQuery(db),
		addTeamMembers:           teamaction.NewAddMembersAction(db, taskEnqueuer),
		removeTeamMembers:        teamaction.NewRemoveMembersAction(db),
		updateProfile:            useraction.NewUpdateProfileAction(db),
		changePassword:           useraction.NewChangePasswordAction(db),
		updateUserPreferences:    useraction.NewUpdatePreferencesAction(db),
		updateUserWorkStatus:     useraction.NewUpdateWorkStatusAction(db, taskEnqueuer),
		listRoles:                roleaction.NewListRolesQuery(db),
		getRole:                  roleaction.NewGetRoleQuery(db),
		createRole:               roleaction.NewCreateRoleAction(db),
		updateRole:               roleaction.NewUpdateRoleAction(db),
		deleteRole:               roleaction.NewDeleteRoleAction(db),
		updateOrganization:       organizationaction.NewUpdateOrganizationAction(db),
	}
}

// UpdateProfile 修改当前用户的头像、姓名和邮箱。
func (o *directOperations) UpdateProfile(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ProfileInput) (CurrentUser, error) {
	updatedIdentity, err := o.updateProfile.Execute(ctx, identity, useraction.ProfileInput{
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		AvatarFileID: input.AvatarFileID,
	})
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorProfileUpdateFailed, profileFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	slog.Info("个人资料保存成功", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID)
	user, err := o.currentUserFromIdentity(ctx, updatedIdentity)
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorProfileUpdateFailed, profileFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	return user, nil
}

// ChangePassword 核验当前密码并保存新密码。
func (o *directOperations) ChangePassword(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ChangePasswordInput) error {
	err := o.changePassword.Execute(ctx, identity, useraction.ChangePasswordInput{
		CurrentPassword: input.CurrentPassword,
		NewPassword:     input.NewPassword,
	})
	if err != nil {
		return o.currentUserError(ctx, meta, err, cervii18n.ErrorPasswordUpdateFailed,
			// 把密码校验错误码映射为本地化文案键。
			func(fields map[string]common.FieldCode) map[string]cervii18n.Key {
				keys := map[common.FieldCode]cervii18n.Key{
					useraction.ValidationCurrentPasswordIncorrect: cervii18n.FieldCurrentPasswordIncorrect,
					useraction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
					useraction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
				}
				return translateValidationFields(fields, keys)
			}, identity.Organization.ID, identity.User.ID)
	}
	slog.Info("密码修改成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return nil
}

// UpdateUserPreferences 保存当前用户的偏好设置。
func (o *directOperations) UpdateUserPreferences(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input UserPreferencesInput) (CurrentUser, error) {
	updatedIdentity, err := o.updateUserPreferences.Execute(ctx, identity, useraction.PreferencesInput{
		Locale:                      domain.Locale(input.Locale),
		TimeZone:                    input.TimeZone,
		MessageNotificationsEnabled: input.MessageNotificationsEnabled,
	})
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorPreferencesUpdateFailed, preferencesFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	slog.Info("用户偏好保存成功",
		"organization_id", identity.Organization.ID,
		"user_id", identity.User.ID,
		"locale", input.Locale,
		"time_zone", input.TimeZone,
		"message_notifications_enabled", input.MessageNotificationsEnabled,
	)
	user, err := o.currentUserFromIdentity(ctx, updatedIdentity)
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorPreferencesUpdateFailed, preferencesFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	return user, nil
}

// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
func (o *directOperations) UpdateUserWorkStatus(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input UserWorkStatusInput) (CurrentUser, error) {
	updatedIdentity, err := o.updateUserWorkStatus.Execute(ctx, identity, useraction.WorkStatusInput{
		WorkStatus: domain.WorkStatus(input.WorkStatus),
	})
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorWorkStatusUpdateFailed, workStatusFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	slog.Info("工作状态保存成功", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID, "work_status", input.WorkStatus)
	user, err := o.currentUserFromIdentity(ctx, updatedIdentity)
	if err != nil {
		return CurrentUser{}, o.currentUserError(ctx, meta, err, cervii18n.ErrorWorkStatusUpdateFailed, workStatusFieldKeys, identity.Organization.ID, identity.User.ID)
	}
	return user, nil
}

// ListUsers 返回企业成员列表。
func (o *directOperations) ListUsers(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input UserListInput) (UserList, error) {
	output, err := o.listUsers.Execute(ctx, identity, useraction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), RoleID: input.RoleID, TeamID: input.TeamID, Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		if ctx.Err() != nil {
			return UserList{}, ctx.Err()
		}
		if errors.Is(err, useraction.ErrQueryInvalid) {
			return UserList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
		}
		slog.Warn("读取企业成员列表失败", "organization_id", identity.Organization.ID, "error", err)
		return UserList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	avatarFileIDs := make([]*string, 0, len(output.Users))
	for _, user := range output.Users {
		avatarFileIDs = append(avatarFileIDs, user.AvatarFileID)
	}
	avatarURLs, err := o.optionalFileURLs(ctx, identity, avatarFileIDs...)
	if err != nil {
		if ctx.Err() != nil {
			return UserList{}, ctx.Err()
		}
		slog.Warn("读取企业成员头像失败", "organization_id", identity.Organization.ID, "error", err)
		return UserList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	users := make([]User, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, userFromAction(user, avatarURLs))
	}
	return UserList{Users: users, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetUser 返回企业成员详情。
func (o *directOperations) GetUser(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string) (User, error) {
	user, err := o.getUser.Execute(ctx, identity, userID)
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		if errors.Is(err, useraction.ErrNotFound) {
			return User{}, NotFoundError(meta, cervii18n.ErrorUserNotFound)
		}
		slog.Warn("读取企业成员失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	output, err := o.userWithAvatar(ctx, identity, *user)
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		slog.Warn("读取企业成员头像失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	return output, nil
}

// CreateUser 创建企业成员账号。
func (o *directOperations) CreateUser(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input CreateUserInput) (User, error) {
	user, err := o.createUser.Execute(ctx, identity, useraction.CreateInput{DisplayName: input.DisplayName, Email: input.Email, Password: input.Password, RoleID: input.RoleID, TeamIDs: input.TeamIDs, HandlesCustomers: input.HandlesCustomers, MaxServiceSessions: input.MaxServiceSessions, AvatarFileID: input.AvatarFileID})
	if err != nil {
		return User{}, o.userMutationError(ctx, meta, err, cervii18n.ErrorUserCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("企业成员创建成功", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", user.ID, "role_id", user.RoleID)
	return o.userMutationResult(ctx, meta, identity, *user, cervii18n.ErrorUserCreateFailed)
}

// UpdateUser 修改企业成员资料、角色、接待开关和所属团队。
func (o *directOperations) UpdateUser(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string, input UpdateUserInput) (User, error) {
	user, err := o.updateUser.Execute(ctx, identity, userID, useraction.UpdateInput{DisplayName: input.DisplayName, Email: input.Email, RoleID: input.RoleID, TeamIDs: input.TeamIDs, HandlesCustomers: input.HandlesCustomers, MaxServiceSessions: input.MaxServiceSessions})
	if err != nil {
		return User{}, o.userMutationError(ctx, meta, err, cervii18n.ErrorUserUpdateFailed, identity.Organization.ID, userID)
	}
	slog.Info("企业成员更新成功", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", userID, "role_id", user.RoleID)
	return o.userMutationResult(ctx, meta, identity, *user, cervii18n.ErrorUserUpdateFailed)
}

// DeactivateUser 禁用企业成员账号。
func (o *directOperations) DeactivateUser(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string) (User, error) {
	return o.changeUserStatus(ctx, meta, identity, userID, domain.UserStatusInactive)
}

// ReactivateUser 恢复企业成员账号。
func (o *directOperations) ReactivateUser(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string) (User, error) {
	return o.changeUserStatus(ctx, meta, identity, userID, domain.UserStatusActive)
}

// changeUserStatus 修改企业成员账号状态。
func (o *directOperations) changeUserStatus(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, userID string, status domain.UserStatus) (User, error) {
	user, err := o.updateUserStatus.Execute(ctx, identity, userID, status)
	if err != nil {
		return User{}, o.userMutationError(ctx, meta, err, cervii18n.ErrorUserStatusUpdateFailed, identity.Organization.ID, userID)
	}
	slog.Info("企业成员账号状态已修改", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", userID, "status", status)
	return o.userMutationResult(ctx, meta, identity, *user, cervii18n.ErrorUserStatusUpdateFailed)
}

// currentUserError 转换当前用户资料、密码、偏好和工作状态操作错误。
func (o *directOperations) currentUserError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, fieldKeys func(map[string]common.FieldCode) map[string]cervii18n.Key, organizationID, userID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, fieldKeys(validationError.Fields))
	}
	if errors.Is(err, useraction.ErrAvatarFileNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	slog.Warn("当前用户操作失败", "organization_id", organizationID, "user_id", userID, "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

// userMutationError 转换企业成员写入错误。
func (o *directOperations) userMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, userID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, userFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, useraction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorUserNotFound)
	}
	if errors.Is(err, fileaction.ErrLinkedImageNotFound) {
		return NotFoundError(meta, cervii18n.ErrorFileNotFound)
	}
	if errors.Is(err, useraction.ErrLastActiveAdministrator) {
		return InvalidError(meta, cervii18n.ErrorUserLastActiveAdministrator, nil)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if userID != "" {
		attributes = append(attributes, "user_id", userID)
	}
	slog.Warn("企业成员操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// userMutationResult 补齐写入后企业成员的头像地址。
func (o *directOperations) userMutationResult(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, user useraction.User, failureKey cervii18n.Key) (User, error) {
	output, err := o.userWithAvatar(ctx, identity, user)
	if err != nil {
		return User{}, o.userMutationError(ctx, meta, err, failureKey, identity.Organization.ID, user.ID)
	}
	return output, nil
}

// userWithAvatar 解析单个企业成员的头像地址并转换契约。
func (o *directOperations) userWithAvatar(ctx context.Context, identity *servermodels.Identity, user useraction.User) (User, error) {
	avatarURLs, err := o.optionalFileURLs(ctx, identity, user.AvatarFileID)
	if err != nil {
		return User{}, err
	}
	return userFromAction(user, avatarURLs), nil
}

// userFromAction 转换企业成员契约。
func userFromAction(user useraction.User, avatarURLs map[string]string) User {
	teams := make([]TeamSummary, 0, len(user.Teams))
	for _, team := range user.Teams {
		teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
	}
	return User{ID: user.ID, IdentityID: user.IdentityID, Email: user.Email, DisplayName: user.DisplayName, AvatarURL: optionalFileURL(avatarURLs, user.AvatarFileID), Role: RoleSummary{ID: user.RoleID, Kind: RoleKind(user.RoleKind), Name: user.RoleName}, HandlesCustomers: user.HandlesCustomers, MaxServiceSessions: user.MaxServiceSessions, Status: UserStatus(user.Status), WorkStatus: WorkStatus(user.WorkStatus), Teams: teams, CreatedAt: user.CreatedAt}
}

// userFieldKeys 把企业成员校验错误码映射为本地化文案键。
func userFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationDisplayNameRequired:       cervii18n.FieldDisplayNameRequired,
		useraction.ValidationDisplayNameInvalid:        cervii18n.FieldDisplayNameInvalid,
		useraction.ValidationEmailInvalid:              cervii18n.FieldEmailInvalid,
		useraction.ValidationEmailDuplicate:            cervii18n.FieldEmailDuplicate,
		useraction.ValidationPasswordTooShort:          cervii18n.FieldPasswordTooShort,
		useraction.ValidationPasswordTooLong:           cervii18n.FieldPasswordTooLong,
		useraction.ValidationRoleInvalid:               cervii18n.FieldMemberRoleInvalid,
		useraction.ValidationTeamInvalid:               cervii18n.FieldTeamInvalid,
		useraction.ValidationStatusInvalid:             cervii18n.FieldUserStatusInvalid,
		useraction.ValidationMaxServiceSessionsInvalid: cervii18n.FieldMaxServiceSessionsInvalid,
	}
	return translateValidationFields(fields, keys)
}

// profileFieldKeys 把个人资料校验错误码映射为本地化文案键。
func profileFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationDisplayNameRequired: cervii18n.FieldDisplayNameRequired,
		useraction.ValidationDisplayNameInvalid:  cervii18n.FieldDisplayNameInvalid,
		useraction.ValidationEmailInvalid:        cervii18n.FieldEmailInvalid,
		useraction.ValidationEmailDuplicate:      cervii18n.FieldEmailDuplicate,
	}
	return translateValidationFields(fields, keys)
}

// preferencesFieldKeys 把语言和时区校验错误码映射为本地化文案键。
func preferencesFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationLocaleInvalid:   cervii18n.FieldLocaleInvalid,
		useraction.ValidationTimeZoneInvalid: cervii18n.FieldTimeZoneInvalid,
	}
	return translateValidationFields(fields, keys)
}

// workStatusFieldKeys 把工作状态校验错误码映射为本地化文案键。
func workStatusFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationWorkStatusInvalid: cervii18n.FieldWorkStatusInvalid,
	}
	return translateValidationFields(fields, keys)
}
