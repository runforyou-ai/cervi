//go:build server

package chatstate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// RouteSnapshot 定义渠道路由解析出的客服处理周期负责人与团队；两者都为空表示公共队列。
type RouteSnapshot struct {
	TeamID             *string
	TeamName           *string
	AssigneeIdentityID *string
	AssigneeName       *string
	AssigneeType       domain.OrganizationIdentityType
}

// SessionTarget 返回路由快照对应的转交去向与名称快照。
func (r RouteSnapshot) Target() domain.ServiceSessionTarget {
	switch {
	case r.AssigneeIdentityID != nil:
		return domain.ServiceSessionTarget{Kind: domain.ServiceSessionTargetMember, IdentityID: r.AssigneeIdentityID, DisplayName: r.AssigneeName}
	case r.TeamID != nil:
		return domain.ServiceSessionTarget{Kind: domain.ServiceSessionTargetTeam, TeamID: r.TeamID, TeamName: r.TeamName}
	default:
		return domain.ServiceSessionTarget{Kind: domain.ServiceSessionTargetPublicQueue}
	}
}

// ResolveNewSessionRoute 按渠道初始目标、失败目标、公共队列的顺序解析新客服处理周期的路由；目标身份取 FOR KEY SHARE，调用方须在进入会话锁之前调用。
func ResolveNewSessionRoute(ctx context.Context, db bun.IDB, channel *servermodels.Channel) (RouteSnapshot, error) {
	channelType := domain.ChannelType(channel.Type)
	if route, available, err := availableRoute(ctx, db, channel.OrganizationID, channelType, domain.ChannelRoutingTargetType(channel.InitialRoutingTargetType), channel.InitialRoutingTargetID, true); err != nil {
		return RouteSnapshot{}, fmt.Errorf("resolve message channel initial route: %w", err)
	} else if available {
		return route, nil
	}
	slog.Warn("消息渠道初始路由不可用", "organization_id", channel.OrganizationID, "channel_id", channel.ID, "target_type", channel.InitialRoutingTargetType)
	if route, available, err := availableRoute(ctx, db, channel.OrganizationID, channelType, domain.ChannelRoutingTargetType(channel.FallbackRoutingTargetType), channel.FallbackRoutingTargetID, true); err != nil {
		return RouteSnapshot{}, fmt.Errorf("resolve message channel fallback route: %w", err)
	} else if available {
		return route, nil
	}
	slog.Warn("消息渠道失败路由不可用，进入公共队列", "organization_id", channel.OrganizationID, "channel_id", channel.ID, "target_type", channel.FallbackRoutingTargetType)
	return RouteSnapshot{}, nil
}

// ResolveHandoffRoute 只按渠道失败目标解析 AI 转交人工的去向：目标不可用、为 AI 员工或无效时进入公共队列；lock 为 true 时目标身份取 FOR KEY SHARE。
func ResolveHandoffRoute(ctx context.Context, db bun.IDB, channel *servermodels.Channel, lock bool) (RouteSnapshot, error) {
	// 身份类型不可变，失败目标为 AI 员工时不加锁直接进入公共队列，交接只锁定人工目标。
	if domain.ChannelRoutingTargetType(channel.FallbackRoutingTargetType) == domain.ChannelRoutingTargetTypeMember && channel.FallbackRoutingTargetID != nil {
		var identityType domain.OrganizationIdentityType
		err := db.NewSelect().Model((*servermodels.OrganizationIdentity)(nil)).Column("type").
			Where("oi.organization_id = ? AND oi.id = ?", channel.OrganizationID, *channel.FallbackRoutingTargetID).
			Scan(ctx, &identityType)
		if errors.Is(err, sql.ErrNoRows) || identityType == domain.OrganizationIdentityTypeAgent {
			return RouteSnapshot{}, nil
		}
		if err != nil {
			return RouteSnapshot{}, fmt.Errorf("load message channel handoff target type: %w", err)
		}
	}
	route, available, err := availableRoute(ctx, db, channel.OrganizationID, domain.ChannelType(channel.Type),
		domain.ChannelRoutingTargetType(channel.FallbackRoutingTargetType), channel.FallbackRoutingTargetID, lock)
	if err != nil {
		return RouteSnapshot{}, fmt.Errorf("resolve message channel handoff route: %w", err)
	}
	if !available || route.AssigneeType == domain.OrganizationIdentityTypeAgent {
		return RouteSnapshot{}, nil
	}
	return route, nil
}

