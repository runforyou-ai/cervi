package domain

// ChannelType 定义渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite ChannelType = "website"
)

// ChannelRoutingTargetType 定义渠道会话流转目标类型。
type ChannelRoutingTargetType string

const (
	ChannelRoutingTargetTypePublicQueue ChannelRoutingTargetType = "public_queue"
	ChannelRoutingTargetTypeTeam        ChannelRoutingTargetType = "team"
	ChannelRoutingTargetTypeMember      ChannelRoutingTargetType = "member"
)
