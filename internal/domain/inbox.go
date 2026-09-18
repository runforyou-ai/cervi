package domain

// InboxScope 定义统一收件箱读取范围。
type InboxScope string

const (
	InboxScopeAll      InboxScope = "all"
	InboxScopeCustomer InboxScope = "customer"
	InboxScopeInternal InboxScope = "internal"
)

// CustomerInboxView 定义客户会话的处理归属视图。
type CustomerInboxView string

const (
	CustomerInboxViewQueue     CustomerInboxView = "queue"
	CustomerInboxViewMine      CustomerInboxView = "mine"
	CustomerInboxViewCoworkers CustomerInboxView = "coworkers"
	CustomerInboxViewMentioned CustomerInboxView = "mentioned"
)

// InboxPartition 定义统一收件箱的置顶分区，未指定时按完整活动序返回全部会话。
type InboxPartition string

const (
	InboxPartitionAll     InboxPartition = "all"
	InboxPartitionPinned  InboxPartition = "pinned"
	InboxPartitionRegular InboxPartition = "regular"
)

// ConversationPinPosition 定义置顶顺序中的落点：before 与 after 相对邻居会话，start 与 end 指整个置顶区的首尾。
type ConversationPinPosition string

const (
	ConversationPinPositionBefore ConversationPinPosition = "before"
	ConversationPinPositionAfter  ConversationPinPosition = "after"
	ConversationPinPositionStart  ConversationPinPosition = "start"
	ConversationPinPositionEnd    ConversationPinPosition = "end"
)
