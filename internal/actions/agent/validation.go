//go:build server

package agent

import "github.com/runforyou-ai/cervi/internal/common"

const (
	ValidationDisplayNameRequired       common.FieldCode = "AGENT_DISPLAY_NAME_REQUIRED"
	ValidationTeamInvalid               common.FieldCode = "AGENT_TEAM_INVALID"
	ValidationRoleInvalid               common.FieldCode = "AGENT_ROLE_INVALID"
	ValidationStatusInvalid             common.FieldCode = "AGENT_STATUS_INVALID"
	ValidationWorkStatusInvalid         common.FieldCode = "AGENT_WORK_STATUS_INVALID"
	ValidationWorkStatusUnavailable     common.FieldCode = "AGENT_WORK_STATUS_UNAVAILABLE"
	ValidationExecutionInvalid          common.FieldCode = "AGENT_EXECUTION_INVALID"
	ValidationModelInvalid              common.FieldCode = "AGENT_MODEL_INVALID"
	ValidationSystemInstructionRequired common.FieldCode = "AGENT_SYSTEM_INSTRUCTION_REQUIRED"
	ValidationSystemInstructionTooLong  common.FieldCode = "AGENT_SYSTEM_INSTRUCTION_TOO_LONG"
)
