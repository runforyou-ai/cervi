//go:build server

package agentrun

import (
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
)

// TestComposeInstruction 验证角色基线、企业指令与场景规则的拼接顺序和空段落处理。
func TestComposeInstruction(t *testing.T) {
	baseline := roleBaseline(domain.RoleKindCustomerService, "鹿行", "小鹿")
	if !strings.HasPrefix(baseline, "你是企业「鹿行」的 AI 员工「小鹿」，专业领域是客户服务。") {
		t.Fatalf("客服基线 = %q", baseline)
	}
	if generic := roleBaseline(domain.RoleKindCustomerService, "鹿行", ""); !strings.HasPrefix(generic, "你是企业「鹿行」的 AI 员工，专业领域是客户服务。") {
		t.Fatalf("省略名称的基线 = %q", generic)
	}
	if custom := roleBaseline(domain.RoleKindCustom, "鹿行", "小鹿"); custom != roleBaseline(domain.RoleKindMember, "鹿行", "小鹿") {
		t.Fatalf("自定义角色应按成员基线处理：%q", custom)
	}
	full := composeInstruction(baseline, "  只回答售后问题。 ", agentChatSceneRules)
	if !strings.HasPrefix(full, baseline+"\n\n只回答售后问题。\n\n"+agentChatSceneRules) {
		t.Fatalf("拼接结果 = %q", full)
	}
	if noEnterprise := composeInstruction(baseline, "   ", agentChatSceneRules); noEnterprise != baseline+"\n\n"+agentChatSceneRules {
		t.Fatalf("空企业指令不应占位：%q", noEnterprise)
	}
}

// TestToolGuidance 验证场景规则只描述本次运行实际注册的工具。
func TestToolGuidance(t *testing.T) {
	if guidance := toolGuidance(behaviorTools{}); guidance != "" {
		t.Fatalf("没有内置工具时不应生成说明：%q", guidance)
	}
	guidance := toolGuidance(behaviorTools{Knowledge: true, CustomerHistory: true})
	if !strings.HasPrefix(guidance, "可用工具：\n") || !strings.Contains(guidance, "search_knowledge") || !strings.Contains(guidance, "search_customer_history") {
		t.Fatalf("工具说明 = %q", guidance)
	}
	if only := toolGuidance(behaviorTools{Knowledge: true}); strings.Contains(only, "search_customer_history") {
		t.Fatalf("未注册的工具不应出现：%q", only)
	}
	if only := toolGuidance(behaviorTools{Knowledge: true}); strings.Contains(only, "ask_customer") || strings.Contains(only, "handoff_to_human") {
		t.Fatalf("内部场景不应出现终止工具：%q", only)
	}
	terminal := toolGuidance(behaviorTools{Terminal: true})
	if !strings.HasPrefix(terminal, "可用工具：\n") || !strings.Contains(terminal, "- ask_customer：") || !strings.Contains(terminal, "- handoff_to_human：") {
		t.Fatalf("终止工具说明 = %q", terminal)
	}
}

// TestNewBehaviorSnapshot 验证快照记录角色、场景、规则版本、完整指令与哈希。
func TestNewBehaviorSnapshot(t *testing.T) {
	execution := executionContext{
		AgentName: "小鹿", OrganizationName: "鹿行", RoleKind: string(domain.RoleKindCustomerService), Instruction: "只回答售后问题。",
		ProviderID: "provider", ModelIdentifier: "model", MaxOutputTokens: 1024, ContextWindow: 8192,
	}
	snapshot := newBehaviorSnapshot(execution, agentruntime.SceneCustomer, customerSceneRules, []string{"search_knowledge"}, []string{"工单系统"})
	if snapshot.RoleKind != domain.RoleKindCustomerService || snapshot.Scene != agentruntime.SceneCustomer || snapshot.RulesVersion != behaviorRulesVersion ||
		snapshot.Grounding != agentruntime.GroundingStrict {
		t.Fatalf("快照 = %+v", snapshot)
	}
	if !strings.Contains(snapshot.Instruction, "只回答售后问题。") || !strings.HasSuffix(snapshot.Instruction, customerSceneRules) {
		t.Fatalf("快照指令 = %q", snapshot.Instruction)
	}
	if len(snapshot.InstructionSHA256) != 64 || snapshot.Model.ProviderID != "provider" || snapshot.Model.ContextWindow != 8192 || len(snapshot.Tools) != 1 || len(snapshot.MCPServers) != 1 {
		t.Fatalf("快照元数据 = %+v", snapshot)
	}
	other := newBehaviorSnapshot(execution, agentruntime.SceneAgentChat, agentChatSceneRules, nil, nil)
	if other.InstructionSHA256 == snapshot.InstructionSHA256 {
		t.Fatal("不同指令应产生不同哈希")
	}
	if other.Grounding != "" {
		t.Fatalf("内部场景依据策略 = %q", other.Grounding)
	}
}
