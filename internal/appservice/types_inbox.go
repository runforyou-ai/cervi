package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// ServiceSessionStatus 表示客服处理状态。
type ServiceSessionStatus string

const (
	ServiceSessionStatusOpen   ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusOpen)
	ServiceSessionStatusClosed ServiceSessionStatus = ServiceSessionStatus(domain.ServiceSessionStatusClosed)
)

// AgentRunStatus 表示会话中 Agent 最近一次运行状态。
type AgentRunStatus string

const (
	AgentRunStatusQueued    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusQueued)
	AgentRunStatusRunning   AgentRunStatus = AgentRunStatus(domain.AgentRunStatusRunning)
	AgentRunStatusSucceeded AgentRunStatus = AgentRunStatus(domain.AgentRunStatusSucceeded)
	AgentRunStatusFailed    AgentRunStatus = AgentRunStatus(domain.AgentRunStatusFailed)
	AgentRunStatusCancelled AgentRunStatus = AgentRunStatus(domain.AgentRunStatusCancelled)
)

// InboxScope 表示统一收件箱读取范围。
type InboxScope string

const (
	InboxScopeAll      InboxScope = InboxScope(domain.InboxScopeAll)
	InboxScopeCustomer InboxScope = InboxScope(domain.InboxScopeCustomer)
	InboxScopeInternal InboxScope = InboxScope(domain.InboxScopeInternal)
)

// CustomerInboxView 表示客户会话的处理归属视图。
type CustomerInboxView string

const (
	CustomerInboxViewQueue     CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewQueue)
	CustomerInboxViewMine      CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewMine)
	CustomerInboxViewCoworkers CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewCoworkers)
	CustomerInboxViewMentioned CustomerInboxView = CustomerInboxView(domain.CustomerInboxViewMentioned)
)

// InboxPartition 表示统一收件箱的置顶分区。
type InboxPartition string

const (
	InboxPartitionAll     InboxPartition = InboxPartition(domain.InboxPartitionAll)
	InboxPartitionPinned  InboxPartition = InboxPartition(domain.InboxPartitionPinned)
	InboxPartitionRegular InboxPartition = InboxPartition(domain.InboxPartitionRegular)
)

// ConversationPinPosition 表示置顶顺序中的落点：before 与 after 相对邻居会话，start 与 end 指整个置顶区的首尾。
type ConversationPinPosition string

const (
	ConversationPinPositionBefore ConversationPinPosition = ConversationPinPosition(domain.ConversationPinPositionBefore)
	ConversationPinPositionAfter  ConversationPinPosition = ConversationPinPosition(domain.ConversationPinPositionAfter)
	ConversationPinPositionStart  ConversationPinPosition = ConversationPinPosition(domain.ConversationPinPositionStart)
	ConversationPinPositionEnd    ConversationPinPosition = ConversationPinPosition(domain.ConversationPinPositionEnd)
)

// ConversationPinInput 定义个人置顶写入；position 为空表示新置顶追加到末尾、已置顶保持原位，取消置顶不接受位置指令。
type ConversationPinInput struct {
	Pinned                  bool                    `json:"pinned"`
	NeighborID              string                  `json:"neighborId"`
	Position                ConversationPinPosition `json:"position"`
	ExpectedPinOrderVersion string                  `json:"expectedPinOrderVersion"`
}

// ConversationPinState 返回写入后的个人置顶事实与顺序版本。
type ConversationPinState struct {
	Pinned          bool   `json:"pinned"`
	PinOrderVersion string `json:"pinOrderVersion"`
}

// InboxQuery 定义与分页边界无关的会话筛选；search 非空时按会话名称搜索，searchRange 为 list 时沿用列表筛选，为 readable 时覆盖全部可读会话且不带其他列表筛选。
type InboxQuery struct {
	Partition          InboxPartition       `json:"partition" query:"partition"`
	Scope              InboxScope           `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView    `json:"customerView" query:"customerView"`
	AssigneeIdentityID string               `json:"assigneeIdentityId" query:"assigneeIdentityId"`
	ChannelID          string               `json:"channelId" query:"channelId"`
	ServiceStatus      ServiceSessionStatus `json:"serviceStatus" query:"serviceStatus"`
	Kinds              []ConversationType   `json:"kinds" query:"kinds"`
	Search             string               `json:"search" query:"search"`
	SearchRange        InboxSearchRange     `json:"searchRange" query:"searchRange"`
}

// LoadInboxInput 定义统一收件箱筛选、会话名称搜索和分页边界。
type LoadInboxInput struct {
	Partition          InboxPartition       `json:"partition" query:"partition"`
	Scope              InboxScope           `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView    `json:"customerView" query:"customerView"`
	AssigneeIdentityID string               `json:"assigneeIdentityId" query:"assigneeIdentityId"`
	ChannelID          string               `json:"channelId" query:"channelId"`
	ServiceStatus      ServiceSessionStatus `json:"serviceStatus" query:"serviceStatus"`
	Kinds              []ConversationType   `json:"kinds" query:"kinds"`
	Search             string               `json:"search" query:"search"`
	SearchRange        InboxSearchRange     `json:"searchRange" query:"searchRange"`
	Cursor             string               `json:"cursor" query:"cursor"`
	BeforeCursor       string               `json:"beforeCursor" query:"beforeCursor"`
	Limit              int                  `json:"limit" query:"limit,default=50"`
}

