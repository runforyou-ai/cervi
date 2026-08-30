//go:build server

// Package agentrun 实现 Agent 单聊运行的持久化与任务执行。
package agentrun

const RunActionName = "agent.run"

// RunInput 定义 Agent Worker 任务输入。
type RunInput struct {
	RunID string `json:"run_id"`
}
