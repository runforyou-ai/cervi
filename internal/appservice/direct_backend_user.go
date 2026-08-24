//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// UpdateProfile 修改当前用户的头像、姓名和邮箱。
func (b *DirectBackend) UpdateProfile(ctx context.Context, meta RequestMeta, input ProfileInput) (CurrentUser, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CurrentUser{}, err
	}
	updatedIdentity, err := b.updateProfile.Execute(ctx, identity, useraction.ProfileInput{
		DisplayName:  input.DisplayName,
		Email:        input.Email,
		AvatarFileID: input.AvatarFileID,
	})
	if err != nil {
		if ctx.Err() != nil {
			return CurrentUser{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return CurrentUser{}, InvalidError(meta, cervii18n.ErrorValidationFailed, profileFieldKeys(validationError.Fields))
		}
		if errors.Is(err, useraction.ErrAvatarFileNotFound) {
			return CurrentUser{}, NotFoundError(meta, cervii18n.ErrorFileNotFound)
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return CurrentUser{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存个人资料失败", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID, "error", err)
		return CurrentUser{}, FailedError(meta, cervii18n.ErrorProfileUpdateFailed)
	}
	slog.Info("个人资料保存成功", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID)
	return currentUserFromIdentity(updatedIdentity), nil
}

// ChangePassword 核验当前密码并保存新密码。
func (b *DirectBackend) ChangePassword(ctx context.Context, meta RequestMeta, input ChangePasswordInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	err = b.changePassword.Execute(ctx, identity, useraction.ChangePasswordInput{
		CurrentPassword: input.CurrentPassword,
		NewPassword:     input.NewPassword,
	})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return InvalidError(meta, cervii18n.ErrorValidationFailed, passwordFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("修改密码失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return FailedError(meta, cervii18n.ErrorPasswordUpdateFailed)
	}
	slog.Info("密码修改成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return nil
}

// UpdateUserPreferences 保存当前用户的偏好设置。
func (b *DirectBackend) UpdateUserPreferences(ctx context.Context, meta RequestMeta, input UserPreferencesInput) (CurrentUser, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CurrentUser{}, err
	}
	updatedIdentity, err := b.updateUserPreferences.Execute(ctx, identity, useraction.PreferencesInput{
		Locale: domain.Locale(input.Locale), TimeZone: input.TimeZone, MessageNotificationsEnabled: input.MessageNotificationsEnabled,
	})
	if err != nil {
		if ctx.Err() != nil {
			return CurrentUser{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return CurrentUser{}, InvalidError(meta, cervii18n.ErrorValidationFailed, preferencesFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return CurrentUser{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存用户偏好失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "locale", input.Locale, "time_zone", input.TimeZone, "message_notifications_enabled", input.MessageNotificationsEnabled, "error", err)
		return CurrentUser{}, FailedError(meta, cervii18n.ErrorPreferencesUpdateFailed)
	}
	slog.Info("用户偏好保存成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "locale", input.Locale, "time_zone", input.TimeZone, "message_notifications_enabled", input.MessageNotificationsEnabled)
	return currentUserFromIdentity(updatedIdentity), nil
}

// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
func (b *DirectBackend) UpdateUserWorkStatus(ctx context.Context, meta RequestMeta, input UserWorkStatusInput) (CurrentUser, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return CurrentUser{}, err
	}
	updatedIdentity, err := b.updateUserWorkStatus.Execute(ctx, identity, useraction.WorkStatusInput{
		WorkStatus: domain.WorkStatus(input.WorkStatus),
	})
	if err != nil {
		if ctx.Err() != nil {
			return CurrentUser{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return CurrentUser{}, InvalidError(meta, cervii18n.ErrorValidationFailed, workStatusFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return CurrentUser{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存工作状态失败", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID, "error", err)
		return CurrentUser{}, FailedError(meta, cervii18n.ErrorWorkStatusUpdateFailed)
	}
	slog.Info("工作状态保存成功", "organization_id", identity.Organization.ID, "identity_id", identity.User.IdentityID, "user_id", identity.User.ID, "work_status", input.WorkStatus)
	return currentUserFromIdentity(updatedIdentity), nil
}

// ListUsers 返回企业成员列表。
func (b *DirectBackend) ListUsers(ctx context.Context, meta RequestMeta, input UserListInput) (UserList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return UserList{}, err
	}
	output, err := b.listUsers.Execute(ctx, identity, useraction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), RoleID: input.RoleID, TeamID: input.TeamID, Page: input.Page, PageSize: input.PageSize,
	})
	if errors.Is(err, useraction.ErrQueryInvalid) {
		return UserList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return UserList{}, ctx.Err()
		}
		slog.Warn("读取企业成员列表失败", "organization_id", identity.Organization.ID, "error", err)
		return UserList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	users := make([]User, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, userFromAction(user))
	}
	return UserList{Users: users, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetUser 返回企业成员详情。
func (b *DirectBackend) GetUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.getUser.Execute(ctx, identity, userID)
	if errors.Is(err, useraction.ErrNotFound) {
		return User{}, NotFoundError(meta, cervii18n.ErrorUserNotFound)
	}
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		slog.Warn("读取企业成员失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	return userFromAction(*user), nil
}

// CreateUser 创建企业成员账号。
func (b *DirectBackend) CreateUser(ctx context.Context, meta RequestMeta, input CreateUserInput) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.createUser.Execute(ctx, identity, useraction.CreateInput{DisplayName: input.DisplayName, Email: input.Email, Password: input.Password, RoleID: input.RoleID, TeamIDs: input.TeamIDs})
	if err != nil {
		return User{}, b.userMutationError(ctx, meta, err, cervii18n.ErrorUserCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("企业成员创建成功", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", user.ID, "role_id", user.RoleID)
	return userFromAction(*user), nil
}

// UpdateUser 修改企业成员资料、角色和所属团队。
func (b *DirectBackend) UpdateUser(ctx context.Context, meta RequestMeta, userID string, input UpdateUserInput) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.updateUser.Execute(ctx, identity, userID, useraction.UpdateInput{DisplayName: input.DisplayName, Email: input.Email, RoleID: input.RoleID, TeamIDs: input.TeamIDs})
	if err != nil {
		return User{}, b.userMutationError(ctx, meta, err, cervii18n.ErrorUserUpdateFailed, identity.Organization.ID, userID)
	}
	slog.Info("企业成员更新成功", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", userID, "role_id", user.RoleID)
	return userFromAction(*user), nil
}

// UpdateUserRoles 在一个事务中批量调整企业成员角色。
func (b *DirectBackend) UpdateUserRoles(ctx context.Context, meta RequestMeta, input UserRoleChangesInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	changes := make([]useraction.RoleChangeInput, 0, len(input.Changes))
	for _, change := range input.Changes {
		changes = append(changes, useraction.RoleChangeInput{UserID: change.UserID, RoleID: change.RoleID})
	}
	if err := b.updateUserRoles.Execute(ctx, identity, changes); err != nil {
		return b.userMutationError(ctx, meta, err, cervii18n.ErrorUserUpdateFailed, identity.Organization.ID, "")
	}
	slog.Info("企业成员角色批量调整成功", "organization_id", identity.Organization.ID, "change_count", len(changes))
	return nil
}

// DeactivateUser 禁用企业成员账号。
func (b *DirectBackend) DeactivateUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	return b.changeUserStatus(ctx, meta, userID, domain.UserStatusInactive)
}

// ReactivateUser 恢复企业成员账号。
func (b *DirectBackend) ReactivateUser(ctx context.Context, meta RequestMeta, userID string) (User, error) {
	return b.changeUserStatus(ctx, meta, userID, domain.UserStatusActive)
}

// changeUserStatus 修改企业成员账号状态。
func (b *DirectBackend) changeUserStatus(ctx context.Context, meta RequestMeta, userID string, status domain.UserStatus) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.updateUserStatus.Execute(ctx, identity, userID, status)
	if err != nil {
		return User{}, b.userMutationError(ctx, meta, err, cervii18n.ErrorUserStatusUpdateFailed, identity.Organization.ID, userID)
	}
	slog.Info("企业成员账号状态已修改", "organization_id", identity.Organization.ID, "identity_id", user.IdentityID, "user_id", userID, "status", status)
	return userFromAction(*user), nil
}

// userMutationError 转换企业成员写入错误。
func (b *DirectBackend) userMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, userID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, userFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, useraction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorUserNotFound)
	}
	if errors.Is(err, useraction.ErrLastActiveAdministrator) {
		return InvalidError(meta, cervii18n.ErrorUserLastActiveAdministrator, nil)
	}
	if errors.Is(err, useraction.ErrRoleChangesInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if userID != "" {
		attributes = append(attributes, "user_id", userID)
	}
	slog.Warn("企业成员操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// userFromAction 转换企业成员契约。
func userFromAction(user useraction.User) User {
	teams := make([]TeamSummary, 0, len(user.Teams))
	for _, team := range user.Teams {
		teams = append(teams, TeamSummary{ID: team.ID, Name: team.Name})
	}
	return User{ID: user.ID, IdentityID: user.IdentityID, Email: user.Email, DisplayName: user.DisplayName, Role: RoleSummary{ID: user.RoleID, Kind: RoleKind(user.RoleKind), Name: user.RoleName}, Status: UserStatus(user.Status), WorkStatus: WorkStatus(user.WorkStatus), Teams: teams, CreatedAt: user.CreatedAt}
}

// userFieldKeys 把企业成员校验错误码映射为本地化文案键。
func userFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationDisplayNameRequired: cervii18n.FieldDisplayNameRequired,
		useraction.ValidationEmailInvalid:        cervii18n.FieldEmailInvalid,
		useraction.ValidationEmailDuplicate:      cervii18n.FieldEmailDuplicate,
		useraction.ValidationPasswordTooShort:    cervii18n.FieldPasswordTooShort,
		useraction.ValidationPasswordTooLong:     cervii18n.FieldPasswordTooLong,
		useraction.ValidationRoleInvalid:         cervii18n.FieldMemberRoleInvalid,
		useraction.ValidationTeamInvalid:         cervii18n.FieldUserTeamInvalid,
		useraction.ValidationStatusInvalid:       cervii18n.FieldUserStatusInvalid,
	}
	return translateValidationFields(fields, keys)
}

// profileFieldKeys 把个人资料校验错误码映射为本地化文案键。
func profileFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationDisplayNameRequired: cervii18n.FieldDisplayNameRequired,
		useraction.ValidationEmailInvalid:        cervii18n.FieldEmailInvalid,
		useraction.ValidationEmailDuplicate:      cervii18n.FieldEmailDuplicate,
	}
	return translateValidationFields(fields, keys)
}

// passwordFieldKeys 把密码校验错误码映射为本地化文案键。
func passwordFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		useraction.ValidationCurrentPasswordIncorrect: cervii18n.FieldCurrentPasswordIncorrect,
		useraction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
		useraction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
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