// query 返回不含分页边界的会话筛选。
func (input LoadInboxInput) query() InboxQuery {
	return InboxQuery{
		Partition: input.Partition,
		Scope:     input.Scope, CustomerView: input.CustomerView, AssigneeIdentityID: input.AssigneeIdentityID,
		ChannelID: input.ChannelID, ServiceStatus: input.ServiceStatus, Kinds: input.Kinds,
		Search: input.Search, SearchRange: input.SearchRange,
	}
}

// InboxAssignee 定义客户会话负责人摘要。
type InboxAssignee struct {
	IdentityID  string                   `json:"identityId"`
	Type        OrganizationIdentityType `json:"type"`
	DisplayName string                   `json:"displayName"`
	AvatarURL   string                   `json:"avatarUrl"`
}

// CustomerServiceAssigneeList 定义客服筛选候选列表。
type CustomerServiceAssigneeList struct {
	Assignees []InboxAssignee `json:"assignees"`
}

// InboxChannel 定义收件箱渠道筛选候选。
type InboxChannel struct {
	ID      string      `json:"id"`
	Type    ChannelType `json:"type"`
	Name    string      `json:"name"`
	Enabled bool        `json:"enabled"`
}

// InboxChannelList 定义收件箱渠道筛选候选列表。
type InboxChannelList struct {
	Channels []InboxChannel `json:"channels"`
}

// ConversationType 表示会话类型，Copilot 线程只在所属客户会话的 AI 助手中出现，不进入统一收件箱。
type ConversationType string

const (
	ConversationTypeCustomer ConversationType = ConversationType(domain.ConversationTypeCustomer)
	ConversationTypeDirect   ConversationType = ConversationType(domain.ConversationTypeDirect)
	ConversationTypeAgent    ConversationType = ConversationType(domain.ConversationTypeAgent)
	ConversationTypeGroup    ConversationType = ConversationType(domain.ConversationTypeGroup)
	ConversationTypeCopilot  ConversationType = ConversationType(domain.ConversationTypeCopilot)
)

// CustomerInboxConversation 定义客户会话摘要。
type CustomerInboxConversation struct {
	Title                     string                    `json:"title"`
	ContactName               *string                   `json:"contactName"`
	ContactAvatarURL          string                    `json:"contactAvatarUrl"`
	ChannelType               ChannelType               `json:"channelType"`
	ChannelName               string                    `json:"channelName"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	// PreviewVisibility 标明摘要取自对客消息还是内部备注。
	PreviewVisibility    *MessageVisibility   `json:"previewVisibility"`
	LastMessageAt        *time.Time           `json:"lastMessageAt"`
	ServiceSessionStatus ServiceSessionStatus `json:"serviceSessionStatus"`
	ServiceSessionID     string               `json:"serviceSessionId"`
	Assignee             *InboxAssignee       `json:"assignee"`
	// AttachmentSupported 表示来源渠道当前支持向客户发送附件。
	AttachmentSupported bool `json:"attachmentSupported"`
	// AttachmentByteLimit 是来源渠道单个外发附件的字节上限。
	AttachmentByteLimit int64 `json:"attachmentByteLimit"`
	// AttachmentCaptionLimit 是来源渠道附件说明的字符上限。
	AttachmentCaptionLimit int `json:"attachmentCaptionLimit"`
	// UnansweredMentionCount 是当前客服周期内被提醒成员尚未在会话中发言的内部提醒数。
	UnansweredMentionCount int `json:"unansweredMentionCount"`
}

// DirectInboxConversation 定义内部单聊摘要。
type DirectInboxConversation struct {
	PeerIdentityID            string                    `json:"peerIdentityId"`
	PeerType                  OrganizationIdentityType  `json:"peerType"`
	PeerName                  string                    `json:"peerName"`
	PeerAvatarURL             string                    `json:"peerAvatarUrl"`
	PeerStatus                UserStatus                `json:"peerStatus"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
}

