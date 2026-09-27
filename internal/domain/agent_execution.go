package domain

// AgentExecutionMode 表示 AI 员工与助理的执行方式。
type AgentExecutionMode string

const (
	// AgentExecutionModeManaged 表示由 Cervi 的运行时使用企业模型服务执行。
	AgentExecutionModeManaged AgentExecutionMode = "managed"
	// AgentExecutionModeLocalAgent 表示由助理绑定电脑上的本机 Agent 执行，只用于助理。
	AgentExecutionModeLocalAgent AgentExecutionMode = "local_agent"
)

// LocalAgentKind 表示经 ACP 驱动的本机 Agent 种类。
type LocalAgentKind string

const (
	LocalAgentKindCodex LocalAgentKind = "codex"
)

// LocalAgentKindValid 判断本机 Agent 种类是否受支持。
func LocalAgentKindValid(kind LocalAgentKind) bool {
	return kind == LocalAgentKindCodex
}
