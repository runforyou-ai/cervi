package appservice

import (
	"encoding/json"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// DeviceWorkRun 定义设备待领取运行的摘要。
type DeviceWorkRun struct {
	RunID          string `json:"runId"`
	ConversationID string `json:"conversationId"`
}

// DeviceWork 定义设备的工作水位与按创建顺序排列的待领取运行。
type DeviceWork struct {
	WorkSeq int64           `json:"workSeq,string"`
	Runs    []DeviceWorkRun `json:"runs"`
}

// DeviceRunClaim 定义设备领取运行后得到的有效配置、租约到期时间、续租间隔与运行总时限；有效配置是运行时的不透明 JSON。
type DeviceRunClaim struct {
	Assignment                json.RawMessage `json:"assignment"`
	LeaseExpiresAt            time.Time       `json:"leaseExpiresAt"`
	LeaseRenewIntervalSeconds int             `json:"leaseRenewIntervalSeconds"`
	RunTimeoutSeconds         int             `json:"runTimeoutSeconds"`
}

// DeviceRunLease 定义续租结果，Ended 为 true 表示运行已结束，设备应停止执行。
type DeviceRunLease struct {
	Ended          bool       `json:"ended"`
	LeaseExpiresAt *time.Time `json:"leaseExpiresAt"`
}

// DeviceRunInputPeekInput 定义读取输入信号的起点序号。
type DeviceRunInputPeekInput struct {
	AfterSeq int `query:"afterSeq,default=0"`
}

// DeviceRunInputSignals 定义尚未认领的连续输入序号。
type DeviceRunInputSignals struct {
	Seqs []int64 `json:"seqs"`
}

// DeviceRunInputClaimInput 定义认领输入的截止序号。
type DeviceRunInputClaimInput struct {
	ThroughSeq int64 `json:"throughSeq,string"`
}

// DeviceRunClaimedInput 定义认领结果：Suppressed 为 true 表示运行已失效，设备应停止执行；上下文消息是运行时的不透明 JSON。
type DeviceRunClaimedInput struct {
	Suppressed bool            `json:"suppressed"`
	EndSeq     int64           `json:"endSeq,string"`
	Messages   json.RawMessage `json:"messages"`
}

// DeviceRunKnowledgeSearchInput 定义设备运行的知识检索参数，检索参数是运行时的不透明 JSON。
type DeviceRunKnowledgeSearchInput struct {
	Request json.RawMessage `json:"request"`
}

// DeviceRunKnowledgeSearchResult 定义设备运行的知识检索结果，检索结果是运行时的不透明 JSON。
type DeviceRunKnowledgeSearchResult struct {
	Result json.RawMessage `json:"result"`
}

// DeviceRunWebSearchInput 定义设备运行的联网搜索参数，搜索参数是运行时的不透明 JSON。
type DeviceRunWebSearchInput struct {
	Request json.RawMessage `json:"request"`
}

// DeviceRunWebSearchResult 定义设备运行的联网搜索结果，搜索结果是运行时的不透明 JSON。
type DeviceRunWebSearchResult struct {
	Result json.RawMessage `json:"result"`
}

// DeviceRunResultInput 定义设备运行的成功结果；结束方式、用量与过程内容块是运行时的不透明 JSON，为空表示直接回答且没有过程内容。
type DeviceRunResultInput struct {
	Content  string          `json:"content"`
	EndSeq   int64           `json:"endSeq,string"`
	Decision json.RawMessage `json:"decision,omitempty"`
	Usage    json.RawMessage `json:"usage,omitempty"`
	Blocks   json.RawMessage `json:"blocks,omitempty"`
}

// DeviceRunFailureCode 定义设备可上报的运行失败原因。
type DeviceRunFailureCode string

const (
	DeviceRunFailureRuntimeFailed DeviceRunFailureCode = DeviceRunFailureCode(domain.AgentRunErrorCodeDeviceRunFailed)
)

// DeviceRunFailureInput 定义设备上报的运行失败原因与详情；用量与过程内容块是运行时的不透明 JSON，为空表示没有已产生的过程内容。
type DeviceRunFailureInput struct {
	ErrorCode DeviceRunFailureCode `json:"errorCode"`
	Message   string               `json:"message"`
	Usage     json.RawMessage      `json:"usage,omitempty"`
	Blocks    json.RawMessage      `json:"blocks,omitempty"`
}