// AgentInboxConversation 定义 AI 聊天摘要。
type AgentInboxConversation struct {
	Title                     string                    `json:"title"`
	AgentIdentityID           string                    `json:"agentIdentityId"`
	AgentName                 string                    `json:"agentName"`
	AgentAvatarURL            string                    `json:"agentAvatarUrl"`
	AgentStatus               UserStatus                `json:"agentStatus"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
	AgentRunStatus            *AgentRunStatus           `json:"agentRunStatus"`
}

// GroupInboxConversation 定义企业群聊摘要。
type GroupInboxConversation struct {
	Title                     string                    `json:"title"`
	ImageURL                  string                    `json:"imageUrl"`
	Status                    ConversationStatus        `json:"status"`
	Preview                   *string                   `json:"preview"`
	PreviewSenderIdentityType *OrganizationIdentityType `json:"previewSenderIdentityType"`
	LastMessageAt             *time.Time                `json:"lastMessageAt"`
	MemberCount               int                       `json:"memberCount"`
}

// InboxConversation 定义成员统一收件箱列表项。
type InboxConversation struct {
	// PositionCursor 保存列表窗口和匹配锚点的查询位置。
	PositionCursor       string           `json:"positionCursor"`
	LastActivityAt       *time.Time       `json:"lastActivityAt"`
	LastMessageType      *MessageType     `json:"lastMessageType"`
	ID                   string           `json:"id"`
	Type                 ConversationType `json:"type"`
	UnreadCount          int              `json:"unreadCount"`
	MentionedUnreadCount int              `json:"mentionedUnreadCount"`
	MarkedUnread         bool             `json:"markedUnread"`
	Muted                bool             `json:"muted"`
	// Pinned 表示当前用户已把该会话放入个人置顶区。
	Pinned            bool                       `json:"pinned"`
	LastMessageID     *string                    `json:"lastMessageId"`
	LastReadMessageID *string                    `json:"lastReadMessageId"`
	Agent             *AgentInboxConversation    `json:"agent"`
	Customer          *CustomerInboxConversation `json:"customer"`
	Direct            *DirectInboxConversation   `json:"direct"`
	Group             *GroupInboxConversation    `json:"group"`
}

// Inbox 定义成员收件箱查询结果。
type Inbox struct {
	StartCursor string `json:"startCursor"`
	EndCursor   string `json:"endCursor"`
	// PinOrderVersion 是本人置顶顺序的当前版本，置顶写入以它作为并发校验依据。
	PinOrderVersion      string              `json:"pinOrderVersion"`
	HasBefore            bool                `json:"hasBefore"`
	Conversations        []InboxConversation `json:"conversations"`
	NextCursor           string              `json:"nextCursor"`
	HasMore              bool                `json:"hasMore"`
	UnreadCount          int                 `json:"unreadCount"`
	AttentionUnreadCount int                 `json:"attentionUnreadCount"`
	// CustomerMentionedUnreadCount 是处理中客户会话的当前周期内提醒本人且尚未读到的消息数。
	CustomerMentionedUnreadCount int `json:"customerMentionedUnreadCount"`
}

// InboxConversationAvailability 表示指定会话的阅读和列表资格。
type InboxConversationAvailability string

const (
	InboxConversationMatching     InboxConversationAvailability = "matching"
	InboxConversationOutsideQuery InboxConversationAvailability = "outside_query"
	InboxConversationUnavailable  InboxConversationAvailability = "unavailable"
)

// ReadInboxConversationsInput 指定待核对的会话及完整列表筛选。
type ReadInboxConversationsInput struct {
	ConversationIDs []string   `json:"conversationIds"`
	Query           InboxQuery `json:"query"`
}

// InboxConversationResult 返回匹配状态，不可用时保留请求 ID 和资格。
type InboxConversationResult struct {
	ID           string                        `json:"id"`
	Availability InboxConversationAvailability `json:"availability"`
	Conversation *InboxConversation            `json:"conversation"`
}

// InboxConversationResults 按请求顺序返回每项结果。
type InboxConversationResults struct {
	Results []InboxConversationResult `json:"results"`
}

// InboxContextInput 按会话当前位置或原查询位置读取邻域，前后数量零值均为二十五。
type InboxContextInput struct {
	Query        InboxQuery `json:"query"`
	AnchorID     string     `json:"anchorId"`
	AnchorCursor string     `json:"anchorCursor"`
	BeforeLimit  int        `json:"beforeLimit"`
	AfterLimit   int        `json:"afterLimit"`
}

// InboxWindowInput 指定同一查询已加载范围的首尾游标，包含两侧边界。
type InboxWindowInput struct {
	Query       InboxQuery `json:"query"`
	StartCursor string     `json:"startCursor"`
	EndCursor   string     `json:"endCursor"`
}

// InboxWindow 保存连续范围和双向续读位置，空范围仍可保留原边界。
type InboxWindow struct {
	Conversations   []InboxConversation `json:"conversations"`
	StartCursor     string              `json:"startCursor"`
	EndCursor       string              `json:"endCursor"`
	PinOrderVersion string              `json:"pinOrderVersion"`
	HasBefore       bool                `json:"hasBefore"`
	HasAfter        bool                `json:"hasAfter"`
}

// InboxContext 独立返回锚点资格，列表只渲染 Window，Anchor 可与窗口行重叠。
type InboxContext struct {
	Anchor InboxConversationResult `json:"anchor"`
	Window InboxWindow             `json:"window"`
}

// SyncHeads 保存同步探针的不透明比较值，客户端只判断与上次返回是否相同。
type SyncHeads struct {
	ConversationCount      int    `json:"conversationCount"`
	ConversationChecksum   string `json:"conversationChecksum"`
	IdentityProfileVersion string `json:"identityProfileVersion"`
	PinOrderVersion        string `json:"pinOrderVersion"`
}

// InboxSearchRange 表示收件箱检索范围。
type InboxSearchRange string

const (
	InboxSearchRangeList         InboxSearchRange = "list"
	InboxSearchRangeReadable     InboxSearchRange = "readable"
	InboxSearchRangeConversation InboxSearchRange = "conversation"
)

// InboxSearchInput 定义检索文本与范围；列表筛选只在 list 范围生效，会话编号只在 conversation 范围生效。
type InboxSearchInput struct {
	Query              string               `json:"query" query:"query"`
	Range              InboxSearchRange     `json:"range" query:"range"`
	ConversationID     string               `json:"conversationId" query:"conversationId"`
	Scope              InboxScope           `json:"scope" query:"scope"`
	CustomerView       CustomerInboxView    `json:"customerView" query:"customerView"`
	AssigneeIdentityID string               `json:"assigneeIdentityId" query:"assigneeIdentityId"`
	ChannelID          string               `json:"channelId" query:"channelId"`
	ServiceStatus      ServiceSessionStatus `json:"serviceStatus" query:"serviceStatus"`
	Kinds              []ConversationType   `json:"kinds" query:"kinds"`
}

// InboxSearchSegment 表示摘要中的一段文字及其是否命中。
type InboxSearchSegment struct {
	Text  string `json:"text"`
	Match bool   `json:"match"`
}

// InboxSearchMessage 表示命中的消息、所在会话和高亮摘要。
type InboxSearchMessage struct {
	ID           string               `json:"id"`
	Type         MessageType          `json:"type"`
	SenderName   *string              `json:"senderName"`
	OriginatedAt time.Time            `json:"originatedAt"`
	Excerpt      []InboxSearchSegment `json:"excerpt"`
	Conversation InboxConversation    `json:"conversation"`
}

// InboxSearchPersonKind 表示人员检索结果的来源。
type InboxSearchPersonKind string

const (
	InboxSearchPersonMember  InboxSearchPersonKind = "member"
	InboxSearchPersonContact InboxSearchPersonKind = "contact"
)

// InboxSearchPerson 表示命中的企业成员或外部联系人；真人成员携带 userId，AI 员工携带 agentId，外部联系人没有客户会话时 conversationId 为空。
type InboxSearchPerson struct {
	Kind           InboxSearchPersonKind     `json:"kind"`
	ID             string                    `json:"id"`
	UserID         *string                   `json:"userId"`
	AgentID        *string                   `json:"agentId"`
	IdentityType   *OrganizationIdentityType `json:"identityType"`
	DisplayName    string                    `json:"displayName"`
	AvatarURL      string                    `json:"avatarUrl"`
	ConversationID *string                   `json:"conversationId"`
}

// InboxSearchResult 返回会话、消息和人员三组检索结果，每组最多六条。
type InboxSearchResult struct {
	Conversations []InboxConversation  `json:"conversations"`
	Messages      []InboxSearchMessage `json:"messages"`
	People        []InboxSearchPerson  `json:"people"`
}
