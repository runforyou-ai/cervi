//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

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
		return UserList{}, localizedError(meta, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return UserList{}, ctx.Err()
		}
		slog.Warn("读取企业成员列表失败", "organization_id", identity.Organization.ID, "error", err)
		return UserList{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserListFailed, nil)
	}
	users := make([]DirectoryUser, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), CreatedAt: user.CreatedAt})
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
		return DirectoryUser{}, localizedError(meta, http.StatusNotFound, "USER_NOT_FOUND", cervii18n.ErrorUserNotFound, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			return DirectoryUser{}, ctx.Err()
		}
		slog.Warn("读取企业成员失败", "organization_id", identity.Organization.ID, "user_id", userID, "error", err)
		return DirectoryUser{}, localizedError(meta, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserReadFailed, nil)
	}
	return DirectoryUser{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Role: UserRole(user.Role), Status: UserStatus(user.Status), CreatedAt: user.CreatedAt}, nil
}
