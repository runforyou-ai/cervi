package domain

// ChannelType 定义渠道类型。
type ChannelType string

const (
	ChannelTypeWebsite               ChannelType = "website"
	ChannelTypeTelegram              ChannelType = "telegram"
	ChannelTypeWeChatOfficialAccount ChannelType = "wechat_official_account"
)

// MessageChannelTypes 返回当前支持管理的消息渠道类型。
func MessageChannelTypes() []ChannelType {
	return []ChannelType{ChannelTypeWebsite, ChannelTypeTelegram, ChannelTypeWeChatOfficialAccount}
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

// ChannelSupportsAgentAssignee 判断渠道是否允许 AI 员工作为会话负责人并对外回复。
func ChannelSupportsAgentAssignee(channelType ChannelType) bool {
	return channelType == ChannelTypeWebsite
}

// ChannelRoutingTargetType 定义渠道会话流转目标类型。
type ChannelRoutingTargetType string

const (
	ChannelRoutingTargetTypePublicQueue ChannelRoutingTargetType = "public_queue"
	ChannelRoutingTargetTypeTeam        ChannelRoutingTargetType = "team"
	ChannelRoutingTargetTypeMember      ChannelRoutingTargetType = "member"
)
