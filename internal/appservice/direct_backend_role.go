//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	roleaction "github.com/runforyou-ai/cervi/internal/actions/role"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListRoles 返回当前企业的角色和预定义权限目录。
func (b *DirectBackend) ListRoles(ctx context.Context, meta RequestMeta) (RoleList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return RoleList{}, err
	}
	output, err := b.listRoles.Execute(ctx, identity)
	if err != nil {
		return RoleList{}, b.roleError(ctx, meta, err, cervii18n.ErrorRoleListFailed, identity.Organization.ID)
	}
	roles := make([]Role, 0, len(output.Roles))
	for _, role := range output.Roles {
		roles = append(roles, roleFromAction(role))
	}
	permissions := make([]PermissionDefinition, 0, len(output.Permissions))
	for _, permission := range output.Permissions {
		permissions = append(permissions, PermissionDefinition{
			Code: PermissionCode(permission.Code), Resource: PermissionResource(permission.Resource), Level: PermissionLevel(permission.Level),
		})
	}
	return RoleList{Roles: roles, Permissions: permissions}, nil
}

// GetRole 返回当前企业的角色详情。
func (b *DirectBackend) GetRole(ctx context.Context, meta RequestMeta, roleID string) (Role, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Role{}, err
	}
	role, err := b.getRole.Execute(ctx, identity, roleID)
	if err != nil {
		return Role{}, b.roleError(ctx, meta, err, cervii18n.ErrorRoleReadFailed, identity.Organization.ID, "role_id", roleID)
	}
	return roleFromAction(*role), nil
}

// CreateRole 创建自定义角色。
func (b *DirectBackend) CreateRole(ctx context.Context, meta RequestMeta, input RoleInput) (Role, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Role{}, err
	}
	role, err := b.createRole.Execute(ctx, identity, roleInput(input))
	if err != nil {
		return Role{}, b.roleMutationError(ctx, meta, err, cervii18n.ErrorRoleCreateFailed, identity.Organization.ID)
	}
	slog.Info("角色创建成功", "organization_id", identity.Organization.ID, "role_id", role.ID, "permission_count", len(role.Permissions))
	return roleFromAction(*role), nil
}

// UpdateRole 修改角色信息和权限。
func (b *DirectBackend) UpdateRole(ctx context.Context, meta RequestMeta, roleID string, input RoleInput) (Role, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Role{}, err
	}
	role, err := b.updateRole.Execute(ctx, identity, roleID, roleInput(input))
	if err != nil {
		return Role{}, b.roleMutationError(ctx, meta, err, cervii18n.ErrorRoleUpdateFailed, identity.Organization.ID, "role_id", roleID)
	}
	slog.Info("角色保存成功", "organization_id", identity.Organization.ID, "role_id", role.ID, "role_kind", role.Kind, "permission_count", len(role.Permissions))
	return roleFromAction(*role), nil
}

// DeleteRole 删除自定义角色。
func (b *DirectBackend) DeleteRole(ctx context.Context, meta RequestMeta, roleID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteRole.Execute(ctx, identity, roleID); err != nil {
		if errors.Is(err, roleaction.ErrBuiltInDeleteForbidden) {
			return InvalidError(meta, cervii18n.ErrorRoleBuiltInDeleteForbidden, nil)
		}
		return b.roleError(ctx, meta, err, cervii18n.ErrorRoleDeleteFailed, identity.Organization.ID, "role_id", roleID)
	}
	slog.Info("角色删除成功", "organization_id", identity.Organization.ID, "role_id", roleID)
	return nil
}

// roleMutationError 转换角色写入校验和操作错误。
func (b *DirectBackend) roleMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, roleFieldKeys(validationError.Fields))
	}
	if errors.Is(err, roleaction.ErrAdminImmutable) {
		return InvalidError(meta, cervii18n.ErrorRoleAdminImmutable, nil)
	}
	return b.roleError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// roleError 转换角色读取和删除错误。
func (b *DirectBackend) roleError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, roleaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorRoleNotFound)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	slog.Warn("角色操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// roleInput 转换角色输入。
func roleInput(input RoleInput) roleaction.Input {
	permissions := make([]domain.PermissionCode, 0, len(input.Permissions))
	for _, permission := range input.Permissions {
		permissions = append(permissions, domain.PermissionCode(permission))
	}
	return roleaction.Input{Name: input.Name, Description: input.Description, Permissions: permissions}
}

// roleFromAction 转换角色输出。
func roleFromAction(input roleaction.Record) Role {
	permissions := make([]PermissionCode, 0, len(input.Permissions))
	for _, permission := range input.Permissions {
		permissions = append(permissions, PermissionCode(permission))
	}
	return Role{
		ID: input.ID, Kind: RoleKind(input.Kind), Name: input.Name, Description: input.Description,
		Permissions: permissions, MemberCount: input.MemberCount, CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}

// roleFieldKeys 把角色校验错误码映射为本地化文案键。
func roleFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		roleaction.ValidationNameRequired: cervii18n.FieldRoleNameRequired, roleaction.ValidationNameTooLong: cervii18n.FieldRoleNameTooLong,
		roleaction.ValidationNameDuplicate: cervii18n.FieldRoleNameDuplicate, roleaction.ValidationDescriptionTooLong: cervii18n.FieldRoleDescriptionTooLong,
		roleaction.ValidationPermissionsInvalid: cervii18n.FieldRolePermissionsInvalid,
	}
	return translateValidationFields(fields, keys)
}
