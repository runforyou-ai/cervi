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

// ListTeamMembers 返回团队成员列表。
func (b *DirectBackend) ListTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberListInput) (TeamMemberList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return TeamMemberList{}, err
	}
	output, err := b.listTeamMembers.Execute(ctx, identity, teamID, teamaction.MemberListInput{
		Query: input.Query, WorkStatus: optionalDomain[WorkStatus, domain.WorkStatus](input.WorkStatus), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return TeamMemberList{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberListFailed, identity.Organization.ID, teamID)
	}
	members := make([]TeamMember, 0, len(output.Members))
	for _, member := range output.Members {
		members = append(members, TeamMember{
			IdentityID: member.IdentityID, IdentityType: OrganizationIdentityType(member.IdentityType),
			DisplayName: member.DisplayName, WorkStatus: WorkStatus(member.WorkStatus), JoinedAt: member.JoinedAt,
		})
	}
	return TeamMemberList{Members: members, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// ListTeamMemberCandidates 返回尚未加入团队的企业身份。
func (b *DirectBackend) ListTeamMemberCandidates(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberCandidateInput) (TeamMemberCandidateList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return TeamMemberCandidateList{}, err
	}
	output, err := b.listTeamMemberCandidates.Execute(ctx, identity, teamID, teamaction.MemberCandidateInput{Query: input.Query, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return TeamMemberCandidateList{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberListFailed, identity.Organization.ID, teamID)
	}
	avatarFileIDs := make([]string, 0, len(output.Members))
	for _, member := range output.Members {
		if member.AvatarFileID != nil {
			avatarFileIDs = append(avatarFileIDs, *member.AvatarFileID)
		}
	}
	avatarURLs, err := b.activeFileURLs(ctx, identity, avatarFileIDs)
	if err != nil {
		return TeamMemberCandidateList{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberListFailed, identity.Organization.ID, teamID)
	}
	members := make([]TeamMemberCandidate, 0, len(output.Members))
	for _, member := range output.Members {
		members = append(members, TeamMemberCandidate{
			IdentityType: OrganizationIdentityType(member.IdentityType), IdentityID: member.IdentityID,
			DisplayName: member.DisplayName, AvatarURL: optionalFileURL(avatarURLs, member.AvatarFileID),
		})
	}
	return TeamMemberCandidateList{Members: members, Page: PageInfo{Number: output.Page.Number, Size: output.Page.Size, Total: output.Page.Total}}, nil
}

// AddTeamMembers 将企业身份批量加入团队。
func (b *DirectBackend) AddTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberInput) (Team, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Team{}, err
	}
	members := make([]teamaction.MemberIdentity, 0, len(input.Members))
	for _, member := range input.Members {
		members = append(members, teamaction.MemberIdentity{IdentityType: domain.OrganizationIdentityType(member.IdentityType), IdentityID: member.IdentityID})
	}
	team, err := b.addTeamMembers.Execute(ctx, identity, teamID, members)
	if err != nil {
		return Team{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberAddFailed, identity.Organization.ID, teamID)
	}
	slog.Info("团队成员添加成功", "organization_id", identity.Organization.ID, "team_id", teamID, "requested_member_count", len(members))
	return teamFromAction(*team), nil
}

// RemoveTeamMembers 将企业身份批量移出团队。
func (b *DirectBackend) RemoveTeamMembers(ctx context.Context, meta RequestMeta, teamID string, input TeamMemberInput) (Team, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Team{}, err
	}
	members := make([]teamaction.MemberIdentity, 0, len(input.Members))
	for _, member := range input.Members {
		members = append(members, teamaction.MemberIdentity{IdentityType: domain.OrganizationIdentityType(member.IdentityType), IdentityID: member.IdentityID})
	}
	team, err := b.removeTeamMembers.Execute(ctx, identity, teamID, members)
	if err != nil {
		return Team{}, b.teamError(ctx, meta, err, cervii18n.ErrorTeamMemberRemoveFailed, identity.Organization.ID, teamID)
	}
	slog.Info("团队成员移出成功", "organization_id", identity.Organization.ID, "team_id", teamID, "requested_member_count", len(members))
	return teamFromAction(*team), nil
}

// teamError 转换团队领域错误。
func (b *DirectBackend) teamError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, teamID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
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
	if errors.Is(err, teamaction.ErrMemberInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
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
		teamaction.ValidationWorkStatusInvalid:  cervii18n.FieldWorkStatusInvalid,
	}
	return translateValidationFields(fields, keys)
}
