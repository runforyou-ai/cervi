//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	teamaction "github.com/runforyou-ai/cervi/internal/actions/team"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListTeams 返回企业团队列表。
func (b *DirectBackend) ListTeams(ctx context.Context, meta RequestMeta, input TeamListInput) (TeamList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return TeamList{}, err
	}
	output, err := b.listTeams.Execute(ctx, identity, teamaction.ListInput{Query: input.Query, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return TeamList{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamListFailed, identity.Organization.ID, "")
	}
	teams := make([]Team, 0, len(output.Teams))
	for _, team := range output.Teams {
		teams = append(teams, teamFromAction(team))
	}
	return TeamList{Teams: teams, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// CreateTeam 创建企业团队。
func (b *DirectBackend) CreateTeam(ctx context.Context, meta RequestMeta, input TeamInput) (Team, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Team{}, err
	}
	team, err := b.createTeam.Execute(ctx, identity, teamaction.Input{Name: input.Name, Description: input.Description})
	if err != nil {
		return Team{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("团队创建成功", "organization_id", identity.Organization.ID, "team_id", team.ID)
	return teamFromAction(*team), nil
}

// UpdateTeam 修改企业团队。
func (b *DirectBackend) UpdateTeam(ctx context.Context, meta RequestMeta, teamID string, input TeamInput) (Team, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Team{}, err
	}
	team, err := b.updateTeam.Execute(ctx, identity, teamID, teamaction.Input{Name: input.Name, Description: input.Description})
	if err != nil {
		return Team{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamUpdateFailed, identity.Organization.ID, teamID)
	}
	slog.Info("团队更新成功", "organization_id", identity.Organization.ID, "team_id", teamID)
	return teamFromAction(*team), nil
}

// DeleteTeam 删除企业团队及其成员关系。
func (b *DirectBackend) DeleteTeam(ctx context.Context, meta RequestMeta, teamID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteTeam.Execute(ctx, identity, teamID); err != nil {
		return b.teamError(ctx, meta, err, cervii18n.ErrorTeamDeleteFailed, identity.Organization.ID, teamID)
	}
	slog.Info("团队删除成功", "organization_id", identity.Organization.ID, "team_id", teamID)
	return nil
}

// RemoveTeamMember 移出团队成员。
func (b *DirectBackend) RemoveTeamMember(ctx context.Context, meta RequestMeta, teamID string, identityType MemberIdentityType, identityID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.removeTeamMember.Execute(ctx, identity, teamID, domain.MemberIdentityType(identityType), identityID); err != nil {
		return b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberRemoveFailed, identity.Organization.ID, teamID)
	}
	slog.Info("团队成员移出成功", "organization_id", identity.Organization.ID, "team_id", teamID, "identity_type", identityType, "identity_id", identityID)
	return nil
}

// teamError 转换团队领域错误。
func (b *DirectBackend) teamError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, teamID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, teamFieldKeys(validationError.Fields))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, teamaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorTeamNotFound)
	}
	if errors.Is(err, teamaction.ErrMemberNotFound) {
		return NotFoundError(meta, cervii18n.ErrorTeamMemberNotFound)
	}
	attributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	if teamID != "" {
		attributes = append(attributes, "team_id", teamID)
	}
	slog.Warn("团队操作失败", attributes...)
	return FailedError(meta, failureKey)
}

// teamFromAction 转换团队契约。
func teamFromAction(team teamaction.TeamRecord) Team {
	return Team{ID: team.ID, Name: team.Name, Description: team.Description, MemberCount: team.MemberCount, CreatedAt: team.CreatedAt, UpdatedAt: team.UpdatedAt}
}

// teamFieldKeys 把团队校验错误码映射为本地化文案键。
func teamFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		teamaction.ValidationNameRequired:       cervii18n.FieldTeamNameRequired,
		teamaction.ValidationNameTooLong:        cervii18n.FieldTeamNameTooLong,
		teamaction.ValidationNameDuplicate:      cervii18n.FieldTeamNameDuplicate,
		teamaction.ValidationDescriptionTooLong: cervii18n.FieldTeamDescriptionTooLong,
		teamaction.ValidationQueryInvalid:       cervii18n.FieldTeamQueryInvalid,
	}
	return translateValidationFields(fields, keys)
}
