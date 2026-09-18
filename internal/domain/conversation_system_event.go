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
	ConversationSystemEventServiceSessionHandedOff   ConversationSystemEventType = "service_session_handed_off"
	ConversationSystemEventServiceSessionClaimed     ConversationSystemEventType = "service_session_claimed"
	ConversationSystemEventServiceSessionTakenOver   ConversationSystemEventType = "service_session_taken_over"
	ConversationSystemEventServiceSessionTransferred ConversationSystemEventType = "service_session_transferred"
	ConversationSystemEventServiceSessionClosed      ConversationSystemEventType = "service_session_closed"
	ConversationSystemEventServiceSessionReopened    ConversationSystemEventType = "service_session_reopened"
)

// ServiceSessionTargetKind 定义客服处理周期流转去向的类型。
type ServiceSessionTargetKind string

const (
	ServiceSessionTargetPublicQueue ServiceSessionTargetKind = "public_queue"
	ServiceSessionTargetTeam        ServiceSessionTargetKind = "team"
	ServiceSessionTargetMember      ServiceSessionTargetKind = "member"
)

// ServiceSessionTarget 记录客服处理周期流转的去向及名称快照。
type ServiceSessionTarget struct {
	Kind        ServiceSessionTargetKind `json:"kind"`
	TeamID      *string                  `json:"teamId,omitempty"`
	TeamName    *string                  `json:"teamName,omitempty"`
	IdentityID  *string                  `json:"identityId,omitempty"`
	DisplayName *string                  `json:"displayName,omitempty"`
}

// ServiceSessionHandedOffEvent 是 service_session_handed_off 事件的结构化内容；ReasonText 只向成员展示。
type ServiceSessionHandedOffEvent struct {
	ServiceSessionID string               `json:"serviceSessionId"`
	FromIdentityID   string               `json:"fromIdentityId"`
	FromDisplayName  string               `json:"fromDisplayName"`
	Target           ServiceSessionTarget `json:"target"`
	Reason           AgentHandoffReason   `json:"reason"`
	ReasonText       string               `json:"reasonText"`
	AgentRunID       *string              `json:"agentRunId"`
}

// ServiceSessionOperatedEvent 是成员领取、接管、转交、关闭、重开客服处理周期事件的结构化内容：操作人、原负责人与转交目标。
type ServiceSessionOperatedEvent struct {
	ServiceSessionID string                `json:"serviceSessionId"`
	ActorIdentityID  string                `json:"actorIdentityId"`
	ActorDisplayName string                `json:"actorDisplayName"`
	FromIdentityID   *string               `json:"fromIdentityId,omitempty"`
	FromDisplayName  *string               `json:"fromDisplayName,omitempty"`
	Target           *ServiceSessionTarget `json:"target,omitempty"`
}
