//go:build server

package appservice

import (
	"context"
	"log/slog"

	memberaction "github.com/runforyou-ai/cervi/internal/actions/member"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListMemberOptions 返回可分配的企业成员和 AI 员工。
func (b *DirectBackend) ListMemberOptions(ctx context.Context, meta RequestMeta, input MemberOptionListInput) (MemberOptionList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MemberOptionList{}, err
	}
	output, err := b.listMemberOptions.Execute(ctx, identity, memberaction.ListOptionsInput{Query: input.Query, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		if ctx.Err() != nil {
			return MemberOptionList{}, ctx.Err()
		}
		slog.Warn("读取企业成员选择项失败", "organization_id", identity.Organization.ID, "error", err)
		return MemberOptionList{}, FailedError(meta, cervii18n.ErrorUserListFailed)
	}
	members := make([]MemberOption, 0, len(output.Members))
	for _, member := range output.Members {
		members = append(members, MemberOption{
			ID: member.ID, Type: MemberIdentityType(member.Type), DisplayName: member.DisplayName, AvatarURL: avatarContentURL(member.AvatarFileID),
		})
	}
	return MemberOptionList{Members: members, Page: PageInfo{Number: output.Page, Size: output.Size, Total: output.Total}}, nil
}
