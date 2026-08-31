package domain

// InboxScope 定义统一收件箱读取范围。
type InboxScope string

const (
	InboxScopeAll      InboxScope = "all"
	InboxScopeCustomer InboxScope = "customer"
	InboxScopeInternal InboxScope = "internal"
)

// CustomerInboxView 定义客户会话队列视图。
type CustomerInboxView string

const (
	CustomerInboxViewQueue     CustomerInboxView = "queue"
	CustomerInboxViewMine      CustomerInboxView = "mine"
	CustomerInboxViewCoworkers CustomerInboxView = "coworkers"
	CustomerInboxViewClosed    CustomerInboxView = "closed"
)
