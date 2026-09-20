//go:build server

package agentruntime

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// AssignmentRulesVersion 是角色基线与场景规则的规则版本，基线或场景规则增删时加一；措辞调整只体现在指令哈希上。
const AssignmentRulesVersion = 2

// SceneContext 表示拼接场景规则所需的运行期事实，群聊字段只在群聊场景取值。
type SceneContext struct {
	Scene             Scene
	GroupTitle        string
	MentionCandidates []string // 群内名称唯一的可点名成员，按展示顺序排列。
}

// AssignmentModel 记录一次运行固定使用的模型标识与参数，不含供应商凭据。
type AssignmentModel struct {
	ProviderID      string `json:"providerId"`
	Identifier      string `json:"identifier"`
	MaxOutputTokens int64  `json:"maxOutputTokens"`
	ContextWindow   int64  `json:"contextWindow"`
}

// AssignmentFacts 表示解析有效配置所需的业务事实，由服务端从运行、配置版本与会话读出。
type AssignmentFacts struct {
	RoleKind         domain.RoleKind
	OrganizationName string
	AgentName        string
	Instruction      string // 配置版本中的企业指令。
	Model            AssignmentModel
	Scene            SceneContext
}

// Capabilities 表示执行侧本次实际能提供的工具能力，服务端与设备各自按自己的能力填写。
type Capabilities struct {
	Knowledge  bool     // 本次运行可检索会话绑定的知识库。
	MCPServers []string // 可连接的远程 MCP 服务名称。
}

// Assignment 是一次运行的有效配置，同时作为运行时入参、运行审计快照和设备执行指派。
type Assignment struct {
	RoleKind          domain.RoleKind `json:"roleKind"`
	AgentName         string          `json:"agentName"`
	Scene             Scene           `json:"scene"`
	RulesVersion      int             `json:"rulesVersion"`
	Instruction       string          `json:"instruction"`
	InstructionSHA256 string          `json:"instructionSha256"`
	Model             AssignmentModel `json:"model"`
	Tools             []string        `json:"tools"`
	MCPServers        []string        `json:"mcpServers"`
	Grounding         GroundingPolicy `json:"grounding,omitempty"` // 对客正文的依据检查策略，客服场景为严格策略。
}

// ResolveAssignment 按业务事实与执行侧能力产出一次运行的有效配置，同一份事实在两端只允许工具清单不同。
func ResolveAssignment(facts AssignmentFacts, capabilities Capabilities) Assignment {
	scene := facts.Scene.Scene
	tools := builtinTools{Knowledge: capabilities.Knowledge, Terminal: scene == SceneCustomer}
	instruction := composeInstruction(
		RoleBaseline(facts.RoleKind, facts.OrganizationName, facts.AgentName),
		facts.Instruction,
		sceneRules(facts.Scene, tools),
	)
	sum := sha256.Sum256([]byte(instruction))
	var grounding GroundingPolicy
	if scene == SceneCustomer {
		grounding = GroundingStrict
	}
	return Assignment{
		RoleKind:          facts.RoleKind,
		AgentName:         facts.AgentName,
		Scene:             scene,
		RulesVersion:      AssignmentRulesVersion,
		Instruction:       instruction,
		InstructionSHA256: hex.EncodeToString(sum[:]),
		Model:             facts.Model,
		Tools:             builtinToolNames(scene, capabilities),
		MCPServers:        capabilities.MCPServers,
		Grounding:         grounding,
	}
}

// builtinToolNames 按注册顺序列出本次运行的内置工具，开发期计算器只在内部场景注册，终止工具只在客服场景注册。
func builtinToolNames(scene Scene, capabilities Capabilities) []string {
	names := make([]string, 0, 3)
	if scene != SceneCustomer {
		names = append(names, "calculator")
	}
	if capabilities.Knowledge {
		names = append(names, "search_knowledge")
	}
	if scene == SceneCustomer {
		names = append(names, "ask_customer", "handoff_to_human")
	}
	return names
}