// LoadConversationChannel 读取客户会话所属的消息渠道。
func LoadConversationChannel(ctx context.Context, db bun.IDB, organizationID, conversationID string) (*servermodels.Channel, error) {
	channel := &servermodels.Channel{}
	if err := db.NewSelect().Model(channel).
		Join("JOIN contact_channel_identities AS cci ON cci.channel_id = c.id AND cci.organization_id = c.organization_id").
		Join("JOIN customer_conversations AS cc ON cc.contact_channel_identity_id = cci.id AND cc.organization_id = cci.organization_id").
		Where("cc.organization_id = ? AND cc.conversation_id = ?", organizationID, conversationID).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load customer conversation channel: %w", err)
	}
	return channel, nil
}

// availableRoute 判断路由目标当前是否可用并返回对应快照。
func availableRoute(ctx context.Context, db bun.IDB, organizationID string, channelType domain.ChannelType, targetType domain.ChannelRoutingTargetType, targetID *string, lock bool) (RouteSnapshot, bool, error) {
	switch targetType {
	case domain.ChannelRoutingTargetTypePublicQueue:
		return RouteSnapshot{}, true, nil
	case domain.ChannelRoutingTargetTypeTeam:
		if targetID == nil {
			return RouteSnapshot{}, false, nil
		}
		team := &servermodels.Team{}
		err := db.NewSelect().Model(team).Column("t.id", "t.name").
			Where("t.organization_id = ?", organizationID).
			Where("t.id = ?", *targetID).
			Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return RouteSnapshot{}, false, nil
		}
		if err != nil {
			return RouteSnapshot{}, false, err
		}
		// 团队队列须由真人承接，团队内没有开启接待的真人成员时视为不可用。
		available, err := identityaction.TeamHasCustomerHandler(ctx, db, organizationID, team.ID)
		if err != nil {
			return RouteSnapshot{}, false, err
		}
		if !available {
			slog.Warn("消息渠道路由的团队没有可接待的真人成员", "organization_id", organizationID, "team_id", team.ID)
			return RouteSnapshot{}, false, nil
		}
		return RouteSnapshot{TeamID: &team.ID, TeamName: &team.Name}, true, nil
	case domain.ChannelRoutingTargetTypeMember:
		if targetID == nil {
			return RouteSnapshot{}, false, nil
		}
		load := identityaction.LoadActiveCustomerHandlingIdentity
		if lock {
			load = identityaction.LockActiveCustomerHandlingIdentity
		}
		identity, err := load(ctx, db, organizationID, *targetID)
		if errors.Is(err, sql.ErrNoRows) {
			return RouteSnapshot{}, false, nil
		}
		if err != nil {
			return RouteSnapshot{}, false, err
		}
		identityType := domain.OrganizationIdentityType(identity.Type)
		if identityType == domain.OrganizationIdentityTypeAgent && !domain.ChannelSupportsAgentAssignee(channelType) {
			slog.Warn("消息渠道不支持 AI 员工作为负责人",
				"organization_id", organizationID,
				"channel_type", channelType,
				"agent_identity_id", identity.ID,
			)
			return RouteSnapshot{}, false, nil
		}
		return RouteSnapshot{AssigneeIdentityID: &identity.ID, AssigneeName: &identity.DisplayName, AssigneeType: identityType}, true, nil
	default:
		return RouteSnapshot{}, false, nil
	}
}
