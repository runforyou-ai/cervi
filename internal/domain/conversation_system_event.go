package domain

// ConversationSystemEventType 定义会话时间线中的系统事件类型。
type ConversationSystemEventType string

const (
	ConversationSystemEventGroupRenamed          ConversationSystemEventType = "group_renamed"
	ConversationSystemEventGroupMembersAdded     ConversationSystemEventType = "group_members_added"
	ConversationSystemEventGroupMemberRemoved    ConversationSystemEventType = "group_member_removed"
	ConversationSystemEventGroupMemberLeft       ConversationSystemEventType = "group_member_left"
	ConversationSystemEventGroupOwnerTransferred ConversationSystemEventType = "group_owner_transferred"
	ConversationSystemEventGroupDissolved        ConversationSystemEventType = "group_dissolved"
	// 客户会话事件统一使用 service_session_* 前缀。
	ConversationSystemEventServiceSessionHandedOff ConversationSystemEventType = "service_session_handed_off"
)

// ServiceSessionHandoffTargetKind 定义转交人工的去向类型。
type ServiceSessionHandoffTargetKind string

const (
	ServiceSessionHandoffTargetPublicQueue ServiceSessionHandoffTargetKind = "public_queue"
	ServiceSessionHandoffTargetTeam        ServiceSessionHandoffTargetKind = "team"
	ServiceSessionHandoffTargetMember      ServiceSessionHandoffTargetKind = "member"
)

// ServiceSessionHandoffTarget 记录转交人工的去向及名称快照。
type ServiceSessionHandoffTarget struct {
	Kind        ServiceSessionHandoffTargetKind `json:"kind"`
	TeamID      *string                         `json:"teamId,omitempty"`
	TeamName    *string                         `json:"teamName,omitempty"`
	IdentityID  *string                         `json:"identityId,omitempty"`
	DisplayName *string                         `json:"displayName,omitempty"`
}

// ServiceSessionHandedOffEvent 是 service_session_handed_off 事件的结构化内容；ReasonText 只向成员展示。
type ServiceSessionHandedOffEvent struct {
	ServiceSessionID string                      `json:"serviceSessionId"`
	FromIdentityID   string                      `json:"fromIdentityId"`
	FromDisplayName  string                      `json:"fromDisplayName"`
	Target           ServiceSessionHandoffTarget `json:"target"`
	Reason           AgentHandoffReason          `json:"reason"`
	ReasonText       string                      `json:"reasonText"`
	AgentRunID       *string                     `json:"agentRunId"`
}
