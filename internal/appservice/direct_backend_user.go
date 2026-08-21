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

// UpdateProfile 修改当前用户的姓名和邮箱。
func (b *DirectBackend) UpdateProfile(ctx context.Context, meta RequestMeta, input ProfileInput) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.updateProfile.Execute(ctx, identity, useraction.ProfileInput{
		DisplayName: input.DisplayName,
		Email:       input.Email,
	})
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return User{}, InvalidError(meta, cervii18n.ErrorValidationFailed, profileFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return User{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存个人资料失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorProfileUpdateFailed)
	}
	slog.Info("个人资料保存成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return userFromModel(*user), nil
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

// UpdateUserPreferences 保存当前用户的语言和时区设置。
func (b *DirectBackend) UpdateUserPreferences(ctx context.Context, meta RequestMeta, input UserPreferencesInput) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.updateUserPreferences.Execute(ctx, identity, useraction.PreferencesInput{
		Locale: domain.Locale(input.Locale), TimeZone: input.TimeZone,
	})
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return User{}, InvalidError(meta, cervii18n.ErrorValidationFailed, preferencesFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return User{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存语言和时区失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorPreferencesUpdateFailed)
	}
	slog.Info("语言和时区保存成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "locale", input.Locale, "time_zone", input.TimeZone)
	return userFromModel(*user), nil
}

// UpdateUserWorkStatus 保存当前用户主动设置的工作状态。
func (b *DirectBackend) UpdateUserWorkStatus(ctx context.Context, meta RequestMeta, input UserWorkStatusInput) (User, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return User{}, err
	}
	user, err := b.updateUserWorkStatus.Execute(ctx, identity, useraction.WorkStatusInput{
		WorkStatus: domain.WorkStatus(input.WorkStatus),
	})
	if err != nil {
		if ctx.Err() != nil {
			return User{}, ctx.Err()
		}
		var validationError *common.FieldError
		if errors.As(err, &validationError) {
			return User{}, InvalidError(meta, cervii18n.ErrorValidationFailed, workStatusFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return User{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("保存工作状态失败", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "error", err)
		return User{}, FailedError(meta, cervii18n.ErrorWorkStatusUpdateFailed)
	}
	slog.Info("工作状态保存成功", "organization_id", identity.Organization.ID, "user_id", identity.User.ID, "work_status", input.WorkStatus)
	return userFromModel(*user), nil
}

// ListUsers 返回企业成员列表。
func (b *DirectBackend) ListUsers(ctx context.Context, meta RequestMeta, input UserListInput) (UserList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return UserList{}, err
	}
	output, err := b.listUsers.Execute(ctx, identity, useraction.ListInput{
		Query: input.Query, Status: optionalDomain[UserStatus, domain.UserStatus](input.Status), Role: optionalDomain[UserRole, domain.UserRole](input.Role), Page: input.Page, PageSize: input.PageSize,
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
	users := make([]DirectoryUser, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), WorkStatus: WorkStatus(user.WorkStatus), CreatedAt: user.CreatedAt})
	}
	return UserList{Users: users, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// GetUser 返回企业成员详情。
func (b *DirectBackend) GetUser(ctx context.Context, meta RequestMeta, userID string) (DirectoryUser, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return DirectoryUser{}, err
	}
	user, err := b.getUser.Execute(ctx, identity, userID)
	if errors.Is(err, useraction.ErrNotFound) {
		return DirectoryUser{}, NotFoundError(meta, cervii18n.ErrorUserNotFound)
	}
	if err != nil {
		if ctx.Err() != nil {
			return DirectoryUser{}, ctx.Err()
		}
		slog.Warn("读取企业成员失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return DirectoryUser{}, FailedError(meta, cervii18n.ErrorUserReadFailed)
	}
	return DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), WorkStatus: WorkStatus(user.WorkStatus), CreatedAt: user.CreatedAt}, nil
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
