package domain

// ChannelType 定义渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite  ChannelType = "website"
	ChannelTypeTelegram ChannelType = "telegram"
)

// MessageChannelTypes 返回当前支持管理的消息渠道类型。
func MessageChannelTypes() []ChannelType {
	return []ChannelType{ChannelTypeWebsite, ChannelTypeTelegram}
}

// SupportedMessageChannelType 判断消息渠道类型是否受支持。
func SupportedMessageChannelType(channelType ChannelType) bool {
	for _, supportedType := range MessageChannelTypes() {
		if channelType == supportedType {
			return true
		}
	}
	return false
}

// ChannelRoutingTargetType 定义渠道会话流转目标类型。
type ChannelRoutingTargetType string

const (
	ChannelRoutingTargetTypePublicQueue ChannelRoutingTargetType = "public_queue"
	ChannelRoutingTargetTypeTeam        ChannelRoutingTargetType = "team"
	ChannelRoutingTargetTypeMember      ChannelRoutingTargetType = "member"
)
