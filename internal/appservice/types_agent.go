package appservice

import (
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// AgentExecutionMode 表示 AI 员工的执行方式。
type AgentExecutionMode string

const (
	AgentExecutionModeManaged AgentExecutionMode = AgentExecutionMode(domain.AgentExecutionModeManaged)
)

// CreateAgentInput 定义新增 AI 员工字段。
type CreateAgentInput struct {
	DisplayName string              `json:"displayName"`
	RoleID      string              `json:"roleId"`
	TeamIDs     []string            `json:"teamIds"`
	Execution   AgentExecutionInput `json:"execution"`
}

// UpdateAgentInput 定义 AI 员工可编辑字段。
type UpdateAgentInput struct {
	DisplayName string   `json:"displayName"`
	RoleID      string   `json:"roleId"`
	TeamIDs     []string `json:"teamIds"`
}

// AgentWorkStatusInput 定义 AI 员工工作状态修改字段。
type AgentWorkStatusInput struct {
	WorkStatus WorkStatus `json:"workStatus"`
}

// AgentExecutionInput 定义 AI 员工执行配置输入。
type AgentExecutionInput struct {
	Mode    AgentExecutionMode          `json:"mode"`
	Managed *AgentManagedExecutionInput `json:"managed,omitempty"`
}

// AgentManagedExecutionInput 定义平台托管执行配置输入。
type AgentManagedExecutionInput struct {
	ProviderID        string `json:"providerId"`
	ModelIdentifier   string `json:"modelIdentifier"`
	SystemInstruction string `json:"systemInstruction"`
}

// AgentListInput 定义 AI 员工目录查询条件。
type AgentListInput struct {
	Query    string      `json:"query" query:"query"`
	Status   *UserStatus `json:"status,omitempty" query:"status"`
	Page     int         `json:"page" query:"page,default=1"`
	PageSize int         `json:"pageSize" query:"pageSize,default=50"`
}

// Agent 定义 AI 员工信息。
type Agent struct {
	ID          string         `json:"id"`
	IdentityID  string         `json:"identityId"`
	DisplayName string         `json:"displayName"`
	Role        RoleSummary    `json:"role"`
	Status      UserStatus     `json:"status"`
	WorkStatus  WorkStatus     `json:"workStatus"`
	Teams       []TeamSummary  `json:"teams"`
	Execution   AgentExecution `json:"execution"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// AgentListItem 定义 AI 员工目录项。
type AgentListItem struct {
	ID          string                `json:"id"`
	IdentityID  string                `json:"identityId"`
	DisplayName string                `json:"displayName"`
	Role        RoleSummary           `json:"role"`
	Status      UserStatus            `json:"status"`
	WorkStatus  WorkStatus            `json:"workStatus"`
	Teams       []TeamSummary         `json:"teams"`
	Execution   AgentExecutionSummary `json:"execution"`
	CreatedAt   time.Time             `json:"createdAt"`
}

// AgentExecution 定义 AI 员工当前生效的执行配置。
type AgentExecution struct {
	RevisionID string                 `json:"revisionId"`
	Mode       AgentExecutionMode     `json:"mode"`
	Managed    *AgentManagedExecution `json:"managed,omitempty"`
}

// AgentManagedExecution 定义平台托管执行配置。
type AgentManagedExecution struct {
	ProviderID        string `json:"providerId"`
	ProviderName      string `json:"providerName"`
	ModelIdentifier   string `json:"modelIdentifier"`
	ModelName         string `json:"modelName"`
	SystemInstruction string `json:"systemInstruction"`
}

// AgentExecutionSummary 定义 AI 员工当前执行配置摘要。
type AgentExecutionSummary struct {
	RevisionID string                        `json:"revisionId"`
	Mode       AgentExecutionMode            `json:"mode"`
	Managed    *AgentManagedExecutionSummary `json:"managed,omitempty"`
}

// AgentManagedExecutionSummary 定义平台托管执行配置摘要。
type AgentManagedExecutionSummary struct {
	ProviderID      string `json:"providerId"`
	ProviderName    string `json:"providerName"`
	ModelIdentifier string `json:"modelIdentifier"`
	ModelName       string `json:"modelName"`
}

// AgentModelOption 定义 AI 员工可使用的对话模型选项。
type AgentModelOption struct {
	ProviderID      string `json:"providerId"`
	ProviderName    string `json:"providerName"`
	ModelIdentifier string `json:"modelIdentifier"`
	ModelName       string `json:"modelName"`
}

// AgentModelOptionList 定义 AI 员工对话模型选项列表。
type AgentModelOptionList struct {
	Models []AgentModelOption `json:"models"`
}

// AgentList 定义 AI 员工分页结果。
type AgentList struct {
	Agents []AgentListItem `json:"agents"`
	Page   PageInfo        `json:"page"`
}
