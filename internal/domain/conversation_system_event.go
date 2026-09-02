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
)
