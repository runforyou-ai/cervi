//go:build server

// Package agentruntime 使用 Eino 执行平台托管 Agent。
package agentruntime

import "context"

// MessageRole 定义模型上下文消息角色。
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

// Message 定义与具体 Agent SDK 无关的上下文消息。
type Message struct {
	Role    MessageRole
	Content string
}

// Trigger 定义等待 TurnLoop 消费的输入信号。
type Trigger struct {
	Seq int64
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
}

// RunRequest 定义一次有界 Agent 业务运行。
type RunRequest struct {
	Name        string
	Instruction string
	Model       ModelConfig
	MaxTurns    int
}

// Usage 定义一次业务运行累计的模型用量。
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// RunResult 定义稳定 Agent 回复及其输入边界。
type RunResult struct {
	Content string
	EndSeq  int64
	Usage   Usage
}

// Runtime 执行一次可吸收后续输入的 Agent Run。
type Runtime interface {
	Run(context.Context, RunRequest, InputFeed) (RunResult, error)
}
