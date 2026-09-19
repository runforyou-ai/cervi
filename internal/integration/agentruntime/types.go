//go:build server

// Package agentruntime 使用 Eino 执行平台托管 Agent。
package agentruntime

import (
	"context"

	"github.com/runforyou-ai/cervi/internal/domain"

	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
)

// MessageRole 定义模型上下文消息角色。
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

// Message 定义带持久编号的上下文消息，编号用于跨轮次去重。
type Message struct {
	ID       string
	Revision string // 消息内容的修订标识，同一编号在修订变化后重新进入运行期轮次历史。
	Role     MessageRole
	Content  string
	Media    *Media // 非空表示消息携带附件，模型支持该附件格式时在预算内随消息直传。
}

// Media 定义上下文消息的附件格式和大小，内容在直传给模型时按消息编号读取。
type Media struct {
	MIMEType string
	ByteSize int64
}

// Trigger 定义等待 TurnLoop 消费的输入信号。
type Trigger struct {
	Seq        int64
	Correction bool // 为 true 表示 Runtime 发起的依据纠正重新执行，不认领持久输入。
}

// ClaimedInput 定义一次 GenInput 已持久化认领的模型输入。
type ClaimedInput struct {
	Messages []Message
	EndSeq   int64
}

// InputFeed 提供 Agent Run 的持久化输入流。
type InputFeed interface {
	Peek(context.Context, int64) ([]Trigger, error)
	Claim(context.Context, int64) (ClaimedInput, error)
}

// ModelConfig 定义模型组件运行所需配置。
type ModelConfig struct {
	Brand           string
	APIKey          string
	BaseURL         string
	Identifier      string
	MaxOutputTokens int
	ContextWindow   int
	InputModalities []domain.AIModelInputModality
	DisableThinking bool // 为 true 时在模型组件提供思考开关的品牌上关闭思考模式。
}

// AttachmentContent 读取本次运行会话中指定附件消息的文件内容。
type AttachmentContent func(context.Context, string) ([]byte, error)

// KnowledgeSearch 检索本次 Agent Run 获准使用的知识库。
type KnowledgeSearch func(context.Context, knowledgeretrieval.Request) (knowledgeretrieval.Result, error)

// CustomerHistorySearch 查询本次运行所属客户会话中已结束的客服周期。
type CustomerHistorySearch func(context.Context, string) (CustomerHistoryResult, error)

// CustomerHistoryResult 明确区分历史查询不可用与查询后没有匹配记录。
type CustomerHistoryResult struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

// Scene 表示一次运行所属的业务场景。
type Scene string

const (
	SceneCustomer  Scene = "customer"
	SceneAgentChat Scene = "agent_chat"
	SceneGroup     Scene = "group"
	SceneCopilot   Scene = "copilot"
)

// GroundingPolicy 表示对客正文的依据检查策略。
type GroundingPolicy string

// GroundingStrict 要求直接输出的正文在当前输入边界内取得有效依据，否则纠正一次后转人工。
const GroundingStrict GroundingPolicy = "strict"

// RunRequest 定义一次有界 Agent 业务运行。
type RunRequest struct {
	RunID                 string
	Name                  string
	Scene                 Scene           // 本次运行所属的业务场景。
	Grounding             GroundingPolicy // 依据检查策略，空值不检查；只在注册终止工具的客服场景生效。
	Instruction           string
	Model                 ModelConfig
	KnowledgeSearch       KnowledgeSearch
	CustomerHistorySearch CustomerHistorySearch
	ReadAttachment        AttachmentContent
	MCPServers            []MCPServer // 本次运行配置版本绑定的远程 MCP 服务。
	MaxIterations         int         // 单轮模型与工具迭代上限，零值使用默认值。
	MaxTurns              int         // 吸收新输入的轮次上限，零值不限制，由运行 context 控制生命周期。
	StreamID              string
	Attempt               int
	OnStream              func(StreamDelta) // 串行接收合并后的运行流增量，实现不得阻塞。
}

// Usage 定义一次业务运行累计的模型用量。
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// RunResult 定义稳定 Agent 结果及其输入边界；运行出错时 Content、Decision 与 EndSeq 为零值，Usage 和 Blocks 仍给出已产生的部分。
type RunResult struct {
	Content  string           // 发给对方的正文：回答、追问内容或转人工说明；Runtime 构造的转人工为空。
	Decision TerminalDecision // 结束方式，Kind 为空表示直接输出正文作为回答。
	EndSeq   int64
	Usage    Usage
	Blocks   []Block
}

// Runtime 执行一次可吸收后续输入的 Agent Run；返回错误时一并给出已产生的用量和内容块。
type Runtime interface {
	Run(context.Context, RunRequest, InputFeed) (RunResult, error)
}
